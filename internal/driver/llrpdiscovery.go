// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2022 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/edgexfoundry/device-onvif-camera/pkg/netscan"
	"github.com/edgexfoundry/device-rfid-llrp-go/internal/llrp"
	dsModels "github.com/edgexfoundry/device-sdk-go/v2/pkg/models"
	contract "github.com/edgexfoundry/go-mod-core-contracts/v2/models"
	"github.com/pkg/errors"
	"net"
	"strconv"
	"strings"
)

const (
	defaultDevicePrefix    = "LLRP"
	unknownVendorID        = 0
	unknownModelID         = 0
	unknownFirmwareVersion = ""
)

// discoveryInfo holds information about a discovered device
type discoveryInfo struct {
	deviceName string
	host       string
	port       string
	vendor     uint32
	model      uint32
	fwVersion  string
}

// newDiscoveryInfo creates a new discoveryInfo with just a host and port pre-filled
func newDiscoveryInfo(host, port string) *discoveryInfo {
	return &discoveryInfo{
		host: host,
		port: port,
	}
}

// LLRPProtocolDiscovery implements netscan.ProtocolSpecificDiscovery
type LLRPProtocolDiscovery struct {
	driver    *Driver
	deviceMap map[string]contract.Device
}

func NewLLRPProtocolDiscovery(driver *Driver) *LLRPProtocolDiscovery {
	return &LLRPProtocolDiscovery{driver: driver}
}

// ProbeFilter takes in a host and a slice of ports to be scanned. It should return a slice
// of ports to actually scan, or a nil/empty slice if the host is to not be scanned at all.
// Can be used to filter out known devices from being probed again if required.
func (proto *LLRPProtocolDiscovery) ProbeFilter(ipStr string, ports []string) []string {
	// For llrp we do not want to probe any connected devices
	var results []string
	for _, port := range ports {
		addr := ipStr + ":" + port
		if d, found := proto.deviceMap[addr]; found {
			if d.OperatingState == contract.Up {
				proto.driver.lc.Debug("Skip scan of " + addr + ", device already registered.")
				continue
			}
			proto.driver.lc.Info("Existing device in disabled (disconnected) state will be scanned again.",
				"address", addr,
				"deviceName", d.Name)
		}

		// add the port to be scanned
		results = append(results, port)
	}
	return results
}

