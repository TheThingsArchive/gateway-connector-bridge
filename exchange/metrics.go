// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package exchange

import (
	"github.com/TheThingsNetwork/api/protocol"
	"github.com/TheThingsNetwork/api/protocol/lorawan"
	"github.com/prometheus/client_golang/prometheus"
)

var info = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: "ttn",
		Subsystem: "bridge",
		Name:      "info",
		Help:      "Information about the TTN environment.",
	}, []string{
		"build_date", "git_commit", "id", "version",
	},
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

func mTypeToString(mType lorawan.MType) string {
	switch mType {
	case lorawan.MType_JOIN_REQUEST:
		return "JoinRequest"
	case lorawan.MType_JOIN_ACCEPT:
		return "JoinAccept"
	case lorawan.MType_UNCONFIRMED_UP:
		return "UnconfirmedUp"
	case lorawan.MType_UNCONFIRMED_DOWN:
		return "UnconfirmedDown"
	case lorawan.MType_CONFIRMED_UP:
		return "ConfirmedUp"
	case lorawan.MType_CONFIRMED_DOWN:
		return "ConfirmedDown"
	case 6:
		return "RejoinRequest"
	case 7:
		return "Proprietary"
	default:
		return "Unknown"
	}
}

func messageType(msg *protocol.Message) string {
	if msg := msg.GetLoRaWAN(); msg != nil {
		return mTypeToString(msg.GetMType())
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
	prometheus.MustRegister(info)
	prometheus.MustRegister(connectedGateways)
	prometheus.MustRegister(handledCounter)
	for mType := lorawan.MType(0); mType < 8; mType++ {
		handledCounter.WithLabelValues(mTypeToString(mType)).Add(0)
	}
}
