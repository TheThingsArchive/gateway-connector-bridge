// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package exchange

import (
	"time"
)

type watchdog struct {
	*time.Timer
}

const watchdogExpire = time.Second

func newWatchdog(callback func()) *watchdog {
	return &watchdog{
		Timer: time.AfterFunc(watchdogExpire, callback),
	}
}

// Kick the watchdog. No effect if already expired
func (w *watchdog) Kick() {
	if w.Stop() {
		w.Reset(watchdogExpire)
	}
}
