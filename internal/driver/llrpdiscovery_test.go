//
// Copyright (C) 2020 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"context"
	"github.com/edgexfoundry/device-onvif-camera/pkg/netscan"
	dsModels "github.com/edgexfoundry/device-sdk-go/v2/pkg/models"
	"github.com/stretchr/testify/require"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/edgexfoundry/device-rfid-llrp-go/internal/llrp"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/clients/logger"
)

var (
	svc = NewMockSdkService()
)

func TestMain(m *testing.M) {
	_ = Instance()

	driver.svc = svc
	driver.lc = logger.NewClient("test", "DEBUG")

	os.Exit(m.Run())
}

func makeParams() netscan.Params {
	return netscan.Params{
		Subnets:         []string{"127.0.0.1/32"},
		ScanPorts:       []string{"59923"},
		AsyncLimit:      1000,
		NetworkProtocol: netscan.NetworkTCP,
		Timeout:         1 * time.Second,
		Logger:          driver.lc,
	}
}

// todo: add unit tests for re-discovering disabled devices, and device ip changes

// TestAutoDiscoverNaming tests the naming of discovered devices using various
// different known and unknown vendors and models. It also tests the difference between
// mac based and epc based identification.
func TestAutoDiscoverNaming(t *testing.T) {
	params := makeParams()

	tests := []struct {
		name        string
		description string
		identity    llrp.Identification
		caps        llrp.GeneralDeviceCapabilities
	}{
		{
			name:        "SpeedwayR-19-C5-D6",
			description: "Test standard Speedway R420",
			identity: llrp.Identification{
				IDType:   llrp.ID_MAC_EUI64,
				ReaderID: []byte{0x00, 0x00, 0x00, 0x00, 0x19, 0xC5, 0xD6},
			},
			caps: llrp.GeneralDeviceCapabilities{
				DeviceManufacturer: uint32(Impinj),
				Model:              uint32(SpeedwayR420),
				FirmwareVersion:    "5.14.0.240",
			},
		},
		{
			name:        "xArray-25-9C-D4",
			description: "Test standard xArray",
			identity: llrp.Identification{
				IDType:   llrp.ID_MAC_EUI64,
				ReaderID: []byte{0x00, 0x00, 0x00, 0x00, 0x25, 0x9C, 0xD4},
			},
			caps: llrp.GeneralDeviceCapabilities{
				DeviceManufacturer: uint32(Impinj),
				Model:              uint32(XArray),
				FirmwareVersion:    "5.14.0.240",
			},
		},
		{
			name:        "LLRP-D2-7F-A1",
			description: "Test unknown Impinj model with MAC_EUI64 ID type",
			identity: llrp.Identification{
				IDType:   llrp.ID_MAC_EUI64,
				ReaderID: []byte{0x00, 0x00, 0x00, 0x00, 0xD2, 0x7F, 0xA1},
			},
			caps: llrp.GeneralDeviceCapabilities{
				DeviceManufacturer: uint32(Impinj),
				Model:              uint32(0x32), // unknown model
				FirmwareVersion:    "7.0.0",
			},
		},
		{
			name:        "LLRP-302411f9c92d4f",
			description: "Test unknown Impinj model with EPC ID type",
			identity: llrp.Identification{
				IDType:   llrp.ID_EPC,
				ReaderID: []byte{0x30, 0x24, 0x11, 0xF9, 0xC9, 0x2D, 0x4F},
			},
			caps: llrp.GeneralDeviceCapabilities{
				DeviceManufacturer: uint32(Impinj),
				Model:              uint32(0x32), // unknown model
				FirmwareVersion:    "7.0.0",
			},
		},
		{
			name:        "LLRP-FC-4D-1A",
			description: "Test unknown vendor and unknown model with MAC_EUI64 ID type",
			identity: llrp.Identification{
				IDType:   llrp.ID_MAC_EUI64,
				ReaderID: []byte{0x00, 0x00, 0x00, 0x00, 0xFC, 0x4D, 0x1A},
			},
			caps: llrp.GeneralDeviceCapabilities{
				DeviceManufacturer: uint32(0x32), // unknown vendor
				Model:              uint32(0x32), // unknown model
				FirmwareVersion:    "1.0.0",
			},
		},
		{
			name:        "LLRP-001a004fd9ca2b",
			description: "Test unknown vendor and unknown model with EPC ID type",
			identity: llrp.Identification{
				IDType:   llrp.ID_EPC,                                      // test non-mac id types
				ReaderID: []byte{0x00, 0x1A, 0x00, 0x4F, 0xD9, 0xCA, 0x2B}, // will be used as-is, not parsed
			},
			caps: llrp.GeneralDeviceCapabilities{
				DeviceManufacturer: uint32(0x32), // unknown vendor
				Model:              uint32(0x32), // unknown model
				FirmwareVersion:    "1.0.0",
			},
		},
	}

	// create a fake rx sensitivity table to produce a more valid GetReaderCapabilitiesResponse
	var i uint16
	sensitivities := make([]llrp.ReceiveSensitivityTableEntry, 11)
	for i = 1; i <= 11; i++ {
		sensitivities = append(sensitivities, llrp.ReceiveSensitivityTableEntry{
			Index:              i,
			ReceiveSensitivity: 10 + (4 * (i - 1)), // min 10, max 50
		})
	}

	scanPort, err := strconv.Atoi(params.ScanPorts[0])
	if err != nil {
		t.Fatalf("Failed to parse driver.config.ScanPort, unable to run discovery tests. value = %v", params.ScanPorts[0])
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			emu := llrp.NewTestEmulator(!testing.Verbose())
			if err := emu.StartAsync(scanPort); err != nil {
				t.Fatalf("unable to start emulator: %+v", err)
			}
			defer func() {
				if err := emu.Shutdown(); err != nil {
					t.Errorf("error shutting down test emulator: %s", err.Error())
				}
			}()

			readerConfig := llrp.GetReaderConfigResponse{
				Identification: &test.identity,
			}
			emu.SetResponse(llrp.MsgGetReaderConfig, &readerConfig)

			test.caps.ReceiveSensitivities = sensitivities
			readerCaps := llrp.GetReaderCapabilitiesResponse{
				GeneralDeviceCapabilities: &test.caps,
			}
			emu.SetResponse(llrp.MsgGetReaderCapabilities, &readerCaps)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			discovered := netscan.AutoDiscover(ctx, NewLLRPProtocolDiscovery(driver), params)
			if len(discovered) != 1 {
				t.Fatalf("expected 1 discovered device, however got: %d", len(discovered))
			}

			if discovered[0].Name != test.name {
				t.Errorf("expected discovered device's name to be %s, but was: %s", test.name, discovered[0].Name)
			}

			info, ok := discovered[0].Info.(dsModels.DiscoveredDevice)
			require.True(t, ok)

			pen, ok := info.Protocols["metadata"]["vendorPEN"]
			if !ok {
				t.Fatalf("Missing vendorPEN field in metadata protocol. ProtocolMap: %+v", info.Protocols)
			}
			if pen != strconv.FormatUint(uint64(test.caps.DeviceManufacturer), 10) {
				t.Errorf("expected vendorPEN to be %v, but was: %s", test.caps.DeviceManufacturer, pen)
			}

			fw, ok := info.Protocols["metadata"]["fwVersion"]
			if !ok {
				t.Fatalf("Missing fwVersion field in metadata protocol. ProtocolMap: %+v", info.Protocols)
			}
			if fw != test.caps.FirmwareVersion {
				t.Errorf("expected fwVersion to be %v, but was: %s", test.caps.FirmwareVersion, fw)
			}
		})
	}
}

