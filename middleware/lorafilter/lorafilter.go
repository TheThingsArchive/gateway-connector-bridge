// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package lorafilter

import (
	"errors"
	"fmt"

	"github.com/TheThingsNetwork/gateway-connector-bridge/middleware"
	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
	"github.com/brocaar/lorawan"
)

// NewFilter returns a middleware that filters traffic so that non-LoRaWAN messages are ignored
func NewFilter() *Filter {
	return &Filter{}
}

// Filter middleware
type Filter struct{}

// HandleUplink blocks duplicate messages
func (*Filter) HandleUplink(_ middleware.Context, msg *types.UplinkMessage) error {
	payload := msg.Message.GetPayload()
	if len(payload) < 5 {
		return fmt.Errorf("lorafilter: %d payload bytes is not enough for a LoRaWAN packet", len(payload))
	}
	var mhdr lorawan.MHDR
	if err := mhdr.UnmarshalBinary(payload[:1]); err != nil {
		return err
	}
	if mhdr.Major != lorawan.LoRaWANR1 {
		return fmt.Errorf("lorafilter: unsupported LoRaWAN version 0x%x", byte(mhdr.Major))
	}
	switch mhdr.MType {
	case lorawan.JoinAccept:
		return errors.New("lorafilter: found JoinAccept payload in UplinkMessage")
	case lorawan.UnconfirmedDataDown, lorawan.ConfirmedDataDown:
		return errors.New("lorafilter: found Downlink payload in UplinkMessage")
	}
	return nil
}
