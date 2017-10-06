// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package deduplicate

import (
	"bytes"
	"errors"
	"sync"

	"github.com/TheThingsNetwork/gateway-connector-bridge/middleware"
	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
	"github.com/TheThingsNetwork/go-utils/log"
)

// NewDeduplicate returns a middleware that deduplicates duplicate uplink messages received from broken gateways
func NewDeduplicate() *Deduplicate {
	return &Deduplicate{
		log:         log.Get(),
		lastMessage: make(map[string]*types.UplinkMessage),
	}
}

// Deduplicate middleware
type Deduplicate struct {
	log         log.Interface
	mu          sync.RWMutex
	lastMessage map[string]*types.UplinkMessage
}

// HandleDisconnect cleans up
func (d *Deduplicate) HandleDisconnect(ctx middleware.Context, msg *types.DisconnectMessage) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.lastMessage, msg.GatewayID)
	return nil
}

// ErrDuplicateMessage is returned when an uplink message is received multiple times
var ErrDuplicateMessage = errors.New("deduplicate: already handled this message")

// HandleUplink blocks duplicate messages
func (d *Deduplicate) HandleUplink(_ middleware.Context, msg *types.UplinkMessage) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if lastMessage, ok := d.lastMessage[msg.GatewayID]; ok {
		if bytes.Equal(msg.Message.Payload, lastMessage.Message.Payload) && // length check on slice is fast
			msg.Message.GatewayMetadata.GetTimestamp() == lastMessage.Message.GatewayMetadata.GetTimestamp() {
			return ErrDuplicateMessage
		}
	}
	d.lastMessage[msg.GatewayID] = msg
	return nil
}
