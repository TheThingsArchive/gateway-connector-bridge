// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package dummy

import "github.com/TheThingsNetwork/gateway-connector-bridge/types"

// WithServer is a Dummy backend that exposes some events on
// a http page with websockets
type WithServer struct {
	*Dummy
	server *Server
}

// WithHTTPServer returns the Dummy that also has a HTTP server exposing the events on addr
func (d *Dummy) WithHTTPServer(addr string) *WithServer {
	ctx := d.ctx.WithField("Connector", "HTTP Debug")
	s, err := NewServer(ctx, addr)
	if err != nil {
		ctx.WithError(err).Fatal("Could not add server to Dummy backend")
		return nil
	}
	go s.Listen()
	return &WithServer{
		Dummy:  d,
		server: s,
	}
}

// PublishUplink implements backend interfaces
func (d *WithServer) PublishUplink(message *types.UplinkMessage) error {
	uplink := *message.Message
	uplink.UnmarshalPayload()
	d.server.Uplink(&types.UplinkMessage{GatewayID: message.GatewayID, Message: &uplink})
	return nil
}

// PublishStatus implements backend interfaces
func (d *WithServer) PublishStatus(message *types.StatusMessage) error {
	d.server.Status(message)
	return nil
}

// PublishDownlink implements backend interfaces
func (d *WithServer) PublishDownlink(message *types.DownlinkMessage) error {
	downlink := *message.Message
	downlink.UnmarshalPayload()
	d.server.Downlink(&types.DownlinkMessage{GatewayID: message.GatewayID, Message: &downlink})
	return nil
}

// SubscribeUplink implements backend interfaces
func (d *WithServer) SubscribeUplink(gatewayID string) (<-chan *types.UplinkMessage, error) {
	d.server.Connect(gatewayID)
	return d.Dummy.SubscribeUplink(gatewayID)
}

// UnsubscribeUplink implements backend interfaces
func (d *WithServer) UnsubscribeUplink(gatewayID string) error {
	d.server.Disconnect(gatewayID)
	return d.Dummy.UnsubscribeUplink(gatewayID)
}
