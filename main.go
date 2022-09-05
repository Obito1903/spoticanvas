package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/blang/mpv"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
)

const redirectURI = "http://localhost:8080/callback"

var (
	auth = spotifyauth.New(
		spotifyauth.WithRedirectURL(redirectURI),
		spotifyauth.WithScopes(
			spotifyauth.ScopeUserReadCurrentlyPlaying,
			spotifyauth.ScopeUserReadPlaybackState,
		),
		spotifyauth.WithClientID(os.Getenv("SPOTIFY_ID")),
		spotifyauth.WithClientSecret(os.Getenv("SPOTIFY_SECRET")),
	)
	ch        = make(chan *spotify.Client)
	mpvClient *mpv.Client
	state     = "abc123"
)

type CanvazResp struct {
	Success   string `json:"success"`
	CanvasUrl string `json:"canvas_url"`
	Message   string `json:"message"`
}

func getTrack() {

	url := auth.AuthURL(state)
	fmt.Println("Please log in to Spotify by visiting the following page in your browser:", url)

	// wait for auth to complete
	client := <-ch

	// use the client to make calls that require authorization
	user, err := client.CurrentUser(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("You are logged in as:", user.ID)

	currentID := ""
	for {
		currentTrack, err := client.PlayerCurrentlyPlaying(context.Background())
		if err != nil || currentTrack.Item == nil {
			fmt.Println("here !")
			log.Println(err)
			time.Sleep(1 * time.Second)
			continue
		}
		if currentID != currentTrack.Item.ID.String() {
			currentID = currentTrack.Item.ID.String()
			fmt.Printf("Currently playing (id): %s (%s)\n", currentTrack.Item.Name, currentTrack.Item.ID.String())

			resp, err := http.DefaultClient.Get(fmt.Sprintf("https://api.delitefully.com/api/canvas/%s", currentTrack.Item.ID.String()))
			if err != nil {
				log.Fatal(err)
			}

			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Fatal(err)
			}
			canvazResp := CanvazResp{}
			err = json.Unmarshal(respBody, &canvazResp)
			if err != nil {
				log.Fatal(err)
			}
			if canvazResp.Success == "true" {
				fmt.Println(canvazResp)
				mpvClient.Loadfile(canvazResp.CanvasUrl, mpv.LoadFileModeReplace)
			} else {
				fmt.Println("No canvas found")
				img := currentTrack.Item.Album.Images[0].URL
				mpvClient.Loadfile(img, mpv.LoadFileModeReplace)
			}

		}

		time.Sleep(1 * time.Second)

	}
}

func main() {

	http.HandleFunc("/callback", completeAuth)

	cmd := exec.Command("mpv", "--idle", "--input-ipc-server=/tmp/mpvsocket")
	cmd.Stdout = os.Stdout
	err := cmd.Start()
	time.Sleep(1 * time.Second)
	if err != nil {
		log.Fatal(err)
	}

	ipcc := mpv.NewIPCClient("/tmp/mpvsocket") // Lowlevel client
	mpvClient = mpv.NewClient(ipcc)            // Highlevel client, can also use RPCClient
	mpvClient.SetProperty("loop", true)
	go getTrack()

	http.ListenAndServe(":8080", nil)
}

func completeAuth(w http.ResponseWriter, r *http.Request) {
	tok, err := auth.Token(r.Context(), state, r)
	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
		log.Fatal(err)
	}
	if st := r.FormValue("state"); st != state {
		http.NotFound(w, r)
		log.Fatalf("State mismatch: %s != %s\n", st, state)
	}

	// use the token to get an authenticated client
	client := spotify.New(auth.Client(r.Context(), tok))
	fmt.Fprintf(w, "Login Completed!")
	ch <- client
}
