// Copyright (c), Denton Gentry <dgentry@decarbon.earth>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	initPrometheusMetrics()
	token := ReadSavedState()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "ok")
		fmt.Println("Root Handler")
	})
	http.Handle("/metrics", promhttp.Handler())

	go UpdateMetricsLoop()
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}
