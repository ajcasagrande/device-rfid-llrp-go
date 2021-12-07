//
// Copyright (C) 2021 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package llrp

import (
	"fmt"
	"time"
)

// setupHandlers registers custom handlers for all of the supported message types by the simulator
func (sim *Simulator) setupHandlers() {
	sim.emu.SetHandler(MsgGetReaderConfig, HandlerCallbackFunc(sim.handleGetReaderConfig))
	sim.emu.SetHandler(MsgSetReaderConfig, HandlerCallbackFunc(sim.handleSetReaderConfig))

	sim.emu.SetHandler(MsgGetReaderCapabilities, HandlerCallbackFunc(sim.handleGetReaderCapabilities))

	sim.emu.SetHandler(MsgAddROSpec, HandlerCallbackFunc(sim.handleAddROSpec))
	sim.emu.SetHandler(MsgDeleteROSpec, HandlerCallbackFunc(sim.handleDeleteROSpec))

	sim.emu.SetHandler(MsgAddAccessSpec, HandlerCallbackFunc(sim.handleAddAccessSpec))
	sim.emu.SetHandler(MsgDeleteAccessSpec, HandlerCallbackFunc(sim.handleDeleteAccessSpec))

	sim.emu.SetHandler(MsgEnableAccessSpec, HandlerCallbackFunc(sim.handleEnableAccessSpec))
	sim.emu.SetHandler(MsgDisableAccessSpec, HandlerCallbackFunc(sim.handleDisableAccessSpec))

	sim.emu.SetHandler(MsgEnableROSpec, HandlerCallbackFunc(sim.handleEnableROSpec))
	sim.emu.SetHandler(MsgDisableROSpec, HandlerCallbackFunc(sim.handleDisableROSpec))

	sim.emu.SetHandler(MsgStartROSpec, HandlerCallbackFunc(sim.handleStartROSpec))
	sim.emu.SetHandler(MsgStopROSpec, HandlerCallbackFunc(sim.handleStopROSpec))

	sim.emu.SetHandler(MsgCustomMessage, HandlerCallbackFunc(sim.handleCustomMessage))
	sim.emu.SetHandler(MsgKeepAliveAck, HandlerCallbackFunc(sim.handleKeepAliveAck))
}

// handleKeepAliveAck is a custom handler for the KeepAliveAck message type
func (sim *Simulator) handleKeepAliveAck(_ *TestDevice, msg Message) {
	sim.Logger.Printf("Received KeepAliveAck messageID: %v\n", msg.id)
	// No response expected
}

// handleCustomMessage is a custom handler for the CustomMessage message type
func (sim *Simulator) handleCustomMessage(td *TestDevice, msg Message) {
	custom := &CustomMessage{}
	if err := msg.UnmarshalTo(custom); err != nil {
		sim.Logger.Printf("Failed to unmarshal async event from LLRP. error: %v\n", err)
		return
	}

	if custom.VendorID == uint32(Impinj) {
		sim.Logger.Printf("Received Custom Message: %s",
			ImpinjCustomMessageType(custom.MessageSubtype).String())
	}

	resp := CustomMessageResponse{
		VendorID: custom.VendorID,
		// todo: hack: we are assuming that the response to the custom message is always
		//			   a value 1 higher than the request message's subtype.
		MessageSubtype: custom.MessageSubtype + 1,
		LLRPStatus:     successStatus,
	}
	td.write(msg.id, &resp)
}

// handleGetReaderConfig is a custom handler for the GetReaderConfig message type
func (sim *Simulator) handleGetReaderConfig(td *TestDevice, msg Message) {
	td.write(msg.id, &sim.config.ReaderConfig)
}

// handleGetReaderCapabilities is a custom handler for the GetReaderCapabilities message type
func (sim *Simulator) handleGetReaderCapabilities(td *TestDevice, msg Message) {
	td.write(msg.id, &sim.config.ReaderCapabilities)
}

// handleAddROSpec is a custom handler for the AddROSpec message type
func (sim *Simulator) handleAddROSpec(td *TestDevice, msg Message) {
	addRO := &AddROSpec{}
	if err := msg.UnmarshalTo(addRO); err != nil {
		sim.Logger.Printf("Failed to unmarshal async event from LLRP. error: %v\n", err)
		return
	}

	if sim.state.ro != nil {
		// todo: what should this return?
		if sim.state.ro.ROSpecID == addRO.ROSpec.ROSpecID {
			td.write(msg.id, &AddROSpecResponse{LLRPStatus: LLRPStatus{
				Status:           StatusFieldInvalid,
				ErrorDescription: fmt.Sprintf("ROSpec already exists with id %d", sim.state.ro.ROSpecID),
			}})
		} else {
			td.write(msg.id, &AddROSpecResponse{LLRPStatus: LLRPStatus{
				Status:           StatusFieldInvalid,
				ErrorDescription: "Only one ROSpec supported by this device",
			}})
		}
		return
	}

	if addRO.ROSpec.ROReportSpec != nil {
		sim.Logger.Printf("addRo ROReportSpec: %+v", addRO.ROSpec.ROReportSpec)
		for _, c := range addRO.ROSpec.ROReportSpec.Custom {
			if c.VendorID == Impinj.Value() &&
				c.Subtype == uint32(ParamImpinjTagReportContentSelector) {
				sim.Logger.Printf("ImpinjTagReportContentSelector: %+v", c)
			}
		}
	}
	sim.Logger.Printf("addROSpec: id=%d", addRO.ROSpec.ROSpecID)

	sim.state.ro = &addRO.ROSpec
	td.write(msg.id, &AddROSpecResponse{LLRPStatus: successStatus})
}

