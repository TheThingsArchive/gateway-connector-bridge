// Copyright © 2016 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package bridge

import "github.com/TheThingsNetwork/gateway-connector-bridge/bridge/mqtt"

// SetupMQTT sets up MQTT
func (b *Bridge) SetupMQTT(config mqtt.Config) error {
	mqtt, err := mqtt.New(config, b.Ctx)
	if err != nil {
		return err
	}
	b.mqtt = mqtt
	return nil
}
