//
// Copyright (C) 2021 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package llrp

import (
	"encoding/hex"
	"encoding/json"
	"github.com/pkg/errors"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"os"
	"sync/atomic"
	"time"
)

const (
	defaultReadInterval = 50 * time.Millisecond
)

const (
	rssiMin = -95
	rssiMax = -55
)

var (
	successStatus = LLRPStatus{
		Status: StatusSuccess,
	}

	rssiStrong    = rssiMax - int(math.Floor((rssiMax-rssiMin)/3))
	rssiWeak      = rssiMin + int(math.Floor((rssiMax-rssiMin)/3))
	rssiRange     = rssiStrong - rssiWeak
	rssiFullRange = rssiMax - rssiMin
)

type Simulator struct {
	filename string
	config   SimulatorConfig
	emu      *TestEmulator
	Logger   *log.Logger

	reading bool

	kaTicker *time.Ticker
	roTicker *time.Ticker

	done chan bool
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
		kaTicker: time.NewTicker(1 * time.Hour),
		roTicker: time.NewTicker(1 * time.Hour),
		done:     make(chan bool),
	}
	// stop the tickers because they start automatically
	sim.kaTicker.Stop()
	sim.roTicker.Stop()

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
	sim.emu.SetResponse(MsgGetReaderConfig, &sim.config.ReaderConfig)
	sim.emu.SetResponse(MsgGetReaderCapabilities, &sim.config.ReaderCapabilities)

	sim.emu.SetResponse(MsgAddROSpec, &AddROSpecResponse{LLRPStatus: successStatus})
	sim.emu.SetResponse(MsgAddAccessSpec, &AddAccessSpecResponse{LLRPStatus: successStatus})
	sim.emu.SetResponse(MsgDeleteROSpec, &DeleteROSpecResponse{LLRPStatus: successStatus})
	sim.emu.SetResponse(MsgDeleteAccessSpec, &DeleteAccessSpecResponse{LLRPStatus: successStatus})
	sim.emu.SetResponse(MsgEnableAccessSpec, &EnableAccessSpecResponse{LLRPStatus: successStatus})
	sim.emu.SetResponse(MsgDisableAccessSpec, &DisableAccessSpecResponse{LLRPStatus: successStatus})

	sim.emu.SetHandler(MsgSetReaderConfig, HandlerCallbackFunc(sim.handleSetReaderConfig))
	sim.emu.SetHandler(MsgEnableROSpec, HandlerCallbackFunc(sim.handleEnableROSpec))
	sim.emu.SetHandler(MsgDisableROSpec, HandlerCallbackFunc(sim.handleDisableROSpec))
	sim.emu.SetHandler(MsgCustomMessage, HandlerCallbackFunc(sim.handleCustomMessage))
	sim.emu.SetHandler(MsgKeepAliveAck, HandlerCallbackFunc(sim.handleKeepAliveAck))
}

func (sim *Simulator) handleKeepAliveAck(_ *TestDevice, msg Message) {
	sim.Logger.Printf("Received KeepAliveAck mid: %v\n", msg.id)
	// No response expected
}

func (sim *Simulator) handleCustomMessage(td *TestDevice, msg Message) {
	custom := &CustomMessage{}
	if err := msg.UnmarshalTo(custom); err != nil {
		sim.Logger.Printf("Failed to unmarshal async event from LLRP. error: %v\n", err)
		return
	}
	td.write(msg.id, custom)
}

func (sim *Simulator) handleEnableROSpec(td *TestDevice, msg Message) {
	sim.reading = true
	sim.roTicker.Reset(defaultReadInterval)
	sim.Logger.Println("Reading is enabled!")
	td.write(msg.id, &EnableROSpecResponse{LLRPStatus: successStatus})
}

func (sim *Simulator) handleDisableROSpec(td *TestDevice, msg Message) {
	sim.reading = false
	sim.roTicker.Stop()
	sim.Logger.Println("Reading is disabled!")
	td.write(msg.id, &DisableROSpecResponse{LLRPStatus: successStatus})
}

func (sim *Simulator) handleSetReaderConfig(td *TestDevice, msg Message) {
	cfg := &SetReaderConfig{}
	if err := msg.UnmarshalTo(cfg); err != nil {
		sim.Logger.Printf("Failed to unmarshal async event from LLRP. error: %v\n", err)
		return
	}

	if cfg.KeepAliveSpec.Trigger == KATriggerPeriodic {
		dur := time.Duration(cfg.KeepAliveSpec.Interval) * time.Millisecond
		sim.Logger.Printf("Setting keep alive duration to %v\n", dur)
		sim.kaTicker.Reset(dur)
	} else {
		sim.Logger.Println("Disabling keep alive")
		sim.kaTicker.Stop()
	}

	td.write(msg.id, &SetReaderConfigResponse{LLRPStatus: successStatus})
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

	go sim.taskLoop()

	return nil
}

func (sim *Simulator) taskLoop() {
	for {
		select {
		case <-sim.done:
			sim.Logger.Println("Simulator task loop exiting.")
			return
		case <-sim.roTicker.C:
			sim.SendTagData()
		case <-sim.kaTicker.C:
			sim.SendKeepAlive()
		}
	}
}

// Shutdown stops the llrp message handler
// todo: stop management rest server
func (sim *Simulator) Shutdown() error {
	sim.Logger.Println("Shutting down simulator...")
	sim.done <- true
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

func peakRssiPtr(val int) *PeakRSSI {
	rssi := PeakRSSI(val)
	return &rssi
}

func antIdPtr(val int) *AntennaID {
	ant := AntennaID(val)
	return &ant
}

func lastSeenPtr() *LastSeenUTC {
	tm := LastSeenUTC(time.Now().UnixNano() / 1e3)
	return &tm
}

func (sim *Simulator) SendTagData() {
	if !sim.reading {
		return
	}

	epc, _ := hex.DecodeString("3014000000000000000000ff")
	// randomize last byte
	//epc[len(epc)-1] = byte(rand.Intn(math.MaxUint8))
	epc[len(epc)-1] = byte(rand.Intn(5))

	data := &ROAccessReport{TagReportData: []TagReportData{
		{
			EPC96: EPC96{
				EPC: epc,
			},
			AntennaID:   antIdPtr(rand.Intn(4) + 1), // antennas are 1-based
			PeakRSSI:    peakRssiPtr(int(rand.Float64()*float64(rssiFullRange)) + rssiMin),
			LastSeenUTC: lastSeenPtr(),
		},
	}}

	sim.emu.devicesMu.Lock()
	sim.Logger.Printf("sending tag read data: %s\t%v\t%v\t%v\n",
		hex.EncodeToString(epc),
		*data.TagReportData[0].AntennaID, *data.TagReportData[0].PeakRSSI, *data.TagReportData[0].LastSeenUTC)
	for td := range sim.emu.devices {
		td.write(nextMessageId(td), data)
	}
	sim.emu.devicesMu.Unlock()
}

func (sim *Simulator) SendKeepAlive() {
	ka := &KeepAlive{}

	sim.emu.devicesMu.Lock()
	for td := range sim.emu.devices {
		sim.Logger.Printf("sending keep alive")
		td.write(nextMessageId(td), ka)
	}
	sim.emu.devicesMu.Unlock()
}

func nextMessageId(td *TestDevice) messageID {
	return messageID(atomic.AddUint32((*uint32)(&td.mid), 1))
}
