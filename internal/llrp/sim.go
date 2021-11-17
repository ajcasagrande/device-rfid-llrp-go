//
// Copyright (C) 2021 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package llrp

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"
)

const (
	defaultReadRate      = 100
	defaultAntennaCount  = 2
	defaultPort          = 5084
	defaultAPIPort       = 55084
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
	flags  SimulatorConfigFlags
	config SimulatorConfig
	emu    *TestEmulator
	Logger *log.Logger

	kaTicker   *time.Ticker
	roTicker   *time.Ticker
	stopTicker *time.Ticker

	state *simulatorState

	flagUpdateCh chan SimulatorConfigFlags

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

// SimulatorConfigFlags are the command line options
type SimulatorConfigFlags struct {
	Filename      string `json:"filename,omitempty"`
	Silent        bool   `json:"silent,omitempty"`
	LLRPPort      int    `json:"llrp_port,omitempty"`
	APIPort       int    `json:"api_port,omitempty"`
	AntennaCount  int    `json:"antenna_count,omitempty"`
	TagPopulation int    `json:"tag_population,omitempty"`
	BaseEPC       string `json:"base_epc,omitempty"`
	MaxRSSI       int    `json:"max_rssi,omitempty"`
	MinRSSI       int    `json:"min_rssi,omitempty"`
	ReadRate      int    `json:"read_rate,omitempty"`
}

// NewSimulatorConfigFlags creates a new SimulatorConfigFlags struct with the default values loaded
func NewSimulatorConfigFlags() SimulatorConfigFlags {
	return SimulatorConfigFlags{
		Filename:      "",
		Silent:        false,
		LLRPPort:      defaultPort,
		APIPort:       defaultAPIPort,
		AntennaCount:  defaultAntennaCount,
		TagPopulation: defaultTagPopulation,
		BaseEPC:       defaultBaseEPC,
		MaxRSSI:       defaultMaxRSSI,
		MinRSSI:       defaultMinRSSI,
		ReadRate:      defaultReadRate,
	}
}

// SimulatorConfig contains the pre-defined ReaderCapabilities and ReaderConfig structs
type SimulatorConfig struct {
	ReaderCapabilities GetReaderCapabilitiesResponse
	ReaderConfig       GetReaderConfigResponse
}

// NewSimulator returns a new un-initialized Simulator without any configuration. Initialize() must
// be called on the returned Simulator.
func NewSimulator() *Simulator {
	return &Simulator{
		Logger:       log.Default(),
		kaTicker:     newInactiveTicker(),
		roTicker:     newInactiveTicker(),
		stopTicker:   newInactiveTicker(),
		state:        &simulatorState{},
		flagUpdateCh: make(chan SimulatorConfigFlags, 2),
		done:         make(chan struct{}),
	}
}

// Initialize parses config file and initializes the simulator but does not start it.
func (sim *Simulator) Initialize(flags SimulatorConfigFlags) error {
	sim.flags = flags
	if sim.flags.Filename == "" {
		return fmt.Errorf("please specify filename")
	}

	sim.Logger.Printf("Loading simulator config from '%s'", sim.flags.Filename)
	if err := sim.loadConfig(); err != nil {
		return err
	}
	sim.Logger.Println("Successfully loaded simulator config.")

	sim.emu = NewTestEmulator(sim.flags.Silent)
	sim.setupHandlers()

	sim.patchReaderConfig()

	return nil
}

// StartAsync starts processing llrp messages async
// todo: add management rest server
func (sim *Simulator) StartAsync() error {
	sim.Logger.Printf("Starting llrp simulator on port: %d\n", sim.flags.LLRPPort)
	if err := sim.emu.StartAsync(sim.flags.LLRPPort); err != nil {
		return err
	}

	wg := &sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		sim.setupAPIRoutes()
		sim.serveAPI()
	}()

	go func() {
		defer wg.Done()
		sim.taskLoop()
	}()

	// wait for graceful shutdowns
	wg.Wait()
	return nil
}

// taskLoop is the main event handling forever loop of the simulator. It listens to the various
// channels to know when to perform actions such as sending tag data, sending keepalives, etc.
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
		case <-sim.stopTicker.C:
			sim.stopReading()
		case f := <-sim.flagUpdateCh:
			// dynamically update config. because we handle everything in this taskLoop there is
			// no need to lock the config using a mutex
			if f.ReadRate != 0 {
				sim.flags.ReadRate = f.ReadRate
				sim.state.roInterval = time.Second / time.Duration(sim.flags.ReadRate)
				sim.Logger.Printf("setting read interval to %v", sim.state.roInterval)
				if sim.state.reading {
					sim.roTicker.Reset(sim.state.roInterval)
				}
			}
			if f.AntennaCount != 0 {
				sim.flags.AntennaCount = f.AntennaCount
			}
			if f.TagPopulation != 0 {
				sim.flags.TagPopulation = f.TagPopulation
			}
			if f.BaseEPC != "" {
				sim.flags.BaseEPC = f.BaseEPC
			}
			if f.MinRSSI != 0 {
				sim.flags.MinRSSI = f.MinRSSI
			}
			if f.MaxRSSI != 0 {
				sim.flags.MaxRSSI = f.MaxRSSI
			}
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
				// todo: only send if enabled via ImpinjCustom and roSpec tag report properties
				// ImpinjPeakRSSI
				impinjCustom(ParamImpinjPeakRSSI, int16ToBytes(int16(*rssi)*100)),
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
