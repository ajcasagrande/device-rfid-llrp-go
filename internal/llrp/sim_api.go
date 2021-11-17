//
// Copyright (C) 2021 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package llrp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

const (
	maxBodyBytes = 100 * 1024
)

func (sim *Simulator) setupAPIRoutes() {
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/api/v1/config", sim.apiConfigHandler)
}

func (sim *Simulator) serveAPI() {
	server := &http.Server{Addr: fmt.Sprintf(":%d", sim.flags.APIPort)}
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			sim.Logger.Fatalf("API Server ListenAndServe failed: %v", err)
		}
	}()

	sim.Logger.Printf("Serving API @ http://localhost:%d", sim.flags.APIPort)

	// wait for done signal
	<-sim.done

	sim.Logger.Println("Shutting down API Server")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		sim.Logger.Printf("API Server Shutdown failed: %v", err)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/html/index.html")
}

func (sim *Simulator) apiConfigHandler(w http.ResponseWriter, r *http.Request) {
	data, err := ioutil.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	defer r.Body.Close()
	if err != nil {
		msg := fmt.Sprintf("Failed to read POST body: %v", err)
		sim.Logger.Println(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	cfg := SimulatorConfigFlags{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		msg := fmt.Sprintf("Failed to unmarshal config data json: %v", err)
		sim.Logger.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	sim.Logger.Printf("Pushing new config data: %s", data)
	// send the new config to be handled in the taskLoop
	sim.flagUpdateCh <- cfg
	w.WriteHeader(http.StatusOK)
}
