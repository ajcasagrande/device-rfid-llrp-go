//
// Copyright (C) 2021 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package llrp

import (
	"encoding/base64"
	"encoding/json"
	"github.com/pkg/errors"
	"io/ioutil"
	"log"
	"os"
	"sync/atomic"
	"time"
)

type Simulator struct {
	filename string
	config   SimulatorConfig
	emu      *TestEmulator
	Logger   *log.Logger

	reading bool
}

type SimulatorConfig struct {
	Silent         bool
	LLRPPort       int
	ManagementPort int

	ReaderCapabilities GetReaderCapabilitiesResponse
	ReaderConfig       GetReaderConfigResponse
}

// CreateSimulator parses config file and sets up a new simulator but does not start it
func CreateSimulator(configFilename string) (*Simulator, error) {
	sim := Simulator{
		filename: configFilename,
		Logger:   log.Default(),
	}
	sim.Logger.Printf("Loading simulator config from '%s'", configFilename)
	if err := sim.loadConfig(); err != nil {
		return nil, err
	}
	sim.Logger.Println("Successfully loaded simulator config.")

	sim.emu = NewTestEmulator(sim.config.Silent)
	sim.SetCannedMessageResponses()

	return &sim, nil
}

func (sim *Simulator) SetCannedMessageResponses() {
	successStatus := LLRPStatus{
		Status: StatusSuccess,
	}

	sim.emu.SetResponse(MsgGetReaderConfig, &sim.config.ReaderConfig)
	sim.emu.SetResponse(MsgGetReaderCapabilities, &sim.config.ReaderCapabilities)

	sim.emu.SetResponse(MsgSetReaderConfig, &SetReaderConfigResponse{LLRPStatus: successStatus})
	sim.emu.SetResponse(MsgAddROSpec, &AddROSpecResponse{LLRPStatus: successStatus})
	sim.emu.SetResponse(MsgAddAccessSpec, &AddAccessSpecResponse{LLRPStatus: successStatus})
	sim.emu.SetResponse(MsgDeleteROSpec, &DeleteROSpecResponse{LLRPStatus: successStatus})
	sim.emu.SetResponse(MsgDeleteAccessSpec, &DeleteAccessSpecResponse{LLRPStatus: successStatus})
	sim.emu.SetResponse(MsgEnableAccessSpec, &EnableAccessSpecResponse{LLRPStatus: successStatus})
	sim.emu.SetResponse(MsgDisableAccessSpec, &DisableAccessSpecResponse{LLRPStatus: successStatus})

	sim.emu.SetHandler(MsgEnableROSpec, HandlerCallbackFunc(func(td *TestDevice, msg Message) {
		sim.reading = true
		sim.Logger.Println("Reading is enabled!")
		td.write(msg.id, &EnableROSpecResponse{LLRPStatus: successStatus})
	}))

	sim.emu.SetHandler(MsgDisableROSpec, HandlerCallbackFunc(func(td *TestDevice, msg Message) {
		sim.reading = false
		sim.Logger.Println("Reading is disabled!")
		td.write(msg.id, &DisableROSpecResponse{LLRPStatus: successStatus})
	}))

	sim.emu.SetHandler(MsgKeepAliveAck, HandlerCallbackFunc(func(td *TestDevice, msg Message) {
		sim.Logger.Println("Received KeepAliveAck")
		// No response expected
	}))

	sim.emu.SetHandler(MsgCustomMessage, HandlerCallbackFunc(func(td *TestDevice, msg Message) {
		custom := &CustomMessage{}
		if err := msg.UnmarshalTo(custom); err != nil {
			sim.Logger.Printf("Failed to unmarshal async event from LLRP. error: %v\n", err)
			return
		}
		td.write(msg.id, custom)
	}))
}

// HandlerCallback can be implemented to handle certain messages received from the Reader.
// See Client.WithHandler for more information.
type HandlerCallback interface {
	Handle(td *TestDevice, msg Message)
}

// HandlerCallbackFunc can wrap a function so it can be used as a HandlerCallback.
type HandlerCallbackFunc func(td *TestDevice, msg Message)

// Handle implements HandlerCallback for HandlerCallbackFunc by calling the function.
func (hcf HandlerCallbackFunc) Handle(td *TestDevice, msg Message) {
	hcf(td, msg)
}

// StartAsync starts processing llrp messages async
// todo: add management rest server
func (sim *Simulator) StartAsync() error {
	sim.Logger.Printf("Starting simulator on llrp port: %d\n", sim.config.LLRPPort)
	if err := sim.emu.StartAsync(sim.config.LLRPPort); err != nil {
		return err
	}
	go sim.TagLoop()
	go sim.KeepAliveLoop()

	return nil
}

// Shutdown stops the llrp message handler
// todo: stop management rest server
func (sim *Simulator) Shutdown() error {
	sim.Logger.Println("Shutting down simulator...")
	return sim.emu.Shutdown()
}

func (sim *Simulator) loadConfig() error {
	f, err := os.Open(sim.filename)
	if err != nil {
		return errors.Wrapf(err, "error opening simulator config file '%s'", sim.filename)
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return errors.Wrapf(err, "error reading simulator config file '%s'", sim.filename)
	}

	err = json.Unmarshal(data, &sim.config)
	if err != nil {
		return errors.Wrap(err, "error unmarshalling simulator config from json")
	}

	return nil
}

func peakRssiPtr(val int8) *PeakRSSI {
	rssi := PeakRSSI(val)
	return &rssi
}

func antIdPtr(val uint16) *AntennaID {
	ant := AntennaID(val)
	return &ant
}

func firstSeenPtr() *FirstSeenUTC {
	tm := FirstSeenUTC(time.Now().UnixNano() / 1e3)
	return &tm
}

func (sim *Simulator) KeepAliveLoop() {
	// todo: read keep alive section of config to determine this
	ticker := time.NewTicker(30 * time.Second)
	for {
		select {
		case <-ticker.C:
			sim.SendKeepAlive()
		}
	}
}

func (sim *Simulator) TagLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <-ticker.C:
			sim.SendTagData()
		}
	}
}

func (sim *Simulator) SendTagData() {
	if !sim.reading {
		return
	}

	epc, _ := base64.StdEncoding.DecodeString("MACrze8AAAAAAAAD")
	data := &ROAccessReport{TagReportData: []TagReportData{
		{
			EPC96:                                   EPC96{
				EPC: epc,
			},
			AntennaID:                               antIdPtr(1),
			PeakRSSI:                                peakRssiPtr(-47),
			ChannelIndex:                            nil,
			FirstSeenUTC:                            firstSeenPtr(),
		},
	}}
	sim.emu.devicesMu.Lock()
	for d := range sim.emu.devices {
		sim.Logger.Printf("sending tag read data")
		d.write(messageID(atomic.AddUint32((*uint32)(&d.mid), 1)), data)
	}
	sim.emu.devicesMu.Unlock()
}

func (sim *Simulator) SendKeepAlive() {
	ka := &KeepAlive{}

	sim.emu.devicesMu.Lock()
	for d := range sim.emu.devices {
		sim.Logger.Printf("sending keep alive")
		d.write(messageID(atomic.AddUint32((*uint32)(&d.mid), 1)), ka)
	}
	sim.emu.devicesMu.Unlock()
}
