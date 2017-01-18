// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package dummy

import (
	"sync"

	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
	"github.com/apex/log"
)

// BufferSize indicates the maximum number of dummy messages that should be buffered
var BufferSize = 10

type dummyGateway struct {
	sync.Mutex
	uplink   chan *types.UplinkMessage
	status   chan *types.StatusMessage
	downlink chan *types.DownlinkMessage
}

// Dummy backend
type Dummy struct {
	mu         sync.Mutex
	ctx        log.Interface
	connect    chan *types.ConnectMessage
	disconnect chan *types.DisconnectMessage
	gateways   map[string]*dummyGateway
}

// New returns a new Dummy backend
func New(ctx log.Interface) *Dummy {
	return &Dummy{
		ctx:      ctx.WithField("Connector", "Dummy"),
		gateways: make(map[string]*dummyGateway),
	}
}

// Connect implements backend interfaces
func (d *Dummy) Connect() error {
	d.ctx.Debug("Connected")
	return nil
}

// Disconnect implements backend interfaces
func (d *Dummy) Disconnect() error {
	d.ctx.Debug("Disconnected")
	return nil
}

// PublishConnect publishes connect messages to the dummy backend
func (d *Dummy) PublishConnect(message *types.ConnectMessage) error {
	select {
	case d.connect <- message:
		d.ctx.Debug("Published connect")
	default:
		d.ctx.Debug("Did not publish connect [buffer full]")
	}
	return nil
}

// SubscribeConnect implements backend interfaces
func (d *Dummy) SubscribeConnect() (<-chan *types.ConnectMessage, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.connect = make(chan *types.ConnectMessage, BufferSize)
	d.ctx.Debug("Subscribed to connect")
	return d.connect, nil
}

// UnsubscribeConnect implements backend interfaces
func (d *Dummy) UnsubscribeConnect() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	close(d.connect)
	d.connect = nil
	d.ctx.Debug("Unsubscribed from connect")
	return nil
}

// PublishDisconnect publishes disconnect messages to the dummy backend
func (d *Dummy) PublishDisconnect(message *types.DisconnectMessage) error {
	select {
	case d.disconnect <- message:
		d.ctx.Debug("Published disconnect")
	default:
		d.ctx.Debug("Did not publish disconnect [buffer full]")
	}
	return nil
}

// SubscribeDisconnect implements backend interfaces
func (d *Dummy) SubscribeDisconnect() (<-chan *types.DisconnectMessage, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.disconnect = make(chan *types.DisconnectMessage, BufferSize)
	d.ctx.Debug("Subscribed to disconnect")
	return d.disconnect, nil
}

// UnsubscribeDisconnect implements backend interfaces
func (d *Dummy) UnsubscribeDisconnect() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	close(d.disconnect)
	d.disconnect = nil
	d.ctx.Debug("Unsubscribed from disconnect")
	return nil
}

func (d *Dummy) getGateway(gatewayID string) *dummyGateway {
	d.mu.Lock()
	defer d.mu.Unlock()
	if gtw, ok := d.gateways[gatewayID]; ok {
		return gtw
	}
	d.gateways[gatewayID] = &dummyGateway{
		uplink:   make(chan *types.UplinkMessage, BufferSize),
		status:   make(chan *types.StatusMessage, BufferSize),
		downlink: make(chan *types.DownlinkMessage, BufferSize),
	}
	return d.gateways[gatewayID]
}

// PublishUplink implements backend interfaces
func (d *Dummy) PublishUplink(message *types.UplinkMessage) error {
	select {
	case d.getGateway(message.GatewayID).uplink <- message:
		d.ctx.WithField("GatewayID", message.GatewayID).Debug("Published uplink")
	default:
		d.ctx.WithField("GatewayID", message.GatewayID).Debug("Did not publish uplink [buffer full]")
	}
	return nil
}

// SubscribeUplink implements backend interfaces
func (d *Dummy) SubscribeUplink(gatewayID string) (<-chan *types.UplinkMessage, error) {
	gtw := d.getGateway(gatewayID)
	gtw.Lock()
	defer gtw.Unlock()
	gtw.uplink = make(chan *types.UplinkMessage, BufferSize)
	d.ctx.WithField("GatewayID", gatewayID).Debug("Subscribed to uplink")
	return gtw.uplink, nil
}

// UnsubscribeUplink implements backend interfaces
func (d *Dummy) UnsubscribeUplink(gatewayID string) error {
	gtw := d.getGateway(gatewayID)
	gtw.Lock()
	defer gtw.Unlock()
	close(gtw.uplink)
	gtw.uplink = nil
	d.ctx.WithField("GatewayID", gatewayID).Debug("Unsubscribed from uplink")
	return nil
}

// PublishDownlink implements backend interfaces
func (d *Dummy) PublishDownlink(message *types.DownlinkMessage) error {
	select {
	case d.getGateway(message.GatewayID).downlink <- message:
		d.ctx.WithField("GatewayID", message.GatewayID).Debug("Published downlink")
	default:
		d.ctx.WithField("GatewayID", message.GatewayID).Debug("Did not publish downlink [buffer full]")
	}
	return nil
}

// SubscribeDownlink implements backend interfaces
func (d *Dummy) SubscribeDownlink(gatewayID string) (<-chan *types.DownlinkMessage, error) {
	gtw := d.getGateway(gatewayID)
	gtw.Lock()
	defer gtw.Unlock()
	gtw.downlink = make(chan *types.DownlinkMessage, BufferSize)
	d.ctx.WithField("GatewayID", gatewayID).Debug("Subscribed to downlink")
	return gtw.downlink, nil
}

// UnsubscribeDownlink implements backend interfaces
func (d *Dummy) UnsubscribeDownlink(gatewayID string) error {
	gtw := d.getGateway(gatewayID)
	gtw.Lock()
	defer gtw.Unlock()
	close(gtw.downlink)
	gtw.downlink = nil
	d.ctx.WithField("GatewayID", gatewayID).Debug("Unsubscribed from downlink")
	return nil
}

// PublishStatus implements backend interfaces
func (d *Dummy) PublishStatus(message *types.StatusMessage) error {
	select {
	case d.getGateway(message.GatewayID).status <- message:
		d.ctx.WithField("GatewayID", message.GatewayID).Debug("Published status")
	default:
		d.ctx.WithField("GatewayID", message.GatewayID).Debug("Did not publish status [buffer full]")
	}
	return nil
}

// SubscribeStatus implements backend interfaces
func (d *Dummy) SubscribeStatus(gatewayID string) (<-chan *types.StatusMessage, error) {
	gtw := d.getGateway(gatewayID)
	gtw.Lock()
	defer gtw.Unlock()
	gtw.status = make(chan *types.StatusMessage, BufferSize)
	d.ctx.WithField("GatewayID", gatewayID).Debug("Subscribed to status")
	return gtw.status, nil
}

// UnsubscribeStatus implements backend interfaces
func (d *Dummy) UnsubscribeStatus(gatewayID string) error {
	gtw := d.getGateway(gatewayID)
	gtw.Lock()
	defer gtw.Unlock()
	close(gtw.status)
	gtw.status = nil
	d.ctx.WithField("GatewayID", gatewayID).Debug("Unsubscribed from status")
	return nil
}

// CleanupGateway implements backend interfaces
func (d *Dummy) CleanupGateway(gatewayID string) {
	// Not closing channels here, that's not our job
	delete(d.gateways, gatewayID)
}
