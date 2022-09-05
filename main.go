package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

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
			spotifyauth.ScopeUserModifyPlaybackState,
			spotifyauth.ScopePlaylistReadCollaborative,
			spotifyauth.ScopePlaylistReadPrivate,
			spotifyauth.ScopeUserLibraryRead,
			"playlist-read",
		),
		spotifyauth.WithClientID("fc2e2b5e7d2549e5811d6784fd4fd02f"),
		spotifyauth.WithClientSecret("7f67518148c6487b9694b2c83318d073"),
	)
	ch    = make(chan *spotify.Client)
	state = "abc123"
)

type CanvazResp struct {
	Success   bool   `json:"success"`
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

	currentTrack, err := client.PlayerCurrentlyPlaying(context.Background())
	if err != nil {
		log.Fatal(err)
	}
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

	fmt.Println(canvazResp)

}

func main() {

	http.HandleFunc("/callback", completeAuth)

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
