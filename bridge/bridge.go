// Copyright © 2016 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package bridge

import (
	"sync"

	"github.com/TheThingsNetwork/gateway-connector-bridge/bridge/mqtt"
	"github.com/TheThingsNetwork/gateway-connector-bridge/bridge/ttn"
	"github.com/apex/log"
)

// Bridge is the bridge between MQTT and gRPC
type Bridge struct {
	Ctx       log.Interface
	mqtt      *mqtt.MQTT
	ttnRouter *ttn.Router
	tokens    map[string]string
	mu        sync.Mutex
}

func (b *Bridge) gatewayToken(gatewayID string) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if token, ok := b.tokens[gatewayID]; ok {
		return token
	}
	return ""
}
