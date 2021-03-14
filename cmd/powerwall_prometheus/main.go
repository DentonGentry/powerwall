// Copyright (c) 2020, Denton Gentry <dgentry@decarbon.earth>
// All rights reserved.

// This source code is licensed under the BSD-style license found in the
// LICENSE file in the root directory of this source tree.

package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	realPower = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "powermon_real",
		Help: "Real power produced/consumed.",
	},
		[]string{"source"},
	)
	reactivePower = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "powermon_reactive",
		Help: "Reactive power produced/consumed.",
	},
		[]string{"source"},
	)
	apparentPower = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "powermon_apparent",
		Help: "Apparent power produced/consumed.",
	},
		[]string{"source"},
	)
	gridConnected = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "powermon_grid_connected",
		Help: "Grid power connected(1) or outage(0).",
	})
	batteryCharge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "powermon_battery_charge",
		Help: "Battery charge percentage.",
	})
)

// Tesla Backup Gateway is provisioned with a self-signed SSL
// server certificate at manufacture. This can be retrieved using:
//     echo quit | openssl s_client -showcerts -servername powerwall \
//         -connect 10.1.1.1:443 >tbg_cert.pem
// and the filename passed in using --certfile=/path/to/tbg_cert.pem
var teslacert = []byte("")

// port number to listen on
var tbg_port = 0

func PowerwallHttpsClient() *http.Client {
	var client = &http.Client{}
	client.Timeout = 10 * time.Second
	client.Transport = http.DefaultTransport

	if len(teslacert) > 0 {
		// Make a copy of the system's Certificate Authorities, and append
		// the self-signed certificate from our Tesla Backup Gateway.
		newRootCAs, err := x509.SystemCertPool()
		if newRootCAs == nil || err != nil {
			log.Fatalf("Failed to get SystemCertPool: %v", err)
		}
		if ok := newRootCAs.AppendCertsFromPEM(teslacert); !ok {
			log.Fatalln("Failed to append Tesla PEM to System CAs")
		}
		client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{RootCAs: newRootCAs}
	} else {
		client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	return client
}

func GetCookie(client *http.Client, passcode string) string {
	type Login struct {
		Username   string `json:"username"`
		Password   string `json:"password"`
		ForceSmOff bool   `json:"force_sm_off"`
	}
	login := Login{"customer", passcode, false}
	js, err := json.Marshal(login)
	if err != nil {
		log.Fatalf("json.Marshal: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://powerwall/api/login/Basic",
		bytes.NewBuffer(js))
	if err != nil {
		log.Fatalf("http.NewRequest: %v", err)
	}
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("client.Do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("ReadAll failed: %v", err)
		}
		var data map[string]interface{}
		json.Unmarshal(body, &data)
		return data["token"].(string)
	} else {
		log.Fatalf("login/Basic failed: %v", resp.StatusCode)
	}

	return ""
}

func GetFromPowerwall(client *http.Client, cookie string, url string) map[string]interface{} {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Fatalln(err)
	}
	req.AddCookie(&http.Cookie{Name: "AuthCookie", Value: cookie})

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	decoder := json.NewDecoder(resp.Body)
	decoder.UseNumber() // don't convert large integers to float
	err = decoder.Decode(&result)
	if err != nil {
		log.Fatalln(err)
	}

	return result
}

func GetStatsFromPowerwall(passcode string, addr string) map[string]float64 {
	client := PowerwallHttpsClient()
	stats := make(map[string]float64)
	var err error

	base_url := "https://" + addr
	cookie := GetCookie(client, passcode)
	result := GetFromPowerwall(client, cookie, base_url+"/api/meters/aggregates")

	// *** /api/meters/aggregates battery ***
	battery := result["battery"].(map[string]interface{})
	stats["battery_real"], err = battery["instant_power"].(json.Number).Float64()
	if err != nil {
		log.Fatalf("battery instant_power: %v", err)
	}

	stats["battery_reactive"], err = battery["instant_reactive_power"].(json.Number).Float64()
	if err != nil {
		log.Fatalf("battery instant_reactive_power: %v", err)
	}

	stats["battery_apparent"], err = battery["instant_apparent_power"].(json.Number).Float64()
	if err != nil {
		log.Fatalf("battery instant_apparent_power: %v", err)
	}

	// *** /api/meters/aggregates solar ***
	solar := result["solar"].(map[string]interface{})
	stats["solar_real"], err = solar["instant_power"].(json.Number).Float64()
	if err != nil {
		log.Fatalf("solar instant_power: %v", err)
	}

	stats["solar_reactive"], err = solar["instant_reactive_power"].(json.Number).Float64()
	if err != nil {
		log.Fatalf("solar instant_reactive_power: %v", err)
	}

	stats["solar_apparent"], err = solar["instant_apparent_power"].(json.Number).Float64()
	if err != nil {
		log.Fatalf("solar instant_apparent_power: %v", err)
	}

	// *** /api/meters/aggregates load ***
	load := result["load"].(map[string]interface{})
	stats["house_real"], err = load["instant_power"].(json.Number).Float64()
	if err != nil {
		log.Fatalf("load instant_power: %v", err)
	}

	stats["house_reactive"], err = load["instant_reactive_power"].(json.Number).Float64()
	if err != nil {
		log.Fatalf("load instant_reactive_power: %v", err)
	}

	stats["house_apparent"], err = load["instant_apparent_power"].(json.Number).Float64()
	if err != nil {
		log.Fatalf("load instant_apparent_power: %v", err)
	}

	// *** /api/system_status/sce ***
	result = GetFromPowerwall(client, cookie, base_url+"/api/system_status/soe")
	stats["charge"], err = result["percentage"].(json.Number).Float64()
	if err != nil {
		log.Fatalf("sce percentage: %v", err)
	}

	// *** /api/system_status/grid_status ***
	result = GetFromPowerwall(client, cookie, base_url+"/api/system_status/grid_status")
	grid_status, ok := result["grid_status"].(string)
	if !ok {
		log.Fatalf("grid_status: %v", err)
	}
	if grid_status == "SystemGridConnected" {
		stats["grid_connected"] = 1.0
	} else {
		stats["grid_connected"] = 0.0
	}

	return stats
}

