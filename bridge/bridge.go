// Copyright © 2016 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package bridge

import "github.com/apex/log"

// Bridge is the bridge between MQTT and gRPC
type Bridge struct {
	Ctx  log.Interface
	mqtt *MQTT
}
