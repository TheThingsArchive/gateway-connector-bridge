// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package exchange

import (
	"strings"

	"github.com/TheThingsNetwork/api/protocol"
	"github.com/prometheus/client_golang/prometheus"
)

var connectedGateways = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Namespace: "ttn",
		Subsystem: "bridge",
		Name:      "connected_gateways",
		Help:      "Number of connected gateways.",
	},
)

var handledCounter = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "ttn",
		Subsystem: "bridge",
		Name:      "messages_handled_total",
		Help:      "Total number of messages handled.",
	}, []string{"message_type"},
)

func messageType(msg *protocol.Message) string {
	if msg := msg.GetLoRaWAN(); msg != nil {
		mType := msg.GetMType().String()
		return strings.Replace(strings.Title(strings.ToLower(strings.Replace(mType, "_", " ", -1))), " ", "", -1)
	}
	return "Unknown"
}

type message interface {
	UnmarshalPayload() error
	GetPayload() []byte
	GetMessage() *protocol.Message
}

func registerHandled(msg message) {
	msg.UnmarshalPayload()
	handledCounter.WithLabelValues(messageType(msg.GetMessage())).Inc()
}

func registerStatus() {
	handledCounter.WithLabelValues("GatewayStatus").Inc()
}

func init() {
	prometheus.MustRegister(connectedGateways)
	prometheus.MustRegister(handledCounter)
}
