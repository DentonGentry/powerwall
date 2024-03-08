// Copyright (c) 2020, Denton Gentry <dgentry@decarbon.earth>
// All rights reserved.

// This source code is licensed under the BSD-style license found in the
// LICENSE file in the root directory of this source tree.

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// At the time of this writing, we use PG&E's EV2A rate plan which includes:
// + inexpensive power ($0.17/kWh) midnight - 3pm
// + partial peak power ($0.38/kWh) 3pm - 4pm and 9pm - midnight
// + peak power ($0.49/kWh) 4pm - 9pm
//
// In summer, the solar panels typically generate 65 kWh/day. This is enough to run the house
// with heat pumps running, or generate substantial extra power if the heat pumps are not run.
//
// In winter, due to the hill immediately behind the house, we get only a few hours of direct
// sunlight and can generate as little 8 kWh in a day.
//
// Summertime strategy: TBD closer to summer 2021.
//
// Wintertime strategy: we want to use the battery to supply as much peak power as possible,
// given the large price difference. However we are only allowed to charge the battery from
// solar power, not the grid. Therefore:
// + set the barttery to charge to 100% just before dawn, so that throughout the day all
//   generated solar power will go to charging it.
// + stop charging the battery at 3pm. The Powerwall is 92.5% round trip efficient, meaning
//   that we lose 7.5% of the solar generation. Once we enter partial peak, we choose to send
//   solar power directly to the house instead of charging/discharging the battery.
// + set the battery to discharge at 4pm, to let it supply the house during peak hours.
//   How deeply to let it discharge depends on how much solar power we expect to generate the
//   next day.

// Set the common set of HTTP headers
func SetRequestHeaders(req *http.Request, token string) {
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("X-Tesla-User-Agent", "https://github.com/DentonGentry/powerwall")
	req.Header.Set("Host", "owner-api.teslamotors.com")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

func GetRandomSlug(n int) []byte {
	characters := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]byte, n)
	for i := range b {
		b[i] = characters[rand.Intn(len(characters))]
	}
	return b
}

// Parses an HTML page to find the form of auth-related inputs.
//
// <form method="post" id="form" class="sso-form sign-in-form">
//   <input type="hidden" name="_csrf" value="mh4O8Qn7-wyTq2wB-2OR2bzVdzSSZjlqd4iE" />
//   <input type="hidden" name="_phase" value="authenticate" />
//   <input type="hidden" name="_process" value="1" />
//   <input type="hidden" name="transaction_id" value="qoqqxdxw" />
//   <input type="hidden" name="cancel" value="" id="form-input-cancel" />
func GetHiddenFormInputs(n *html.Node) url.Values {
	fields := url.Values{}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "input" {
			name := "INVALID"
			value := "INVALID"
			for _, a := range c.Attr {
				if a.Key == "type" && a.Val != "hidden" {
					break
				}
				if a.Key == "name" {
					name = a.Val
				}
				if a.Key == "value" {
					value = a.Val
				}
			}
			if name != "INVALID" && value != "INVALID" {
				fields.Set(name, value)
			}
		}
	}

	return fields
}

