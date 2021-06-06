// Copyright (c) 2020, Denton Gentry <dgentry@decarbon.earth>
// All rights reserved.

// This source code is licensed under the BSD-style license found in the
// LICENSE file in the root directory of this source tree.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

func GetSolarSamples() *model.SampleStream {
	// Prometheus client to fetch timeseries data.
	promUrl := os.Getenv("PROMETHEUS_URL")
	if promUrl == "" {
		promUrl = "http://localhost:9090"
	}

	client, err := api.NewClient(api.Config{Address: promUrl})
	if err != nil {
		log.Fatalf("api.NewClient: %v", err)
	}
	v1api := v1.NewAPI(client)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Fetch data from today. This program expects to be run late at night.
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	r := v1.Range{
		Start: midnight,
		End:   now,
		Step:  5 * time.Minute,
	}

	// 5 minute averaging using technique from https://stackoverflow.com/a/51859662
	numerator := `sum(sum_over_time(powermon_real{source="solar"}[5m]))`
	denominator := `sum(count_over_time(powermon_real{source="solar"}[5m]))`
	query := numerator + " / " + denominator

	result, warnings, err := v1api.QueryRange(ctx, query, r)
	if err != nil {
		log.Fatalf("QueryRange: %v", err)
	}
	if len(warnings) > 0 {
		log.Fatalf("Warnings: %v", warnings)
	}

	matrix := result.(model.Matrix)
	if len(matrix) != 1 {
		log.Fatalf("Unexpected sample shape: %v", len(matrix))
	}
	return matrix[0]
}

// solcast Measurements API
// https://docs.solcast.com.au/#measurements-rooftop-site
type Measurement struct {
	PeriodEnd  time.Time `json:"period_end"`
	Period     string    `json:"period"`
	TotalPower float64   `json:"total_power"`
}
type Measurements struct {
	Measurements []Measurement `json:"measurements"`
}

// Take 24 hours worth of samples, throw away those at night with no power production,
// and return an array of solcast Measurement structs.
func TrimSamples(samples *model.SampleStream) []Measurement {
	utc, err := time.LoadLocation("UTC")
	if err != nil {
		log.Fatalf("time.LoadLocation UTC: %v", err)
	}
	var values []Measurement
	for _, s := range samples.Values {
		if s.Value > 10.0 {
			var m Measurement
			// From staring at graph of power data versus what this program outputs,
			// 1) prometheus timestamp is the end of the sample period and
			// 2) it does not return a sample for the final 5m partially full bucket
			m.PeriodEnd = s.Timestamp.Time().In(utc)
			m.Period = "PT5M"
			m.TotalPower = float64(s.Value) / 1000.0 // Watts -> kiloWatts
			values = append(values, m)
		}
	}

	return values
}

func UploadToSolcast(measurements []Measurement, apiKey string, resourceId string) {
	var m Measurements
	m.Measurements = measurements

	js, err := json.Marshal(m)
	if err != nil {
		log.Fatalf("json.Marshal: %v", err)
	}

	url := "https://api.solcast.com.au" + "/rooftop_sites/" + resourceId + "/measurements"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(js))
	if err != nil {
		log.Fatalln(err)
	}
	req.Header.Set("User-Agent", "https://github.com/DentonGentry/powerwall")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Solcast Measurement POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("Solcast Measurement POST status=%v", resp.StatusCode)
		} else {
			log.Fatalf("HTTP Status = %d\n%s\n", resp.StatusCode, body)
		}
	}
}

func main() {
	apiKeyPtr := flag.String("solcast_api_key", "",
		"https://toolkit.solcast.com.au/register/hobbyist")
	resourceIdPtr := flag.String("solcast_resource_id", "",
		"https://toolkit.solcast.com.au/register/hobbyist")
	flag.Parse()

	apiKey := *apiKeyPtr
	if apiKey == "" {
		apiKey = os.Getenv("SOLCAST_API_KEY")
	}
	if apiKey == "" {
		log.Fatal("Solcast API Key must be provided using --solcast_api_key")
	}

	resourceId := *resourceIdPtr
	if resourceId == "" {
		resourceId = os.Getenv("SOLCAST_RESOURCE_ID")
	}
	if resourceId == "" {
		log.Fatal("Solcast Resource Id must be provided using --solcast_resource_id")
	}

	samples := GetSolarSamples()
	measurements := TrimSamples(samples)
	UploadToSolcast(measurements, apiKey, resourceId)
}
