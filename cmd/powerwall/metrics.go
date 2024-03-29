// Copyright (c), Denton Gentry <dgentry@decarbon.earth>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
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
	fetchSuccess = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "sherwood_energymon_fetch_success",
		Help: "Number of successful fetches from Tesla Energy API.",
	})
	fetchFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "sherwood_energymon_fetch_failed",
		Help: "Number of failed fetches from Tesla Energy API.",
	})
	fetchAuthFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "sherwood_energymon_fetch_failed",
		Help: "Number of attempted fetches from Tesla Energy API prior to authentication.",
	})
	refreshSuccess = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "sherwood_energymon_refresh_success",
		Help: "Number of successful refreshes of the access token from Tesla Energy API.",
	})
	refreshFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "sherwood_energymon_refresh_failed",
		Help: "Number of failed refreshes of the access token from Tesla Energy API.",
	})
)

func UpdateMetricsLoop() {
	t := time.NewTicker(300 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			updateMetricsFromTesla(&Tesla)
		}
	}
}

func initPrometheusMetrics() {
	prometheus.MustRegister(solarPower)
	prometheus.MustRegister(powerwallEnergy)
	prometheus.MustRegister(powerwallCapacity)
	prometheus.MustRegister(powerwallPower)
	prometheus.MustRegister(houseLoadPower)
	prometheus.MustRegister(gridPower)
	prometheus.MustRegister(gridPresent)
	prometheus.MustRegister(stormModeActive)
	prometheus.MustRegister(onGrid)
}
