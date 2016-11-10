// Copyright © 2016 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package bridge

import "github.com/TheThingsNetwork/gateway-connector-bridge/bridge/types"

type NorthboundBackend interface {
	CleanupGateway(gatewayID string)
	Connect() error
	Disconnect() error
	PublishUplink(message *types.UplinkMessage) error
	PublishStatus(message *types.StatusMessage) error
	SubscribeDownlink(gatewayID string) (<-chan *types.DownlinkMessage, error)
	UnsubscribeDownlink(gatewayID string) error
}

type SouthboundBackend interface {
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
