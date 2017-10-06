// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package inject

import (
	"testing"

	"github.com/TheThingsNetwork/api/gateway"
	"github.com/TheThingsNetwork/gateway-connector-bridge/middleware"
	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
	. "github.com/smartystreets/goconvey/convey"
)

func TestInject(t *testing.T) {
	Convey("Given a new Inject", t, func(c C) {
		i := NewInject(Fields{
			FrequencyPlan: "EU_868",
			Bridge:        "bridge",
		})

		Convey("When sending a StatusMessage", func() {
			status := &types.StatusMessage{
				Message: &gateway.Status{},
			}
			err := i.HandleStatus(middleware.NewContext(), status)
			Convey("There should be no error", func() {
				So(err, ShouldBeNil)
			})
			Convey("The StatusMessage should contain the injected fields", func() {
				So(status.Message.FrequencyPlan, ShouldEqual, "EU_868")
				So(status.Message.Bridge, ShouldEqual, "bridge")
			})
		})
	})
}
