// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

// Package mqtt connects to an MQTT broker in order to communicate with a gateway.
//
// Connection/Disconnection of gateways is done by publishing messages to the
// "connect" and "disconnect" topics. When a gateway connects, it (or a plugin
// of the MQTT broker) should publish a protocol buffer of the type
// types.ConnectMessage containing the gateway's ID and either a key or a
// token to the "connect" topic.
//
// The gateway (or the plugin) can also set a will with the MQTT broker for
// when it disconnects. This will should be a protocol buffer of the type
// types.DisconnectMessage containing the gateway's ID on the "disconnect"
// topic.
//
// Uplink messages are sent as protocol buffers on the "[gateway-id]/up" topic.
// The bridge should call `SubscribeUplink("gateway-id")` to subscribe to this
// topic. It is also possible to subscribe to a wildcard gateway by passing "+".
//
// Downlink messages are sent as protocol buffers on the "[gateway-id]/down"
// topic. The bridge should call `PublishDownlink(*types.DownlinkMessage)` in
// order to send the downlink to the gateway.
//
// Gateway status messages are sent as protocol buffers on the
// "[gateway-id]/status" topic. The bridge should call
// `SubscribeStatus("gateway-id")` to subscribe to this topic. It is also
// possible to subscribe to a wildcard gateway by passing "+".
package mqtt
