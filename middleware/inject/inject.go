// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package inject

import (
	"github.com/TheThingsNetwork/gateway-connector-bridge/middleware"
	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
	"github.com/TheThingsNetwork/go-utils/log"
)

// Fields to inject
type Fields struct {
	FrequencyPlan string
	Bridge        string
}

// NewInject returns a middleware that injects fields into all status messages
func NewInject(fields Fields) *Inject {
	return &Inject{
		fields: fields,
		log:    log.Get(),
	}
}

// Inject fields into all status messages
type Inject struct {
	log    log.Interface
	fields Fields
}

// HandleStatus inserts fields into status messages if not present
func (i *Inject) HandleStatus(ctx middleware.Context, msg *types.StatusMessage) error {
	if msg.Message.FrequencyPlan == "" {
		msg.Message.FrequencyPlan = i.fields.FrequencyPlan
	}
	if msg.Message.Bridge == "" {
		msg.Message.Bridge = i.fields.Bridge
		if msg.Backend != "" {
			msg.Message.Bridge += " " + msg.Backend + " Backend"
		}
	}
	return nil
}
