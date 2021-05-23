// Copyright (c) 2021, Denton Gentry <dgentry@decarbon.earth>
// All rights reserved.

// This source code is licensed under the BSD-style license found in the
// LICENSE file in the root directory of this source tree.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"github.com/DentonGentry/powerwall/v2/internal/pkg/solcast"
)

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

	p, _ := solcast.GetSolarProductionForecast(apiKey, resourceId)
	fmt.Println(p)

	t := time.Now().Add(time.Hour * 3).UTC()
	fmt.Println(t)

	estimate := sort.Search(len(p), func(i int) bool { return p[i].End.After(t) })
	fmt.Println(estimate)
}
