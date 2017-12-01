// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package types

import (
	"net"

	"github.com/TheThingsNetwork/api/gateway"
	"github.com/TheThingsNetwork/api/router"
)

// UplinkMessage is used internally
type UplinkMessage struct {
	GatewayID   string
	GatewayAddr net.Addr
	Message     *router.UplinkMessage
}

// DownlinkMessage is used internally
type DownlinkMessage struct {
	GatewayID string
	Message   *router.DownlinkMessage
}

// StatusMessage is used internally
type StatusMessage struct {
	Backend     string
	GatewayID   string
	GatewayAddr net.Addr
	Message     *gateway.Status
}
