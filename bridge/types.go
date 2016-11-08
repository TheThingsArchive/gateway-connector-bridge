// Copyright © 2016 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package bridge

import (
	"github.com/TheThingsNetwork/ttn/api/gateway"
	"github.com/TheThingsNetwork/ttn/api/router"
)

type ConnectMessage struct {
	GatewayID string `json:"id"`
	Token     string `json:"token"`
}

type DisconnectMessage struct {
	GatewayID string `json:"id"`
}

type UplinkMessage struct {
	gatewayID string
	token     string
	message   *router.UplinkMessage
}

type DownlinkMessage struct {
	gatewayID string
	token     string
	message   *router.DownlinkMessage
}

type StatusMessage struct {
	gatewayID string
	token     string
	message   *gateway.Status
}