// Implements the Authorization flow documented in
// https://tesla-api.timdorr.com/api-basics/authentication
func GetOAuthTokenFromTesla(username, password string) string {
	slug := GetRandomSlug(86)
	hash := sha256.Sum256(slug)
	challenge := base64.URLEncoding.EncodeToString(hash[:])
	state := string(GetRandomSlug(20))

	params := url.Values{}
	params.Add("client_id", "ownerapi")
	params.Add("code_challenge", challenge)
	params.Add("code_challenge_method", "S256")
	params.Add("redirect_uri", "https://auth.tesla.com/void/callback")
	params.Add("response_type", "code")
	params.Add("scope", "openid email offline_access")
	params.Add("state", state)

	endpoint, err := url.Parse("https://auth.tesla.com/oauth2/v3/authorize")
	if err != nil {
		log.Fatalf("URL Parse: %v", err)
	}
	endpoint.RawQuery = params.Encode()

	// ---------------------------------------------------------------------------------------
	// Step 1: Obtain the login page
	// ---------------------------------------------------------------------------------------

	resp, err := http.Get(endpoint.String())
	if err != nil {
		log.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		log.Fatalf("Parse: %v", err)
	}

	// ---------------------------------------------------------------------------------------
	// Step 2: Obtain an authorization code
	// ---------------------------------------------------------------------------------------

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

	fields.Set("identity", username)
	fields.Set("credential", password)

	req, err := http.NewRequest("POST", endpoint.String(), strings.NewReader(fields.Encode()))
	req.Header.Add("Cookie", resp.Header.Get("Set-Cookie"))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(fields.Encode())))

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err = client.Do(req)
	if err != nil {
		log.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	location, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		log.Fatalf("Parse POST Response: %v", err)
	}
	query, err := url.ParseQuery(location.RawQuery)
	if err != nil {
		log.Fatalf("ParseQuery POST Response: %v", err)
	}
	code := string(query["code"][0])

	// ---------------------------------------------------------------------------------------
	// Step 3: Exchange authorization code for bearer token
	// ---------------------------------------------------------------------------------------

	verifier := base64.URLEncoding.EncodeToString([]byte(code))
	js, err := json.Marshal(map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     "ownerapi",
		"code":          code,
		"code_verifier": verifier,
		"redirect_uri":  "https://auth.tesla.com/void/callback",
	})
	resp, err = http.Post("https://auth.tesla.com/oauth2/v3/token",
		"application/json", bytes.NewBuffer(js))
	if err != nil {
		log.Fatalf("Bearer POST: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&result)
	if err != nil {
		log.Fatalf("JSON Decode: %v", err)
	}
	access_token := result["access_token"].(string)

	// ---------------------------------------------------------------------------------------
	// Step 4: Exchange bearer token for access token
	// ---------------------------------------------------------------------------------------

	js, err = json.Marshal(map[string]string{
		"grant_type":    "urn:ietf:params:oauth:grant-type:jwt-bearer",
		"client_id":     "81527cff06843c8634fdc09e8ac0abefb46ac849f38fe1e431c2ef2106796384",
		"client_secret": "c7257eb71a564034f9419ee651c7d0e5f7aa6bfbd18bafb5c5c033b093bb2fa3",
	})
	if err != nil {
		log.Fatalln(err)
	}

	req, err = http.NewRequest("POST", "https://owner-api.teslamotors.com/oauth/token",
		strings.NewReader(fields.Encode()))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Content-Length", strconv.Itoa(len(js)))
	req.Header.Add("Authorization", "Bearer "+access_token)

	resp, err = client.Do(req)
	if err != nil {
		log.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	decoder = json.NewDecoder(resp.Body)
	err = decoder.Decode(&result)
	if err != nil {
		log.Fatalf("JSON Decode 2: %v", err)
	}
	access_token = result["access_token"].(string)

	return access_token
}

func GetOAuthTokenFromFile(path string) (string, int64) {
	filename := filepath.Join(path, "tesla_bearer_token")
	token, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", 0
	}

	fileInfo, err := os.Stat(filename)
	if err != nil {
		return "", 0
	}

	now := time.Now()
	mt := fileInfo.ModTime()
	days := now.Sub(mt).Hours() / 24

	return string(token), int64(days)
}

func WriteOAuthTokenToFile(token string, path string) error {
	filename := filepath.Join(path, "tesla_bearer_token")
	newfilename := filename + ".new"

	err := ioutil.WriteFile(newfilename, []byte(token), 0400)
	if err != nil {
		_ = os.Remove(newfilename)
		return err
	}

	err = os.Rename(newfilename, filename)
	if err != nil {
		_ = os.Remove(newfilename)
		return err
	}

	return nil
}

func GetEnergySiteId(token string) (int64, bool) {
	req, err := http.NewRequest("GET", "https://owner-api.teslamotors.com/api/1/products", nil)
	if err != nil {
		log.Fatalln(err)
	}
	SetRequestHeaders(req, token)

	client := &http.Client{Timeout: time.Second * 10}
	resp, err := client.Do(req)
	if err != nil {
		// This is how we determine that a Bearer token we retrieved
		// from a file has expired or is otherwise unuseable.
		return 0, false
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, false
	}

	var result map[string]interface{}
	decoder := json.NewDecoder(resp.Body)
	decoder.UseNumber() // we need the ID field to be an integer, not float64
	err = decoder.Decode(&result)
	if err != nil {
		log.Fatalln(err)
	}

	// walk through the list of products, which can be a mix of vehicles, powerwalls,
	// and other future Tesla products. Pick out the energy_site_id of the first
	// energy site we find.
	var energy_site_id int64
	products := result["response"].([]interface{})
	for idx := 0; idx < len(products); idx++ {
		product := products[idx].(map[string]interface{})
		if id, ok := product["energy_site_id"]; ok {
			energy_site_id, err = id.(json.Number).Int64()
			if err == nil {
				break
			}
		}
	}
	return energy_site_id, true
}

func SetSelfConsumption(token string, energy_site_id int64) {
	url := fmt.Sprintf("https://owner-api.teslamotors.com/api/1/energy_sites/%v/operation",
		energy_site_id)
	js, err := json.Marshal(map[string]string{
		"default_real_mode": "self_consumption",
	})
	if err != nil {
		log.Fatalln(err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(js))
	if err != nil {
		log.Fatalln(err)
	}
	SetRequestHeaders(req, token)

	client := &http.Client{Timeout: time.Second * 10}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&result)
	if err != nil {
		log.Fatalln(err)
	}
}

func SetBackupPercent(token string, energy_site_id int64, percent float64) {
	url := fmt.Sprintf("https://owner-api.teslamotors.com/api/1/energy_sites/%v/backup",
		energy_site_id)
	js, err := json.Marshal(map[string]float64{
		"backup_reserve_percent": percent,
	})
	if err != nil {
		log.Fatalln(err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(js))
	if err != nil {
		log.Fatalln(err)
	}
	SetRequestHeaders(req, token)

	client := &http.Client{Timeout: time.Second * 10}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&result)
	if err != nil {
		log.Fatalln(err)
	}
}

func GetBatteryCharge(token string, energy_site_id int64) float64 {
	url := fmt.Sprintf("https://owner-api.teslamotors.com/api/1/energy_sites/%v/live_status",
		energy_site_id)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalln(err)
	}
	SetRequestHeaders(req, token)

	client := &http.Client{Timeout: time.Second * 10}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	var result map[string]map[string]interface{}
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&result)
	if err != nil {
		log.Fatalln(err)
	}

	response, ok := result["response"]
	if !ok {
		log.Fatalf("no response in %v", result)
	}
	charged, ok := response["percentage_charged"]
	if ok {
		return charged.(float64)
	}

	log.Fatalf("no percentage_charged in %v", response)
	return -1.0
}

