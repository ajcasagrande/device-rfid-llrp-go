//
// Copyright (C) 2021 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"github.com/edgexfoundry/device-rfid-llrp-go/internal/llrp"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	sim, err := llrp.CreateSimulator()
	if err != nil {
		fmt.Printf("Error creating simulator: %v", err)
		os.Exit(1)
	}

	if err = sim.StartAsync(); err != nil {
		sim.Logger.Printf("Error starting simulator: %v\n", err)
		os.Exit(1)
	}

	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	// wait for SIGINT or SIGTERM (block forever until cancelled)
	s := <-signals
	sim.Logger.Printf("Received '%s' signal from OS.\n", s.String())

	if err = sim.Shutdown(); err != nil {
		sim.Logger.Printf("Error shutting down simulator: %v\n", err)
	}
	sim.Logger.Println("Exiting.")
}
