// Copyright (c) 2020, Denton Gentry <dgentry@decarbon.earth>
// All rights reserved.

// This source code is licensed under the BSD-style license found in the
// LICENSE file in the root directory of this source tree.

package main

import (
	"golang.org/x/net/html"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRandomSlug(t *testing.T) {
	sizes := []int{1, 50, 99, 512}
	for size := range sizes {
		slug := GetRandomSlug(size)
		if len(slug) != size {
			t.Fatalf("len(GetRandomSlug(%d)) = %d, want %d", size, len(slug), size)
		}
	}
}

func TestGetHiddenFormInputs(t *testing.T) {
	h := strings.NewReader(`<form method="post" id="form" class="sso-form sign-in-form">
	   <input type="hidden" name="_csrf" value="mh4O8Qn7-wyTq2wB-2OR2bzVdzSSZjlqd4iE" />
	   <input type="hidden" name="_phase" value="authenticate" />
	   <input type="hidden" name="_process" value="1" />
	   <input type="hidden" name="transaction_id" value="qoqqxdxw" />
	   <input type="hidden" name="cancel" value="" id="form-input-cancel" />
	   <input type="hidden" name="testA" value="A" />
	   <input type="hidden" name="testB" value="B" />
	   </form>`)
	doc, err := html.Parse(h)
	if err != nil {
		t.Fatal(err)
	}

	var fields url.Values
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "form" {
			for _, a := range n.Attr {
				if a.Key == "class" && strings.Contains(a.Val, "sign-in-form") {
					fields = GetHiddenFormInputs(n)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	var fieldTests = []struct {
		name string
		want string
	}{
		{"_csrf", "mh4O8Qn7-wyTq2wB-2OR2bzVdzSSZjlqd4iE"},
		{"_phase", "authenticate"},
		{"_process", "1"},
		{"transaction_id", "qoqqxdxw"},
		{"cancel", ""},
		{"testA", "A"},
		{"testB", "B"},
	}

	for _, tt := range fieldTests {
		got := fields.Get(tt.name)
		if got != tt.want {
			t.Fatalf(`Url.Get("%s"), got %q want %q`, tt.name, got, tt.want)
		}
	}
}

func TestOAuthTokenFromFile(t *testing.T) {
	path := t.TempDir()
	token := "TestOAuthToken"
	err := WriteOAuthTokenToFile(token, path)
	if err != nil {
		t.Fatalf("WriteOAuthTokenToFile failed: %v", err)
	}

	filename := filepath.Join(path, "tesla_bearer_token")
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	readtoken := string(b)
	if token != readtoken {
		t.Fatalf("Read token got=%q want=%q", readtoken, token)
	}

	new_time := time.Now().AddDate(0, 0, -7)
	err = os.Chtimes(filename, new_time, new_time)
	if err != nil {
		t.Fatalf("os.Chtimes failed: %v", err)
	}

	readtoken, age := GetOAuthTokenFromFile(path)
	if token != readtoken {
		t.Fatalf("GetOAuthTokenFromFile got=%q want=%q", readtoken, token)
	}
	if age != 7 {
		t.Fatalf("GetOAuthTokenFromFile age got=%d want=7", age)
	}
}
