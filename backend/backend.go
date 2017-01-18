// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package backend

import "github.com/TheThingsNetwork/gateway-connector-bridge/types"

// Northbound backends talk to servers that are up the chain
type Northbound interface {
	Connect() error
	Disconnect() error
	CleanupGateway(gatewayID string)
	PublishUplink(message *types.UplinkMessage) error
	PublishStatus(message *types.StatusMessage) error
	SubscribeDownlink(gatewayID string) (<-chan *types.DownlinkMessage, error)
	UnsubscribeDownlink(gatewayID string) error
}

// Southbound backends talk to gateways or servers that are down the chain
type Southbound interface {
	Connect() error
	Disconnect() error
	SubscribeConnect() (<-chan *types.ConnectMessage, error)
	UnsubscribeConnect() error
	SubscribeDisconnect() (<-chan *types.DisconnectMessage, error)
	UnsubscribeDisconnect() error
	SubscribeUplink(gatewayID string) (<-chan *types.UplinkMessage, error)
	UnsubscribeUplink(gatewayID string) error
	SubscribeStatus(gatewayID string) (<-chan *types.StatusMessage, error)
	UnsubscribeStatus(gatewayID string) error
	PublishDownlink(message *types.DownlinkMessage) error
}
