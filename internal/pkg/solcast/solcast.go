// Copyright (c) 2021, Denton Gentry <dgentry@decarbon.earth>
// All rights reserved.

// This source code is licensed under the BSD-style license found in the
// LICENSE file in the root directory of this source tree.

package solcast

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type SolarPrediction struct {
	End    time.Time
	KWatts float64
}

// solcast Forecast API
// https://docs.solcast.com.au/#forecasts-rooftop-site
type Forecast struct {
	PvEstimate   float64   `json:"pv_estimate"`
	PvEstimate10 float64   `json:"pv_estimate10"`
	PvEstimate90 float64   `json:"pv_estimate90"`
	PeriodEnd    time.Time `json:"period_end"`
	Period       string    `json:"period"`
}

type Forecasts struct {
	Forecasts []Forecast `json:"forecasts"`
}

// Return an array of predicted solar production, stretching at least 24 hours into the future.
func GetSolarProductionForecast(apiKey, resourceId string) (prediction []SolarPrediction, err error) {
	url := "https://api.solcast.com.au" + "/rooftop_sites/" + resourceId + "/forecasts?hours=48"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	req.Header.Set("User-Agent", "https://github.com/DentonGentry/powerwall")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Solcast Measurement POST: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Solcast Measurement POST status=%v", resp.StatusCode)
			return nil, err
		} else {
			log.Println(string(body))
			return nil, err
		}
	}

	var result Forecasts
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&result)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	prediction = make([]SolarPrediction, len(result.Forecasts))
	for idx, forecast := range result.Forecasts {
		prediction[idx].KWatts = forecast.PvEstimate
		prediction[idx].End = forecast.PeriodEnd
	}

	return prediction, nil
}
