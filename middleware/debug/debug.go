// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package debug

import (
	"github.com/TheThingsNetwork/gateway-connector-bridge/middleware"
	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
)

// New returns a middleware that debugs traffic
func New() *Debug {
	return &Debug{}
}

// Debug middleware
type Debug struct{}

// HandleUplink debugs uplink traffic
func (*Debug) HandleUplink(_ middleware.Context, msg *types.UplinkMessage) error {
	if lorawan := msg.Message.ProtocolMetadata.GetLoRaWAN(); lorawan != nil {
		if lorawan.FCnt != 0 {
			lorawan.FCnt = 0
		}
	}
	return nil
}