// OnConnectionDialed handles the protocol specific verification if there is actually
// a valid device or devices at the other end of the connection.
func (proto *LLRPProtocolDiscovery) OnConnectionDialed(host string, port string, conn net.Conn, params netscan.Params) ([]netscan.ProbeResult, error) {
	c := llrp.NewClient(llrp.WithLogger(&edgexLLRPClientLogger{
		devName: "probe-" + host,
		lc:      proto.driver.lc,
	}))

	readerConfig := llrp.GetReaderConfigResponse{}
	readerCaps := llrp.GetReaderCapabilitiesResponse{}

	// send llrp messages in a separate thread and block the main thread until it is complete
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
		defer cancel()

		defer func() {
			if err := c.Shutdown(ctx); err != nil && !errors.Is(err, llrp.ErrClientClosed) {
				params.Logger.Warnf("Error closing discovery device: %s", err.Error())
				_ = c.Close()
			}
		}()

		params.Logger.Debugf("Sending GetReaderConfig.")
		configReq := llrp.GetReaderConfig{
			RequestedData: llrp.ReaderConfReqIdentification,
		}
		err := c.SendFor(ctx, &configReq, &readerConfig)
		if errors.Is(err, llrp.ErrClientClosed) {
			params.Logger.Warnf("Client connection was closed while sending GetReaderConfig to discovered device: %s", err.Error())
			return
		} else if err != nil {
			params.Logger.Warnf("Error sending GetReaderConfig to discovered device: %s", err.Error())
			return
		}

		params.Logger.Debugf("Sending GetReaderCapabilities.")
		capabilitiesReq := llrp.GetReaderCapabilities{
			ReaderCapabilitiesRequestedData: llrp.ReaderCapGeneralDeviceCapabilities,
		}
		err = c.SendFor(ctx, &capabilitiesReq, &readerCaps)
		if errors.Is(err, llrp.ErrClientClosed) {
			params.Logger.Warnf("Client connection was closed while sending GetReaderCapabilities to discovered device: %s", err.Error())
			return
		} else if err != nil {
			params.Logger.Warnf("Error sending GetReaderCapabilities to discovered device: %s", err.Error())
			return
		}
	}()

	params.Logger.Infof("Attempting to connect to potential LLRP device... host=%s port=%s", host, port)
	// this will block until `c.Shutdown()` is called in the above go routine
	if err := c.Connect(conn); !errors.Is(err, llrp.ErrClientClosed) {
		params.Logger.Warnf("Error attempting to connect to potential LLRP device: " + err.Error())
		return nil, err
	}
	params.Logger.Infof("Connection initiated successfully. host=%s port=%s", host, port)

	info := newDiscoveryInfo(host, port)
	if readerCaps.GeneralDeviceCapabilities == nil {
		params.Logger.Warnf("ReaderCapabilities.GeneralDeviceCapabilities was nil, unable to determine vendor and model info")
		info.vendor = unknownVendorID
		info.model = unknownModelID
		info.fwVersion = unknownFirmwareVersion
	} else {
		info.vendor = readerCaps.GeneralDeviceCapabilities.DeviceManufacturer
		info.model = readerCaps.GeneralDeviceCapabilities.Model
		info.fwVersion = readerCaps.GeneralDeviceCapabilities.FirmwareVersion
	}

	if readerConfig.Identification == nil {
		params.Logger.Warnf("ReaderConfig.Identification was nil, unable to register discovered device.")
		return nil, fmt.Errorf("unable to retrieve device identification")
	}

	prefix := defaultDevicePrefix
	if VendorIDType(info.vendor) == Impinj {
		prefix = ImpinjModelType(info.model).HostnamePrefix()
	}

	var suffix string
	rID := readerConfig.Identification.ReaderID
	if readerConfig.Identification.IDType == llrp.ID_MAC_EUI64 && len(rID) >= 3 {
		mac := rID[len(rID)-3:]
		suffix = fmt.Sprintf("%02X-%02X-%02X", mac[0], mac[1], mac[2])
	} else {
		suffix = hex.EncodeToString(rID)
	}

	info.deviceName = prefix + "-" + suffix
	params.Logger.Infof("Discovered device: %+v", info)

	// store result in a generic netscan.ProbeResult
	return []netscan.ProbeResult{{
		Host: host,
		Port: port,
		Data: info,
	}}, nil
}

// ConvertProbeResult takes a raw ProbeResult and transforms it into a
// processed DiscoveredDevice struct.
func (proto *LLRPProtocolDiscovery) ConvertProbeResult(probeResult netscan.ProbeResult, params netscan.Params) (netscan.DiscoveredDevice, error) {
	info, ok := probeResult.Data.(*discoveryInfo)
	if !ok {
		return netscan.DiscoveredDevice{}, fmt.Errorf("unable to cast probe result into *discoveryInfo. type=%T", probeResult.Data)
	}

	// check if any devices already exist at that address, and if so disable them
	existing, found := proto.deviceMap[info.host+":"+info.port]
	if found && existing.Name != info.deviceName {
		// disable it and remove its protocol information since it is no longer valid
		delete(existing.Protocols, "tcp")
		existing.OperatingState = contract.Down
		if err := proto.driver.svc.UpdateDevice(existing); err != nil {
			proto.driver.lc.Warn("There was an issue trying to disable an existing device.",
				"deviceName", existing.Name,
				"error", err)
		}
	}

	// check if we have an existing device registered with this name
	device, err := proto.driver.svc.GetDeviceByName(info.deviceName)
	if err != nil {
		// no existing device; add it to the list and move on
		discovered := newDiscoveredDevice(info)
		return netscan.DiscoveredDevice{
			Name: discovered.Name,
			Info: discovered,
		}, nil
	}

	// this means we have discovered an existing device that is
	// either disabled or has changed IP addresses.
	// we need to update its protocol information and operating state
	if err := info.updateExistingDevice(device); err != nil {
		driver.lc.Warn("There was an issue trying to update an existing device based on newly discovered details.",
			"deviceName", device.Name,
			"discoveryInfo", fmt.Sprintf("%+v", info),
			"error", err)
	}

	return netscan.DiscoveredDevice{}, nil
}

