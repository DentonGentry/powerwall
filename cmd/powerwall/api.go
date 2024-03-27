// Copyright (c), Denton Gentry <dgentry@decarbon.earth>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"time"
	"golang.org/x/oauth2"
)

type TeslaState struct {
	mu       sync.Mutex
	apiUrl   string
	siteId   int
	clientId string
	tokens   oauth2.Token
}

var state TeslaState
var tokenFile = "/var/lib/powerwall/tokens"

func (s *TeslaState) ReadFromFile() error {
	b, err := os.ReadFile(tokenFile)
	if err != nil {
		return err
	}

	var newTokens oauth2.Token
	err = json.Unmarshal(b, &newTokens)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens = newTokens

	return nil
}

func (s *TeslaState) WriteToFile() error {
	s.mu.Lock()
	b, err := json.Marshal(s)
	s.mu.Unlock()
	if err != nil {
		return err
	}

	f, err := os.OpenFile(tokenFile, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	f.Write(b)
}

func ApiUpdateAccessToken() {
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		refreshFailed.Add(1)
		return
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
}

func ApiGet(req *http.Request) {
}