// handleDeleteROSpec is a custom handler for the DeleteROSpec message type
func (sim *Simulator) handleDeleteROSpec(td *TestDevice, msg Message) {
	delRO := &DeleteROSpec{}
	if err := msg.UnmarshalTo(delRO); err != nil {
		sim.Logger.Printf("Failed to unmarshal async event from LLRP. error: %v\n", err)
		return
	}

	// note: id=0 means all
	sim.Logger.Printf("delROSpec: id=%d", delRO.ROSpecID)

	if delRO.ROSpecID != 0 && sim.state.ro == nil || sim.state.ro.ROSpecID != delRO.ROSpecID {
		// todo: what should this return?
		td.write(msg.id, &DeleteROSpecResponse{LLRPStatus: LLRPStatus{
			Status:           StatusFieldInvalid,
			ErrorDescription: fmt.Sprintf("Missing ROSpec with id %d", delRO.ROSpecID),
		}})
		return
	}

	// remove roSpec
	sim.state.ro = nil

	td.write(msg.id, &DeleteROSpecResponse{LLRPStatus: successStatus})
}

// handleStartROSpec is a custom handler for the StartROSpec message type
func (sim *Simulator) handleStartROSpec(td *TestDevice, msg Message) {
	sim.startReading()
	td.write(msg.id, &StartROSpecResponse{LLRPStatus: successStatus})
}

// handleEnableROSpec is a custom handler for the EnableROSpec message type
func (sim *Simulator) handleEnableROSpec(td *TestDevice, msg Message) {
	ro := sim.state.ro
	ro.ROSpecCurrentState = ROSpecStateInactive

	if ro == nil {
		// todo: better error
		td.write(msg.id, &EnableROSpecResponse{LLRPStatus: LLRPStatus{
			Status:           StatusFieldInvalid,
			ErrorDescription: "No ROSpec currently configured",
		}})
		return
	}

	sim.state.roInterval = time.Second / time.Duration(sim.flags.ReadRate)
	if ro.ROReportSpec != nil {
		switch ro.ROReportSpec.Trigger {
		case NSecondsOrAIEnd, NSecondsOrROEnd:
			sim.state.roInterval = time.Duration(ro.ROReportSpec.N) * time.Second
		case NMillisOrAIEnd, NMillisOrROEnd:
			sim.state.roInterval = time.Duration(ro.ROReportSpec.N) * time.Millisecond
		}
	}
	sim.Logger.Printf("setting read interval to %v", sim.state.roInterval)

	if ro.ROBoundarySpec.StartTrigger.Trigger == ROStartTriggerImmediate {
		sim.startReading()
	}

	if ro.ROBoundarySpec.StopTrigger.Trigger == ROStopTriggerDuration {
		stopInterval := time.Duration(ro.ROBoundarySpec.StopTrigger.DurationTriggerValue) * time.Millisecond
		sim.Logger.Printf("Setting stop trigger interval to %v", stopInterval)
		sim.stopTicker.Reset(stopInterval)
	}

	td.write(msg.id, &EnableROSpecResponse{LLRPStatus: successStatus})
}

// handleStopROSpec is a custom handler for the StopROSpec message type
func (sim *Simulator) handleStopROSpec(td *TestDevice, msg Message) {
	sim.stopReading()
	td.write(msg.id, &StopROSpecResponse{LLRPStatus: successStatus})
}

// handleDisableROSpec is a custom handler for the DisableROSpec message type
func (sim *Simulator) handleDisableROSpec(td *TestDevice, msg Message) {
	sim.state.reading = false
	sim.roTicker.Stop()
	sim.Logger.Println("Reading is disabled!")

	td.write(msg.id, &DisableROSpecResponse{LLRPStatus: successStatus})
}

// handleAddAccessSpec is a custom handler for the AddAccessSpec message type
func (sim *Simulator) handleAddAccessSpec(td *TestDevice, msg Message) {
	td.write(msg.id, &AddAccessSpecResponse{LLRPStatus: successStatus})
}

// handleEnableAccessSpec is a custom handler for the EnableAccessSpec message type
func (sim *Simulator) handleEnableAccessSpec(td *TestDevice, msg Message) {
	td.write(msg.id, &EnableAccessSpecResponse{LLRPStatus: successStatus})
}

// handleDisableAccessSpec is a custom handler for the DisableAccessSpec message type
func (sim *Simulator) handleDisableAccessSpec(td *TestDevice, msg Message) {
	td.write(msg.id, &DisableAccessSpecResponse{LLRPStatus: successStatus})
}

// handleDeleteAccessSpec is a custom handler for the DeleteAccessSpec message type
func (sim *Simulator) handleDeleteAccessSpec(td *TestDevice, msg Message) {
	td.write(msg.id, &DeleteAccessSpecResponse{LLRPStatus: successStatus})
}

// handleSetReaderConfig is a custom handler for the SetReaderConfig message type
func (sim *Simulator) handleSetReaderConfig(td *TestDevice, msg Message) {
	cfg := &SetReaderConfig{}
	if err := msg.UnmarshalTo(cfg); err != nil {
		sim.Logger.Printf("Failed to unmarshal async event from LLRP. error: %v\n", err)
		return
	}

	if cfg.KeepAliveSpec != nil && cfg.KeepAliveSpec.Trigger == KATriggerPeriodic {
		dur := time.Duration(cfg.KeepAliveSpec.Interval) * time.Millisecond
		sim.Logger.Printf("Setting keep alive duration to %v\n", dur)
		sim.kaTicker.Reset(dur)
	} else { // KATriggerNone
		sim.Logger.Println("Disabling keep alive")
		sim.kaTicker.Stop()
	}

	if cfg.ResetToFactoryDefaults {
		sim.resetToFactoryDefaults()
	}

	td.write(msg.id, &SetReaderConfigResponse{LLRPStatus: successStatus})
}