// TestAutoDiscover uses an emulator to fake discovery process on localhost.
// It checks:
// 1. That no devices are discovered when the emulator is not running
// 2. It discovers the emulator ok when it is running
// 3. It does not re-discover the emulator when it is already registered with EdgeX
func TestAutoDiscover(t *testing.T) {
	params := makeParams()

	// attempt to discover without emulator, expect none found
	svc.clearDevices()
	discovered := netscan.AutoDiscover(context.Background(), NewLLRPProtocolDiscovery(driver), params)
	if len(discovered) != 0 {
		t.Fatalf("expected 0 discovered devices, however got: %d", len(discovered))
	}

	// attempt to discover WITH emulator, expect emulator to be found
	port, err := strconv.Atoi(params.ScanPorts[0])
	if err != nil {
		t.Fatalf("Failed to parse driver.config.ScanPort, unable to run discovery tests. value = %v", params.ScanPorts[0])
	}
	emu := llrp.NewTestEmulator(!testing.Verbose())
	if err := emu.StartAsync(port); err != nil {
		t.Fatalf("unable to start emulator: %+v", err)
	}

	readerConfig := llrp.GetReaderConfigResponse{
		Identification: &llrp.Identification{
			IDType:   llrp.ID_MAC_EUI64,
			ReaderID: []byte{0x00, 0x00, 0x00, 0x00, 0x19, 0xC5, 0xD6},
		},
	}
	emu.SetResponse(llrp.MsgGetReaderConfig, &readerConfig)

	readerCaps := llrp.GetReaderCapabilitiesResponse{
		GeneralDeviceCapabilities: &llrp.GeneralDeviceCapabilities{
			MaxSupportedAntennas:    4,
			CanSetAntennaProperties: false,
			HasUTCClock:             true,
			DeviceManufacturer:      uint32(Impinj),
			Model:                   uint32(SpeedwayR420),
			FirmwareVersion:         "5.14.0.240",
		},
	}
	emu.SetResponse(llrp.MsgGetReaderCapabilities, &readerCaps)

	discovered = netscan.AutoDiscover(context.Background(), NewLLRPProtocolDiscovery(driver), params)
	if len(discovered) != 1 {
		t.Fatalf("expected 1 discovered device, however got: %d", len(discovered))
	}

	var discoveredDevices []dsModels.DiscoveredDevice
	for _, d := range discovered {
		dev, ok := d.Info.(dsModels.DiscoveredDevice)
		require.True(t, ok)
		discoveredDevices = append(discoveredDevices, dev)
	}
	svc.AddDiscoveredDevices(discoveredDevices)

	// attempt to discover again WITH emulator, however expect emulator to be skipped
	svc.resetAddedCount()
	discovered = netscan.AutoDiscover(context.Background(), NewLLRPProtocolDiscovery(driver), params)
	if len(discovered) != 0 {
		t.Fatalf("expected no devices to be discovered, but was %d", len(discovered))
	}

	if err := emu.Shutdown(); err != nil {
		t.Errorf("error shutting down test emulator: %+v", err)
	}
	// reset
	svc.clearDevices()
}