// newDiscoveredDevice takes the host and port number of a discovered LLRP reader and prepares it for
// registration with EdgeX
func newDiscoveredDevice(info *discoveryInfo) dsModels.DiscoveredDevice {
	labels := []string{
		"RFID",
		"LLRP",
	}

	// Add vendor string only if it is a known vendor
	vendorStr := VendorIDType(info.vendor).String()
	if !strings.HasPrefix(vendorStr, "VendorIDType(") {
		labels = append(labels, vendorStr)
	} else {
		vendorStr = ""
	}

	// Add model string if it is available. Currently, only supported for a select few Impinj devices.
	var modelStr string
	if VendorIDType(info.vendor) == Impinj {
		ipjModelStr := ImpinjModelType(info.model).String()
		if !strings.HasPrefix(modelStr, "ImpinjModelType(") {
			modelStr = ipjModelStr
			labels = append(labels, modelStr)

		}
	}

	// Note that we are adding vendorPEN to the protocol properties in order to
	// allow the provision watchers to be able to match against that info. Currently,
	// that is the only thing the provision watchers use for matching against.
	return dsModels.DiscoveredDevice{
		Name: info.deviceName,
		Protocols: map[string]contract.ProtocolProperties{
			"tcp": {
				"host": info.host,
				"port": info.port,
			},
			// we are adding a "metadata" protocol as a place to store general properties for
			// matching against provision watchers. It is not an actual protocol, but
			// the recommended way of adding generic key/value pairs to a device.
			"metadata": {
				"vendorPEN": strconv.FormatUint(uint64(info.vendor), 10),
				"vendorStr": vendorStr,
				"model":     strconv.FormatUint(uint64(info.model), 10),
				"modelStr":  modelStr,
				"fwVersion": info.fwVersion,
			},
		},
		Description: "LLRP RFID Reader",
		Labels:      labels,
	}
}

// updateExistingDevice is used when an existing device is discovered
// and needs to update its information to either a new address or set
// its operating state to enabled.
func (info *discoveryInfo) updateExistingDevice(device contract.Device) error {
	shouldUpdate := false
	if device.OperatingState == contract.Down {
		device.OperatingState = contract.Up
		shouldUpdate = true
	}

	tcpInfo := device.Protocols["tcp"]
	if tcpInfo == nil ||
		info.host != tcpInfo["host"] ||
		info.port != tcpInfo["port"] {
		driver.lc.Info("Existing device has been discovered with a different network address.",
			"oldInfo", fmt.Sprintf("%+v", tcpInfo),
			"discoveredInfo", fmt.Sprintf("%+v", info))

		device.Protocols["tcp"] = map[string]string{
			"host": info.host,
			"port": info.port,
		}
		// make sure it is enabled
		device.OperatingState = contract.Up
		shouldUpdate = true
	}

	if !shouldUpdate {
		// the address is the same and device is already enabled, should not reach here
		driver.lc.Warn("Re-discovered existing device at the same TCP address, nothing to do.")
		return nil
	}

	if err := driver.svc.UpdateDevice(device); err != nil {
		driver.lc.Error("There was an error updating the tcp address for an existing device.",
			"deviceName", device.Name,
			"error", err)
		return err
	}

	return nil
}
