// Copyright © 2016 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package bridge

import "github.com/TheThingsNetwork/gateway-connector-bridge/bridge/ttn"

// SetupTTNRouter sets up the TTN Router
func (b *Bridge) SetupTTNRouter(config ttn.RouterConfig) error {
	router, err := ttn.New(config, b.Ctx, b.gatewayToken)
	if err != nil {
		return err
	}
	b.ttnRouter = router
	return nil
}
