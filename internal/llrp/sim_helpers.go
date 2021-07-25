//
// Copyright (C) 2021 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package llrp

import (
	"encoding/binary"
	"math/big"
	"math/rand"
	"time"
)

// newInactiveTicker creates a new inactive time.Ticker. To activate with a specific duration, call
// ticker.Reset(duration) on the returned ticker.
func newInactiveTicker() *time.Ticker {
	t := time.NewTicker(time.Hour) // bogus time since we are stopping it right away
	t.Stop() // stop the tickers because they start automatically
	return t
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

func int16ToBytes(i int16) []byte {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, uint16(i))
	return b
}

func impinjCustom(subtype uint32, data []byte) Custom {
	return Custom{
		VendorID: Impinj.Value(),
		Subtype:  subtype,
		Data:     data,
	}
}

// patchReaderConfig modifies the ReaderConfig to match the active properties, such as number
// of antennas connected, etc.
func (sim *Simulator) patchReaderConfig() {
	for i := 0; i < len(sim.config.ReaderConfig.AntennaProperties); i++ {
		sim.config.ReaderConfig.AntennaProperties[i].AntennaConnected = i < sim.flags.AntennaCount
	}
}
