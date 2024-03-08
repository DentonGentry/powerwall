// Copyright (c), Denton Gentry <dgentry@decarbon.earth>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
)

var (
	teslaOauthConfig = &oauth2.Config{
		RedirectURL:  "https://sherwood-energy-mon.decarbon.earth/redirect_url",
		ClientID:     os.Getenv("TESLA_OAUTH_CLIENT_ID"),
		ClientSecret: os.Getenv("TESLA_OAUTH_CLIENT_SECRET"),
		Scopes:       []string{"openid", "offline_access", "energy_device_data", "energy_cmds"},
		Endpoint: oauth2.Endpoint{
			AuthURL:   "https://auth.tesla.com/oauth2/v3/authorize",
			TokenURL:  "https://auth.tesla.com/oauth2/v3/token",
			AuthStyle: oauth2.AuthStyleInParams,
		},
	}
)

func handleTeslaAuthCallback(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Callback handler, state=%v\n", r.FormValue("state"))
	tokens, err := teslaOauthConfig.Exchange(oauth2.NoContext, r.FormValue("code"))
	if err != nil {
		fmt.Println(err)
		fmt.Fprintf(w, "%v", err)
		return
	}

	fmt.Printf("Callback handler, marshalling tokens\n")
	b, err := json.Marshal(tokens)
	if err != nil {
		fmt.Println(err)
		fmt.Fprintf(w, "%v", err)
		return
	}

	fmt.Printf("Callback handler, tokens=%q\n", string(b))
	f, err := os.OpenFile("/tmp/tesla-tokens.json", os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println(err)
		fmt.Fprintf(w, "%v", err)
		return
	}
	defer f.Close()

	fmt.Printf("Callback handler, writing file\n")
	f.Write(b)
}

func handleTeslaAuthLogin(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Login handler\n")
	oauthStateString := generateStateOauthCookie(w)
	fmt.Printf("Login handler, state=%s\n", oauthStateString)
	u := teslaOauthConfig.AuthCodeURL(oauthStateString)
	fmt.Printf("Login handler, redirecting url=%q\n", u)
	http.Redirect(w, r, u, http.StatusTemporaryRedirect)
}

func randomString(len int) string {
	b := make([]byte, len)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func generateStateOauthCookie(w http.ResponseWriter) string {
	expiration := time.Now().Add(365 * 24 * time.Hour)
	state := randomString(24)
	cookie := http.Cookie{Name: "oauthstate", Value: state, Expires: expiration}
	http.SetCookie(w, &cookie)

	return state
}
func main() {
	fmt.Println("Starting...")
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "ok")
		fmt.Println("Root Handler")
	})
	http.HandleFunc("/login", handleTeslaAuthLogin)
	http.HandleFunc("/redirect_url", handleTeslaAuthCallback)

	fmt.Println("Listening...")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}
