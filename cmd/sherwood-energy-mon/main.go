// Copyright (c) 2020, Denton Gentry <dgentry@decarbon.earth>
// All rights reserved.

// This source code is licensed under the BSD-style license found in the
// LICENSE file in the root directory of this source tree.

package main

import (
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	solarPower = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "sherwood_energymon_solar_watts",
		Help: "Instantaneous solar power production in Watts.",
	})
	powerwallEnergy = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "sherwood_energymon_powerwall_energy_wh",
		Help: "Instantaneous energy stored in Powerwall(s) in Watt-hours.",
	})
	powerwallCapacity = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "sherwood_energymon_powerwall_capacity_wh",
		Help: "Energy capacity of Powerwall(s) in Watt-hours.",
	})
	powerwallPower = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "sherwood_energymon_powerwall_watts",
		Help: "Instantaneous powerwall power production in Watts (can be negative).",
	})
	houseLoadPower = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "sherwood_energymon_house_load_watts",
		Help: "Instantaneous power demand from the house in watts.",
	})
	gridPower = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "sherwood_energymon_grid_watts",
		Help: "Instantaneous power drawn from the grid in watts (can be negative).",
	})
	gridPresent = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "sherwood_energymon_grid_present",
		Help: "Whether power grid is powered (1) or not (0).",
	})
	stormModeActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "sherwood_energymon_grid_active",
		Help: "Whether storm mode is active (1) or not (0).",
	})
	onGrid = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "sherwood_energymon_on_grid",
		Help: "Whether Powerwall is on grid (1) or not (0).",
	})
)

func UpdateMetricsLoop() {
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-t.C:
		}
	}
}

func ServePrometheusMetrics() {
	http.Handle("/metrics", promhttp.Handler())
	prometheus.MustRegister(solarPower)
	prometheus.MustRegister(powerwallEnergy)
	prometheus.MustRegister(powerwallCapacity)
	prometheus.MustRegister(powerwallPower)
	prometheus.MustRegister(houseLoadPower)
	prometheus.MustRegister(gridPower)
	prometheus.MustRegister(gridPresent)
	prometheus.MustRegister(stormModeActive)
	prometheus.MustRegister(onGrid)

	go UpdateMetricsLoop()
	log.Fatal(http.ListenAndServe("localhost:8080", nil))
}

func main() {
	ServePrometheusMetrics()
}
