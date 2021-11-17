//
//  Copyright (C) 2021 Intel Corporation
//
//  SPDX-License-Identifier: Apache-2.0

//go:generate stringer -type=ImpinjCustomMessageType,ImpinjCustomParamType -linecomment -output impinj_string.go

package llrp

type ImpinjCustomMessageType uint32

const (
	MsgImpinjEnableExtensions         = ImpinjCustomMessageType(21) // ImpinjEnableExtensions
	MsgImpinjEnableExtensionsResponse = ImpinjCustomMessageType(22) // ImpinjEnableExtensionsResponse
	MsgImpinjSaveSettings             = ImpinjCustomMessageType(23) // ImpinjSaveSettings
	MsgImpinjSaveSettingsResponse     = ImpinjCustomMessageType(24) // ImpinjSaveSettingsResponse
)

type ImpinjCustomParamType uint32

const (
	ParamImpinjRequestedData                       = ImpinjCustomParamType(21) // ImpinjRequestedData
	ParamImpinjSubRegulatoryRegion                 = ImpinjCustomParamType(22) // ImpinjSubRegulatoryRegion
	ParamImpinjInventorySearchMode                 = ImpinjCustomParamType(23) // ImpinjInventorySearchMode
	ParamImpinjFixedFrequencyList                  = ImpinjCustomParamType(26) // ImpinjFixedFrequencyList
	ParamImpinjReducedPowerFrequencyList           = ImpinjCustomParamType(27) // ImpinjReducedPowerFrequencyList
	ParamImpinjLowDutyCycle                        = ImpinjCustomParamType(28) // ImpinjLowDutyCycle
	ParamImpinjDetailedVersion                     = ImpinjCustomParamType(29) // ImpinjDetailedVersion
	ParamImpinjFrequencyCapabilities               = ImpinjCustomParamType(30) // ImpinjFrequencyCapabilities
	ParamImpinjGPIDebounceConfiguration            = ImpinjCustomParamType(36) // ImpinjGPIDebounceConfiguration
	ParamImpinjReaderTemperature                   = ImpinjCustomParamType(37) // ImpinjReaderTemperature
	ParamImpinjLinkMonitorConfiguration            = ImpinjCustomParamType(38) // ImpinjLinkMonitorConfiguration
	ParamImpinjReportBufferConfiguration           = ImpinjCustomParamType(39) // ImpinjReportBufferConfiguration
	ParamImpinjAccessSpecConfiguration             = ImpinjCustomParamType(40) // ImpinjAccessSpecConfiguration
	ParamImpinjBlockWriteWordCount                 = ImpinjCustomParamType(41) // ImpinjBlockWriteWordCount
	ParamImpinjBlockPermalock                      = ImpinjCustomParamType(42) // ImpinjBlockPermalock
	ParamImpinjBlockPermalockOpSpecResult          = ImpinjCustomParamType(43) // ImpinjBlockPermalockOpSpecResult
	ParamImpinjGetBlockPermalockStatus             = ImpinjCustomParamType(44) // ImpinjGetBlockPermalockStatus
	ParamImpinjGetBlockPermalockStatusOpSpecResult = ImpinjCustomParamType(45) // ImpinjGetBlockPermalockStatusOpSpecResult
	ParamImpinjSetQTConfig                         = ImpinjCustomParamType(46) // ImpinjSetQTConfig
	ParamImpinjSetQTConfigOpSpecResult             = ImpinjCustomParamType(47) // ImpinjSetQTConfigOpSpecResult
	ParamImpinjGetQTConfig                         = ImpinjCustomParamType(48) // ImpinjGetQTConfig
	ParamImpinjGetQTConfigOpSpecResult             = ImpinjCustomParamType(49) // ImpinjGetQTConfigOpSpecResult
	ParamImpinjTagReportContentSelector            = ImpinjCustomParamType(50) // ImpinjTagReportContentSelector
	ParamImpinjEnableSerializedTID                 = ImpinjCustomParamType(51) // ImpinjEnableSerializedTID
	ParamImpinjEnableRFPhaseAngle                  = ImpinjCustomParamType(52) // ImpinjEnableRFPhaseAngle
	ParamImpinjEnablePeakRSSI                      = ImpinjCustomParamType(53) // ImpinjEnablePeakRSSI
	ParamImpinjEnableGPSCoordinates                = ImpinjCustomParamType(54) // ImpinjEnableGPSCoordinates
	ParamImpinjSerializedTID                       = ImpinjCustomParamType(55) // ImpinjSerializedTID
	ParamImpinjRFPhaseAngle                        = ImpinjCustomParamType(56) // ImpinjRFPhaseAngle
	ParamImpinjPeakRSSI                            = ImpinjCustomParamType(57) // ImpinjPeakRSSI
	ParamImpinjGPSCoordinates                      = ImpinjCustomParamType(58) // ImpinjGPSCoordinates
	ParamImpinjLoopSpec                            = ImpinjCustomParamType(59) // ImpinjLoopSpec
	ParamImpinjGPSNMEASentences                    = ImpinjCustomParamType(60) // ImpinjGPSNMEASentences
	ParamImpinjGGASentence                         = ImpinjCustomParamType(61) // ImpinjGGASentence
	ParamImpinjRMCSentence                         = ImpinjCustomParamType(62) // ImpinjRMCSentence
	ParamImpinjOpSpecRetryCount                    = ImpinjCustomParamType(63) // ImpinjOpSpecRetryCount
	ParamImpinjAdvancedGPOConfiguration            = ImpinjCustomParamType(64) // ImpinjAdvancedGPOConfiguration
	ParamImpinjEnableOptimizedRead                 = ImpinjCustomParamType(65) // ImpinjEnableOptimizedRead
	ParamImpinjAccessSpecOrdering                  = ImpinjCustomParamType(66) // ImpinjAccessSpecOrdering
	ParamImpinjEnableRFDopplerFrequency            = ImpinjCustomParamType(67) // ImpinjEnableRFDopplerFrequency
	ParamImpinjRFDopplerFrequency                  = ImpinjCustomParamType(68) // ImpinjRFDopplerFrequency
	ParamImpinjInventoryConfiguration              = ImpinjCustomParamType(69) // ImpinjInventoryConfiguration
	ParamImpinjEnableTxPower                       = ImpinjCustomParamType(72) // ImpinjEnableTxPower
	ParamImpinjTxPower                             = ImpinjCustomParamType(73) // ImpinjTxPower
)