func CheckForArguments(username, password *string, statedir *string) {
	if *username == "" {
		*username = os.Getenv("TESLA_CLOUD_USERNAME")
	}
	if *username == "" {
		log.Fatalf("Tesla account name must be provided in --username.")
	}

	if *password == "" {
		*password = os.Getenv("TESLA_CLOUD_PASSWORD")
	}
	if *password == "" {
		log.Fatalf("Tesla account password must be provided in --password.")
	}

	if *statedir == "" {
		*statedir = os.Getenv("POWERWALL_STATE_DIR")
	}
	if *statedir == "" {
		*statedir = os.TempDir()
	}
}

func main() {
	hold := flag.Bool("hold", false, "Hold at current charge.")
	percent := flag.Int("percent", -1, "Battery percentage to aim for.")
	username := flag.String("username", "",
		"User name, typically an email address, as used in the Tesla app")
	password := flag.String("password", "", "Account password, as used in the Tesla app")
	statedir := flag.String("statedir", "", "Directory in which to store state files")
	flag.Parse()
	CheckForArguments(username, password, statedir)

	token, age := GetOAuthTokenFromFile(*statedir)
	energy_site_id, ok := GetEnergySiteId(token)
	if !ok {
		token = GetOAuthTokenFromTesla(*username, *password)
		energy_site_id, ok = GetEnergySiteId(token)
		if !ok {
			log.Fatal("Unable to obtain useable Bearer token")
		}
		err := WriteOAuthTokenToFile(token, *statedir)
		if err != nil {
			log.Fatalf("WriteOAuthTokenToFile: %v", err)
		}
	} else {
		// we try to refresh the bearer token a few days before it expires,
		// as Tesla seems to pretty frequently have transient failures in the
		// OAuth flow. This gives us a few days to keep trying before we lose
		// access. If this refresh fails, we'll stick with the token we have.
		if (45 - age) < 7 {
			new_token := GetOAuthTokenFromTesla(*username, *password)
			energy_site_id, ok = GetEnergySiteId(new_token)
			if ok {
				err := WriteOAuthTokenToFile(new_token, *statedir)
				if err != nil {
					log.Fatalf("WriteOAuthTokenToFile: %v", err)
				}
				token = new_token
			}
		}
	}

	if *percent >= 0.0 {
		SetSelfConsumption(token, energy_site_id)
		SetBackupPercent(token, energy_site_id, float64(*percent))
	} else if *hold {
		SetSelfConsumption(token, energy_site_id)
		charged := GetBatteryCharge(token, energy_site_id)
		SetBackupPercent(token, energy_site_id, charged)
	} else {
		charged := GetBatteryCharge(token, energy_site_id)
		fmt.Printf("%.1f\n", charged)
	}
}
