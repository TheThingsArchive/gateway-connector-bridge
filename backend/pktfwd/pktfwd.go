// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package pktfwd

import (
	"sync"

	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
	"github.com/TheThingsNetwork/go-utils/log"
	"github.com/brocaar/lorawan"
)

// Config contains configuration for PacketForwarder
type Config struct {
	Bind string
}

// New returns a new Dummy backend
func New(config Config, ctx log.Interface) *PacketForwarder {
	f := &PacketForwarder{
		config:     config,
		ctx:        ctx.WithField("Connector", "PacketForwarder"),
		connect:    make(chan *types.ConnectMessage),
		disconnect: make(chan *types.DisconnectMessage),
		uplink:     make(map[string]chan *types.UplinkMessage),
		status:     make(map[string]chan *types.StatusMessage),
	}
	return f
}

// PacketForwarder backend based on github.com/brocaar/lora-gateway-bridge
type PacketForwarder struct {
	config  Config
	backend *Backend
	ctx     log.Interface

	mu         sync.RWMutex
	connect    chan *types.ConnectMessage
	disconnect chan *types.DisconnectMessage
	uplink     map[string]chan *types.UplinkMessage
	status     map[string]chan *types.StatusMessage
}

// Connect implements the Southbound interface
func (f *PacketForwarder) Connect() (err error) {
	f.backend, err = NewBackend(f.config.Bind, f.onNew, f.onDelete, false)
	if err != nil {
		return err
	}
	f.backend.log = f.ctx

	go func() {
		for uplink := range f.backend.RXPacketChan() {
			f.mu.RLock()
			if ch, ok := f.uplink[uplink.GatewayID]; ok {
				f.mu.RUnlock()
				ch <- uplink
			} else if ch, ok := f.uplink[""]; ok {
				f.mu.RUnlock()
				ch <- uplink
			} else {
				f.mu.RUnlock()
				f.ctx.WithField("GatewayID", uplink.GatewayID).Debug("Dropping uplink for inactive gateway")
			}
		}
	}()

	go func() {
		for status := range f.backend.StatsChan() {
			status.Backend = "PacketForwarder"
			f.mu.RLock()
			if ch, ok := f.status[status.GatewayID]; ok {
				f.mu.RUnlock()
				ch <- status
			} else if ch, ok := f.status[""]; ok {
				f.mu.RUnlock()
				ch <- status
			} else {
				f.mu.RUnlock()
				f.ctx.WithField("GatewayID", status.GatewayID).Debug("Dropping status for inactive gateway")
			}
		}
	}()

	return
}

// Disconnect implements the Southbound interface
func (f *PacketForwarder) Disconnect() error {
	return f.backend.Close()
}

func (f *PacketForwarder) onNew(mac lorawan.EUI64) error {
	f.connect <- &types.ConnectMessage{GatewayID: getID(mac)}
	return nil
}

func (f *PacketForwarder) onDelete(mac lorawan.EUI64) error {
	f.disconnect <- &types.DisconnectMessage{GatewayID: getID(mac)}
	return nil
}

// SubscribeConnect implements the Southbound interface
func (f *PacketForwarder) SubscribeConnect() (<-chan *types.ConnectMessage, error) {
	return f.connect, nil
}

// UnsubscribeConnect implements the Southbound interface
func (f *PacketForwarder) UnsubscribeConnect() error {
	close(f.connect)
	return nil
}

// SubscribeDisconnect implements the Southbound interface
func (f *PacketForwarder) SubscribeDisconnect() (<-chan *types.DisconnectMessage, error) {
	return f.disconnect, nil
}

// UnsubscribeDisconnect implements the Southbound interface
func (f *PacketForwarder) UnsubscribeDisconnect() error {
	close(f.disconnect)
	return nil
}

// SubscribeUplink implements the Southbound interface
func (f *PacketForwarder) SubscribeUplink(gatewayID string) (<-chan *types.UplinkMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.uplink[gatewayID] = make(chan *types.UplinkMessage)
	return f.uplink[gatewayID], nil
}

// UnsubscribeUplink implements the Southbound interface
func (f *PacketForwarder) UnsubscribeUplink(gatewayID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if ch, ok := f.uplink[gatewayID]; ok {
		close(ch)
	}
	delete(f.uplink, gatewayID)
	return nil
}

// SubscribeStatus implements the Southbound interface
func (f *PacketForwarder) SubscribeStatus(gatewayID string) (<-chan *types.StatusMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.status[gatewayID] = make(chan *types.StatusMessage)
	return f.status[gatewayID], nil
}

// UnsubscribeStatus implements the Southbound interface
func (f *PacketForwarder) UnsubscribeStatus(gatewayID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if ch, ok := f.status[gatewayID]; ok {
		close(ch)
	}
	delete(f.status, gatewayID)
	return nil
}

// PublishDownlink implements the Southbound interface
func (f *PacketForwarder) PublishDownlink(message *types.DownlinkMessage) error {
	return f.backend.Send(message)
}
