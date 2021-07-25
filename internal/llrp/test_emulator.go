//
// Copyright (C) 2020 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package llrp

import (
	"github.com/pkg/errors"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// TestEmulator acts like an LLRP server. It accepts connections on a configurable network port,
// and when an LLRP client dials this TestEmulator, a new TestDevice is created which manages only
// the "reader" side of the connection.
//
// Functions exist to allow developer to provide "canned" responses to messages, and change them on the fly
// to allow changing
//
// NOTE: Unlike an actual LLRP server/reader, more than one simultaneous client are allowed to connect to it,
//		 however each client will receive the same canned responses.
type TestEmulator struct {
	silent bool

	listener net.Listener
	done     uint32
	connId   uint32

	// keep track of canned responses
	responses   map[MessageType]Outgoing
	responsesMu sync.Mutex

	// keep track of custom handlers
	handlers   map[MessageType]HandlerCallback
	handlersMu sync.Mutex

	// active connections/devices
	devices   map[*TestDevice]uint32
	devicesMu sync.Mutex
}

func NewTestEmulator(silent bool) *TestEmulator {
	return &TestEmulator{
		silent:    silent,
		responses: make(map[MessageType]Outgoing),
		handlers:  make(map[MessageType]HandlerCallback),
		devices:   make(map[*TestDevice]uint32),
	}
}

// StartAsync starts listening on a specific port until emu.Shutdown() is called
func (emu *TestEmulator) StartAsync(port int) error {
	var err error

	emu.listener, err = net.Listen("tcp4", ":"+strconv.Itoa(port))
	if err != nil {
		return err
	}

	go emu.listenUntilCancelled()
	return nil
}

type MultiErr []error

//goland:noinspection GoReceiverNames
func (me MultiErr) Error() string {
	strs := make([]string, len(me))
	for i, s := range me {
		strs[i] = s.Error()
	}

	return strings.Join(strs, "; ")
}

// Shutdown attempts to cleanly shutdown the emulator
func (emu *TestEmulator) Shutdown() error {
	var errs MultiErr
	atomic.StoreUint32(&emu.done, 1)
	if err := emu.listener.Close(); err != nil {
		errs = append(errs, err)
	}

	emu.devicesMu.Lock()
	defer emu.devicesMu.Unlock()

	for dev := range emu.devices {
		if err := dev.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.New(errs.Error())
	}
	return nil
}

// listenUntilCancelled listens forever on the net.Listener until emu.Shutdown() is called
func (emu *TestEmulator) listenUntilCancelled() {
	for {
		conn, err := emu.listener.Accept()
		if atomic.LoadUint32(&emu.done) == 1 {
			return
		} else if err != nil {
			panic("listener accept failed: " + err.Error())
		}

		go emu.handleNewConn(conn)
	}
}

// handleNewConn handles a new incoming connection the net.Listener (should be spawned async)
func (emu *TestEmulator) handleNewConn(conn net.Conn) {
	id := atomic.AddUint32(&emu.connId, 1)
	log.Printf("handing new connection. id=%d", id)
	td, err := NewReaderOnlyTestDevice(conn, 60*time.Second, emu.silent)
	if err != nil {
		panic("unable to create a new ReaderOnly TestDevice: " + err.Error())
	}

	emu.devicesMu.Lock()
	if len(emu.devices) > 0 {
		// todo: we might have a connection leak somewhere, so this code causes issues if we are disconnected
		// 		 abruptly and client attempts to reconnect
		// todo note: I think there may not be a connection leak, but actually a bug with one
		//			  or both of the services where the reader gets put in a DISABLED state and not
		//			  reset back to ENABLED, or the inventory service is not listening for when these
		//			  events occur.
		log.Printf("error, already connected to another client. active connection count: %d", len(emu.devices))
		td.write(td.nextMessageId(), NewConnectMessage(ConnExistsClientInitiated))
		// Note: not calling td.Close directly because that will send a ConnectionClose event
		// 		 that we do NOT want.
		_ = td.reader.Close()
		_ = td.rConn.Close()
		emu.devicesMu.Unlock()
		return
	}
	emu.devicesMu.Unlock()

	emu.responsesMu.Lock()
	for mt, out := range emu.responses {
		td.SetResponse(mt, out)
	}
	emu.responsesMu.Unlock()

	emu.handlersMu.Lock()
	for mt2, handler := range emu.handlers {
		h := handler
		td.reader.handlers[mt2] = MessageHandlerFunc(func(c *Client, msg Message) {
			if td.wrongVersion(msg) {
				return
			}
			h.Handle(td, msg)
		})
	}
	emu.handlersMu.Unlock()

	td.reader.handlers[MsgCloseConnection] = emu.createCloseHandler(td)

	emu.devicesMu.Lock()
	emu.devices[td] = id
	log.Printf("active connection count: %d", len(emu.devices))
	emu.devicesMu.Unlock()

	// blocks until connection closed
	td.ImpersonateReader()

	log.Printf("connection id=%d ended", id)
	emu.devicesMu.Lock()
	delete(emu.devices, td)
	log.Printf("active connection count: %d", len(emu.devices))
	emu.devicesMu.Unlock()
}

// SetResponse adds a canned response to all future clients. Optionally,
// if `applyExisting` is true, this will affect all currently active clients.
//
// NOTE 1: This will OVERRIDE existing response for given message type
//
// NOTE 2: Setting the response for MsgCloseConnection is NOT SUPPORTED and will be overridden by internal handler.
func (emu *TestEmulator) SetResponse(mt MessageType, out Outgoing) {
	emu.responsesMu.Lock()
	emu.responses[mt] = out
	emu.responsesMu.Unlock()
}

// SetHandler adds a canned response to all future clients. Optionally,
// if `applyExisting` is true, this will affect all currently active clients.
//
// NOTE 1: This will OVERRIDE existing response for given message type
//
// NOTE 2: Setting the response for MsgCloseConnection is NOT SUPPORTED and will be overridden by internal handler.
func (emu *TestEmulator) SetHandler(mt MessageType, handler HandlerCallback) {
	emu.handlersMu.Lock()
	emu.handlers[mt] = handler
	emu.handlersMu.Unlock()
}

// createCloseHandler handles a client request to close the connection.
func (emu *TestEmulator) createCloseHandler(td *TestDevice) MessageHandlerFunc {
	return func(_ *Client, msg Message) {
		if td.wrongVersion(msg) {
			return
		}

		td.write(msg.id, &CloseConnectionResponse{})

		emu.devicesMu.Lock()
		id := emu.devices[td]
		emu.devicesMu.Unlock()
		log.Printf("closing connection id=%d", id)

		_ = td.reader.Close()
		_ = td.rConn.Close()

		emu.devicesMu.Lock()
		// remove it from the set of active connections
		delete(emu.devices, td)
		log.Printf("active connection count: %d", len(emu.devices))
		emu.devicesMu.Unlock()
	}
}
