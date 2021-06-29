//
// Copyright (C) 2021 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package llrp

import (
	"encoding/json"
	"flag"
	"github.com/pkg/errors"
	"io/ioutil"
	"log"
	"math/big"
	"math/rand"
	"os"
	"time"
)

const (
	defaultReadInterval  = 50 * time.Millisecond
	defaultAntennaCount  = 2
	defaultPort          = 5084
	defaultTagPopulation = 20
	defaultMinRSSI       = -80 // -95
	defaultMaxRSSI       = -60 // -55
	defaultBaseEPC       = "301400000000000000000000"
)

var (
	successStatus = LLRPStatus{
		Status: StatusSuccess,
	}
)

type Simulator struct {
	flags  ConfigFlags
	config SimulatorConfig
	emu    *TestEmulator
	Logger *log.Logger

	reading bool

	kaTicker *time.Ticker
	roTicker *time.Ticker

	done chan struct{}
}

type ConfigFlags struct {
	Filename      string
	Silent        bool
	LLRPPort      int
	AntennaCount  int
	TagPopulation int
	BaseEPC       string
	MaxRSSI       int
	MinRSSI       int
}

type SimulatorConfig struct {
	ReaderCapabilities GetReaderCapabilitiesResponse
	ReaderConfig       GetReaderConfigResponse
}

func parseFlags() ConfigFlags {
	var cf ConfigFlags

	flag.BoolVar(&cf.Silent, "s", false, "silent")
	flag.BoolVar(&cf.Silent, "silent", false, "silent")

	flag.IntVar(&cf.LLRPPort, "p", defaultPort, "llrp port")
	flag.IntVar(&cf.LLRPPort, "port", defaultPort, "llrp port")

	flag.IntVar(&cf.AntennaCount, "a", defaultAntennaCount, "antenna count")
	flag.IntVar(&cf.AntennaCount, "antenna-count", defaultAntennaCount, "antenna count")

	flag.IntVar(&cf.TagPopulation, "t", defaultTagPopulation, "tag population count")
	flag.IntVar(&cf.TagPopulation, "tags", defaultTagPopulation, "tag population count")
	flag.IntVar(&cf.TagPopulation, "tag-population", defaultTagPopulation, "tag population count")

	flag.StringVar(&cf.BaseEPC, "e", defaultBaseEPC, "Base EPC")
	flag.StringVar(&cf.BaseEPC, "epc", defaultBaseEPC, "Base EPC")
	flag.StringVar(&cf.BaseEPC, "base-epc", defaultBaseEPC, "Base EPC")

	flag.IntVar(&cf.MaxRSSI, "M", defaultMaxRSSI, "max rssi")
	flag.IntVar(&cf.MaxRSSI, "max", defaultMaxRSSI, "max rssi")
	flag.IntVar(&cf.MaxRSSI, "max-rssi", defaultMaxRSSI, "max rssi")

	flag.IntVar(&cf.MinRSSI, "m", defaultMinRSSI, "min rssi")
	flag.IntVar(&cf.MinRSSI, "min", defaultMinRSSI, "min rssi")
	flag.IntVar(&cf.MinRSSI, "min-rssi", defaultMinRSSI, "min rssi")

	flag.StringVar(&cf.Filename, "f", "", "config filename")
	flag.StringVar(&cf.Filename, "file", "", "config filename")

	flag.Parse()

	log.Printf("flags: %+v", cf)

	return cf
}

// CreateSimulator parses config file and sets up a new simulator but does not start it
func CreateSimulator() (*Simulator, error) {
	sim := Simulator{
		flags:    parseFlags(),
		Logger:   log.Default(),
		kaTicker: time.NewTicker(1 * time.Hour),
		roTicker: time.NewTicker(1 * time.Hour),
		done:     make(chan struct{}),
	}
	// stop the tickers because they start automatically
	sim.kaTicker.Stop()
	sim.roTicker.Stop()

	sim.Logger.Printf("Loading simulator config from '%s'", sim.flags.Filename)
	if err := sim.loadConfig(); err != nil {
		return nil, err
	}
	sim.Logger.Println("Successfully loaded simulator config.")

	sim.emu = NewTestEmulator(sim.flags.Silent)
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
	sim.Logger.Printf("Starting simulator on llrp port: %d\n", sim.flags.LLRPPort)
	if err := sim.emu.StartAsync(sim.flags.LLRPPort); err != nil {
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
	close(sim.done)
	return sim.emu.Shutdown()
}

func (sim *Simulator) loadConfig() error {
	f, err := os.Open(sim.flags.Filename)
	if err != nil {
		return errors.Wrapf(err, "error opening simulator config file '%s'", sim.flags.Filename)
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return errors.Wrapf(err, "error reading simulator config file '%s'", sim.flags.Filename)
	}

	err = json.Unmarshal(data, &sim.config)
	if err != nil {
		return errors.Wrap(err, "error unmarshalling simulator config from json")
	}

	return nil
}

// randomRSSI generates a pseudo-random *PeakRSSI integer between [min, max]
func randomRSSI(min, max int) *PeakRSSI {
	rssi := PeakRSSI(int(rand.Float64()*float64(max-min)) + min)
	return &rssi
}

// randomAntennaID generates a pseudo-random *AntennaID uint between [1, antennaCount+1]
func randomAntennaID(antennaCount int) *AntennaID {
	ant := AntennaID(rand.Intn(antennaCount) + 1) // antennas are 1-based
	return &ant
}

// lastSeenPtr converts a time.Time into a *LastSeenUTC (microseconds since epoch)
func lastSeenPtr(tm time.Time) *LastSeenUTC {
	tm2 := LastSeenUTC(tm.UnixNano() / 1e3)
	return &tm2
}

// randomEPC generates a new pseudo-random EPC between [baseEPC, baseEPC+tagPopulation)
func randomEPC(baseEPC string, tagPopulation int) *big.Int {
	epc := new(big.Int)
	// parse baseEPC into a big Int
	epc.SetString(baseEPC, 16)
	// add a pseudo-random offset between [0, tagPopulation)
	epc.Add(epc, big.NewInt(int64(rand.Intn(tagPopulation))))
	return epc
}

// SendTagData generates a new ROAccessReport using random data and sends it via llrp
func (sim *Simulator) SendTagData() {
	if !sim.reading {
		return
	}

	epc := randomEPC(sim.flags.BaseEPC, sim.flags.TagPopulation)

	data := &ROAccessReport{TagReportData: []TagReportData{
		{
			EPC96: EPC96{
				EPC: epc.Bytes(),
			},
			AntennaID:   randomAntennaID(sim.flags.AntennaCount),
			PeakRSSI:    randomRSSI(sim.flags.MinRSSI, sim.flags.MaxRSSI),
			LastSeenUTC: lastSeenPtr(time.Now()),
		},
	}}

	sim.emu.devicesMu.Lock()
	sim.Logger.Printf("sending tag read data: %s\t%v\t%v\t%v\n",
		epc.Text(16), *data.TagReportData[0].AntennaID, *data.TagReportData[0].PeakRSSI, *data.TagReportData[0].LastSeenUTC)
	// when single-connection mode is enabled, this should only ever be one device
	for td := range sim.emu.devices {
		td.write(td.nextMessageId(), data)
	}
	sim.emu.devicesMu.Unlock()
}

func (sim *Simulator) SendKeepAlive() {
	ka := &KeepAlive{}

	sim.emu.devicesMu.Lock()
	for td := range sim.emu.devices {
		sim.Logger.Printf("sending keep alive")
		td.write(td.nextMessageId(), ka)
	}
	sim.emu.devicesMu.Unlock()
}
