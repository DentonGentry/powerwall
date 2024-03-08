// Copyright (c), Denton Gentry <dgentry@decarbon.earth>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

type TeslaOuterResponse struct {
	response TeslaInnerResponse
}

type TeslaInnerResponse struct {
	SolarPower        int     // `json:"solar_power"`
	EnergyLeft        float64 // `json:"energy_left"`
	TotalPackEnergy   int     // `json:"total_pack_energy"`
	PercentageCharged float64 // `json:"percentage_charged"`
	BackupCapable     bool    // `json:"backup_capable"`
	BatteryPower      int     // `json:"battery_power"`
	LoadPower         int     // `json:"load_power"`
	GridStatus        string  // `json:"grid_status"`
	GridPower         int     // `json:"grid_power"`
	IslandStatus      string  // `json:"island_status"`
	StormModeActive   bool    // `json:"storm_mode_active"`
	Timestamp         string  // `json:"timestamp"`
}

func updateMetricsFromTesla(tesla *TeslaState) {
	url := tesla.apiUrl + "/live_status"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		fetchFailed.Add(1)
		return
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	res, err := tesla.c.Do(req)
	if err != nil {
		fetchFailed.Add(1)
		return
	}
	if res.StatusCode == 403 {
		fetchAuthFailed.Add(1)
		return
	}
	if res.StatusCode != 200 {
		fetchFailed.Add(1)
		return
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		fetchFailed.Add(1)
		return
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	var r TeslaOuterResponse
	decoder.Decode(r)
}
