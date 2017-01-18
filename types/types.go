// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package types

import (
	"github.com/TheThingsNetwork/ttn/api/gateway"
	"github.com/TheThingsNetwork/ttn/api/router"
)

// UplinkMessage is used internally
type UplinkMessage struct {
	GatewayID string
	Message   *router.UplinkMessage
}

// DownlinkMessage is used internally
type DownlinkMessage struct {
	GatewayID string
	Message   *router.DownlinkMessage
}

// StatusMessage is used internally
type StatusMessage struct {
	GatewayID string
	Message   *gateway.Status
}