func UpdateMetricsLoop(passcode string, addr string) {
	for {
		start := time.Now()
		stats := GetStatsFromPowerwall(passcode, addr)

		realPower.With(prometheus.Labels{"source": "battery"}).Set(stats["battery_real"])
		reactivePower.With(prometheus.Labels{"source": "battery"}).Set(stats["battery_reactive"])
		apparentPower.With(prometheus.Labels{"source": "battery"}).Set(stats["battery_apparent"])

		realPower.With(prometheus.Labels{"source": "solar"}).Set(stats["solar_real"])
		reactivePower.With(prometheus.Labels{"source": "solar"}).Set(stats["solar_reactive"])
		apparentPower.With(prometheus.Labels{"source": "solar"}).Set(stats["solar_apparent"])

		realPower.With(prometheus.Labels{"source": "house"}).Set(stats["house_real"])
		reactivePower.With(prometheus.Labels{"source": "house"}).Set(stats["house_reactive"])
		apparentPower.With(prometheus.Labels{"source": "house"}).Set(stats["house_apparent"])

		gridConnected.Set(stats["grid_connected"])

		batteryCharge.Set(stats["charge"])

		elapsed := time.Now().Sub(start)
		sleep := time.Duration(8000.0-elapsed.Milliseconds()) * time.Millisecond
		time.Sleep(sleep)
	}
}

func ServePrometheusMetrics(passcode string, addr string) {
	http.Handle("/metrics", promhttp.Handler())
	prometheus.MustRegister(realPower)
	prometheus.MustRegister(reactivePower)
	prometheus.MustRegister(apparentPower)
	prometheus.MustRegister(gridConnected)
	prometheus.MustRegister(batteryCharge)
	go UpdateMetricsLoop(passcode, addr)
	log.Fatal(http.ListenAndServe("localhost:"+strconv.Itoa(tbg_port), nil))
}

func main() {
	passcodePtr := flag.String("passcode", "",
		"last 5 digits of the Tesla Backup Gateway serial number, ex: 00A1B")
	addrPtr := flag.String("addr", "",
		"address of the Tesla Backup Gateway, ex: 'powerwall' in DNS or '192.168.1.9'")
	certFilePtr := flag.String("certfile", "",
		"path to the public certificate of the Tesla Backup Gateway. "+
			"See https://github.com/DentonGentry/powerwall")
	portPtr := flag.Int("port", 8081, "port number to listen on (default 8081)")
	flag.Parse()

	passcode := *passcodePtr
	if passcode == "" {
		passcode = os.Getenv("TESLA_BACKUP_GATEWAY_PASSCODE")
	}
	if passcode == "" {
		log.Fatalf("Tesla Backup Gateway passcode must be provided in --passcode.")
	}

	addr := *addrPtr
	if addr == "" {
		addr = os.Getenv("TESLA_BACKUP_GATEWAY_ADDR")
	}
	if addr == "" {
		log.Fatalf("Tesla Backup Gateway address must be provided in --addr")
	}

	certFile := *certFilePtr
	if certFile == "" {
		certFile = os.Getenv("TESLA_BACKUP_GATEWAY_CERT")
	}
	if certFile != "" {
		c, err := ioutil.ReadFile(certFile)
		if err != nil {
			log.Fatalf("Read cert file: %v", err)
		}
		teslacert = c
	}

	tbg_port = *portPtr

	ServePrometheusMetrics(passcode, addr)
}
