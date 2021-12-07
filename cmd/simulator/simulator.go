//
// Copyright (C) 2021 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"github.com/edgexfoundry/device-rfid-llrp-go/internal/llrp"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	sim := llrp.NewSimulator()
	cf := parseFlags()
	if err := sim.Initialize(cf); err != nil {
		fmt.Printf("Error initializing simulator: %v\n", err)
		os.Exit(1)
	}

	if err := sim.StartAsync(); err != nil {
		sim.Logger.Printf("Error starting simulator: %v\n", err)
		os.Exit(1)
	}

	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	// wait for SIGINT or SIGTERM (block forever until cancelled)
	s := <-signals
	sim.Logger.Printf("Received '%s' signal from OS.\n", s.String())

	if err := sim.Shutdown(); err != nil {
		sim.Logger.Printf("Error shutting down simulator: %v\n", err)
	}
	sim.Logger.Println("Exiting.")
}

// parseFlags configures the command line flags and parses them into a SimulatorConfigFlags struct.
func parseFlags() llrp.SimulatorConfigFlags {
	cf := llrp.NewSimulatorConfigFlags()

	flag.BoolVar(&cf.Silent, "s", cf.Silent, "silent")
	flag.BoolVar(&cf.Silent, "silent", cf.Silent, "silent")

	flag.IntVar(&cf.LLRPPort, "p", cf.LLRPPort, "llrp port")
	flag.IntVar(&cf.LLRPPort, "port", cf.LLRPPort, "llrp port")
	flag.IntVar(&cf.LLRPPort, "llrp-port", cf.LLRPPort, "llrp port")

	flag.IntVar(&cf.APIPort, "P", cf.APIPort, "api port")
	flag.IntVar(&cf.APIPort, "api-port", cf.APIPort, "api port")

	flag.IntVar(&cf.AntennaCount, "a", cf.AntennaCount, "antenna count")
	flag.IntVar(&cf.AntennaCount, "antenna-count", cf.AntennaCount, "antenna count")

	flag.IntVar(&cf.TagPopulation, "t", cf.TagPopulation, "tag population count")
	flag.IntVar(&cf.TagPopulation, "tags", cf.TagPopulation, "tag population count")
	flag.IntVar(&cf.TagPopulation, "tag-population", cf.TagPopulation, "tag population count")

	flag.StringVar(&cf.BaseEPC, "e", cf.BaseEPC, "Base EPC")
	flag.StringVar(&cf.BaseEPC, "epc", cf.BaseEPC, "Base EPC")
	flag.StringVar(&cf.BaseEPC, "base-epc", cf.BaseEPC, "Base EPC")

	flag.IntVar(&cf.MaxRSSI, "M", cf.MaxRSSI, "max rssi")
	flag.IntVar(&cf.MaxRSSI, "max", cf.MaxRSSI, "max rssi")
	flag.IntVar(&cf.MaxRSSI, "max-rssi", cf.MaxRSSI, "max rssi")

	flag.IntVar(&cf.MinRSSI, "m", cf.MinRSSI, "min rssi")
	flag.IntVar(&cf.MinRSSI, "min", cf.MinRSSI, "min rssi")
	flag.IntVar(&cf.MinRSSI, "min-rssi", cf.MinRSSI, "min rssi")

	flag.StringVar(&cf.Filename, "f", cf.Filename, "config filename")
	flag.StringVar(&cf.Filename, "file", cf.Filename, "config filename")

	flag.IntVar(&cf.ReadRate, "r", cf.ReadRate, "read rate (tags/s)")
	flag.IntVar(&cf.ReadRate, "rate", cf.ReadRate, "read rate (tags/s)")
	flag.IntVar(&cf.ReadRate, "read-rate", cf.ReadRate, "read rate (tags/s)")

	flag.StringVar(&cf.ReaderID, "id", cf.ReaderID, "Override Reader ID suffix with 6-digit hex string")
	flag.StringVar(&cf.ReaderID, "reader-id", cf.ReaderID, "Override Reader ID suffix with 6-digit hex string")

	flag.Parse()

	if cf.Filename == "" {
		fmt.Printf("Missing required argument -f/--file\n")
		os.Exit(2)
	}

	log.Printf("flags: %+v", cf)

	return cf
}
