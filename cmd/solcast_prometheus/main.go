// Copyright (c) 2021, Denton Gentry <dgentry@decarbon.earth>
// All rights reserved.

// This source code is licensed under the BSD-style license found in the
// LICENSE file in the root directory of this source tree.

package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/DentonGentry/powerwall/v2/internal/pkg/solcast"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	forecastPower = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "powermon_forecast",
		Help: "Power generation forecast for this time",
	})
)

func GetForecast(apiKey, resourceId string) (forecast []solcast.SolarPrediction, timestamp time.Time) {
	timestamp = time.Now()
	forecast, err := solcast.GetSolarProductionForecast(apiKey, resourceId)
	if err != nil {
		log.Printf("solcast forecast failed: %v\n", err)
		return nil, timestamp
	}
	return forecast, timestamp
}

func UpdateMetricsLoop(apiKey, resourceId string) {
	forecast, timestamp := GetForecast(apiKey, resourceId)
	for {
		start := time.Now()

		estimate := 0.0
		if forecast != nil {
			idx := sort.Search(len(forecast),
				func(i int) bool { return forecast[i].End.After(start.UTC()) })
			estimate = forecast[idx].KWatts
		}
		forecastPower.Set(estimate)

		if start.After(timestamp.Add(time.Hour*23)) ||
			(forecast == nil && start.After(timestamp.Add(time.Hour))) {
			forecast, timestamp = GetForecast(apiKey, resourceId)
		}

		elapsed := time.Now().Sub(start)
		sleep := time.Duration(8000.0-elapsed.Milliseconds()) * time.Millisecond
		time.Sleep(sleep)
	}
}

func main() {
	portPtr := flag.Int("port", 8082, "port number to listen on (default 8082)")
	apiKeyPtr := flag.String("solcast_api_key", "",
		"https://toolkit.solcast.com.au/register/hobbyist")
	resourceIdPtr := flag.String("solcast_resource_id", "",
		"https://toolkit.solcast.com.au/register/hobbyist")
	flag.Parse()

	port := *portPtr

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

	http.Handle("/metrics", promhttp.Handler())
	prometheus.MustRegister(forecastPower)
	go UpdateMetricsLoop(apiKey, resourceId)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(port), nil))
}
