// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package middleware

import (
	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
)

// Context for middleware
type Context interface {
	Set(k, v interface{})
	Get(k interface{}) interface{}
}

// NewContext returns a new middleware context
func NewContext() Context {
	return &context{
		data: make(map[interface{}]interface{}),
	}
}

type context struct {
	data map[interface{}]interface{}
}

func (c *context) Set(k, v interface{}) {
	c.data[k] = v
}

func (c *context) Get(k interface{}) interface{} {
	if v, ok := c.data[k]; ok {
		return v
	}
	return nil
}

// Chain of middleware
type Chain []interface{}

// Execute the chain
func (c Chain) Execute(ctx Context, msg interface{}) error {
	switch msg := msg.(type) {
	case *types.ConnectMessage:
		return c.filterConnect().Execute(ctx, msg)
	case *types.DisconnectMessage:
		return c.filterDisconnect().Execute(ctx, msg)
	case *types.UplinkMessage:
		return c.filterUplink().Execute(ctx, msg)
	case *types.StatusMessage:
		return c.filterStatus().Execute(ctx, msg)
	case *types.DownlinkMessage:
		return c.filterDownlink().Execute(ctx, msg)
	}
	return nil
}

// Connect middleware
type Connect interface {
	HandleConnect(Context, *types.ConnectMessage) error
}

type connectChain []Connect

func (c connectChain) Execute(ctx Context, msg *types.ConnectMessage) error {
	for _, middleware := range c {
		err := middleware.HandleConnect(ctx, msg)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c Chain) filterConnect() (filtered connectChain) {
	for _, middleware := range c {
		if c, ok := middleware.(Connect); ok {
			filtered = append(filtered, c)
		}
	}
	return
}

// Disconnect middleware
type Disconnect interface {
	HandleDisconnect(Context, *types.DisconnectMessage) error
}

type disconnectChain []Disconnect

func (c disconnectChain) Execute(ctx Context, msg *types.DisconnectMessage) error {
	for _, middleware := range c {
		err := middleware.HandleDisconnect(ctx, msg)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c Chain) filterDisconnect() (filtered disconnectChain) {
	for _, middleware := range c {
		if c, ok := middleware.(Disconnect); ok {
			filtered = append(filtered, c)
		}
	}
	return
}

// Uplink middleware
type Uplink interface {
	HandleUplink(Context, *types.UplinkMessage) error
}

type uplinkChain []Uplink

func (c uplinkChain) Execute(ctx Context, msg *types.UplinkMessage) error {
	for _, middleware := range c {
		err := middleware.HandleUplink(ctx, msg)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c Chain) filterUplink() (filtered uplinkChain) {
	for _, middleware := range c {
		if c, ok := middleware.(Uplink); ok {
			filtered = append(filtered, c)
		}
	}
	return
}

// Status middleware
type Status interface {
	HandleStatus(Context, *types.StatusMessage) error
}

type statusChain []Status

func (c statusChain) Execute(ctx Context, msg *types.StatusMessage) error {
	for _, middleware := range c {
		err := middleware.HandleStatus(ctx, msg)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c Chain) filterStatus() (filtered statusChain) {
	for _, middleware := range c {
		if c, ok := middleware.(Status); ok {
			filtered = append(filtered, c)
		}
	}
	return
}

// Downlink middleware
type Downlink interface {
	HandleDownlink(Context, *types.DownlinkMessage) error
}

type downlinkChain []Downlink

func (c downlinkChain) Execute(ctx Context, msg *types.DownlinkMessage) error {
	for _, middleware := range c {
		err := middleware.HandleDownlink(ctx, msg)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c Chain) filterDownlink() (filtered downlinkChain) {
	for _, middleware := range c {
		if c, ok := middleware.(Downlink); ok {
			filtered = append(filtered, c)
		}
	}
	return
}
