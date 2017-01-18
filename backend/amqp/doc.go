// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

// Package amqp connects to an AMQP server in order to communicate with a gateway.
//
// Connection/Disconnection of gateways is done by publishing messages to the
// "connect" and "disconnect" routing keys. When a gateway connects, it (or a
// plugin on the broker) should publish a protocol buffer of the type
// types.ConnectMessage containing the gateway's ID and either a key or a token
// to the "connect" routing key .
//
// The gateway (or the plugin) can send a protocol buffer of the type
// types.DisconnectMessage containing the gateway's ID on the "disconnect"
// routing key when it disconnects, in order to help the bridge clean up
// connections.
//
// Uplink messages are sent as protocol buffers on the "[gateway-id].up" routing
// key. The bridge should call `SubscribeUplink("gateway-id")` to subscribe to
// these. It is also possible to subscribe to a wildcard gateway by passing "*".
//
// Downlink messages are sent as protocol buffers on the "[gateway-id].down"
// routing key. The bridge should call `PublishDownlink(*types.DownlinkMessage)`
// in order to send the downlink to the gateway.
//
// Gateway status messages are sent as protocol buffers on the
// "[gateway-id].status" routing key. The bridge should call
// `SubscribeStatus("gateway-id")` to subscribe to these. It is also possible to
// subscribe to a wildcard gateway by passing "*".
package amqp
