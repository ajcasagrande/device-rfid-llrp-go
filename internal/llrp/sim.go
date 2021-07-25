//
// Copyright (C) 2021 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package llrp

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"log"
	"os"
	"time"
)

const (
	defaultReadRate      = 100
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

// Simulator struct contains the simulator config, state, and connections.
type Simulator struct {
	flags  ConfigFlags
	config SimulatorConfig
	emu    *TestEmulator
	Logger *log.Logger

	kaTicker   *time.Ticker
	roTicker   *time.Ticker
	stopTicker *time.Ticker

	state *simulatorState

	done chan struct{}
}

// simulatorState keeps track of the current state/configuration of the simulator
type simulatorState struct {
	reading    bool
	roInterval time.Duration
	ro         *ROSpec
}

func (s *simulatorState) resetState() {
	s.ro = nil
	s.reading = false
	s.roInterval = time.Second / time.Duration(defaultReadRate)
}

// ConfigFlags are the command line options
type ConfigFlags struct {
	Filename      string
	Silent        bool
	LLRPPort      int
	AntennaCount  int
	TagPopulation int
	BaseEPC       string
	MaxRSSI       int
	MinRSSI       int
	ReadRate      int
}

// SimulatorConfig contains the pre-defined ReaderCapabilities and ReaderConfig structs
type SimulatorConfig struct {
	ReaderCapabilities GetReaderCapabilitiesResponse
	ReaderConfig       GetReaderConfigResponse
}

// parseFlags configures the command line flags and parses them into a ConfigFlags struct.
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

	flag.IntVar(&cf.ReadRate, "r", defaultReadRate, "read rate (tags/s)")
	flag.IntVar(&cf.ReadRate, "rate", defaultReadRate, "read rate (tags/s)")
	flag.IntVar(&cf.ReadRate, "read-rate", defaultReadRate, "read rate (tags/s)")

	flag.Parse()

	if cf.Filename == "" {
		fmt.Printf("Missing required argument -f/--file\n")
		os.Exit(2)
	}

	log.Printf("flags: %+v", cf)

	return cf
}

// CreateSimulator parses config file and sets up a new simulator but does not start it
func CreateSimulator() (*Simulator, error) {
	sim := Simulator{
		flags:      parseFlags(),
		Logger:     log.Default(),
		kaTicker:   newInactiveTicker(),
		roTicker:   newInactiveTicker(),
		stopTicker: newInactiveTicker(),
		state:      &simulatorState{},
		done:       make(chan struct{}),
	}

	sim.kaTicker.Stop()
	sim.roTicker.Stop()
	sim.stopTicker.Stop()

	if sim.flags.Filename == "" {
		return nil, fmt.Errorf("please specify filename")
	}

	sim.Logger.Printf("Loading simulator config from '%s'", sim.flags.Filename)
	if err := sim.loadConfig(); err != nil {
		return nil, err
	}
	sim.Logger.Println("Successfully loaded simulator config.")

	sim.emu = NewTestEmulator(sim.flags.Silent)
	sim.setupHandlers()

	sim.patchReaderConfig()

	return &sim, nil
}

// StartAsync starts processing llrp messages async
// todo: add management rest server
func (sim *Simulator) StartAsync() error {
	sim.Logger.Printf("Starting llrp simulator on port: %d\n", sim.flags.LLRPPort)
	if err := sim.emu.StartAsync(sim.flags.LLRPPort); err != nil {
		return err
	}

	go sim.taskLoop()

	return nil
}

// taskLoop is the main event handling forever loop of the simulator. It listens to the various
// channels to know when to perform actions such as sending tag data, sending keepalives, etc.
func (sim *Simulator) taskLoop() {
	for {
		select {
		case _, err := <-sim.done:
			sim.Logger.Println("Simulator task loop exiting.")
			if err {
				sim.Logger.Printf("Error occurred on done channel: error: %v", err)
			}
			return
		case <-sim.roTicker.C:
			sim.SendTagData()
		case <-sim.kaTicker.C:
			sim.SendKeepAlive()
		case <-sim.stopTicker.C:
			sim.stopReading()
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

// loadConfig reads the json configuration file
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

// SendTagData generates a new ROAccessReport using random data and sends it via llrp
func (sim *Simulator) SendTagData() {
	if !sim.state.reading {
		return
	}

	epc := randomEPC(sim.flags.BaseEPC, sim.flags.TagPopulation)

	rssi := randomRSSI(sim.flags.MinRSSI, sim.flags.MaxRSSI)

	data := &ROAccessReport{TagReportData: []TagReportData{
		{
			EPC96: EPC96{
				EPC: epc.Bytes(),
			},
			AntennaID:   randomAntennaID(sim.flags.AntennaCount),
			PeakRSSI:    rssi,
			LastSeenUTC: lastSeenPtr(time.Now()),
			Custom: []Custom{
				// ImpinjPeakRSSI
				impinjCustom(57, int16ToBytes(int16(*rssi)*100)),
			},
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

// SendKeepAlive sends a KeepAlive message to all connected clients
func (sim *Simulator) SendKeepAlive() {
	ka := &KeepAlive{}

	sim.emu.devicesMu.Lock()
	for td := range sim.emu.devices {
		sim.Logger.Printf("sending keep alive")
		td.write(td.nextMessageId(), ka)
	}
	sim.emu.devicesMu.Unlock()
}
