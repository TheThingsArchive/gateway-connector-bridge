// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package deduplicate

import (
	"testing"

	"github.com/TheThingsNetwork/gateway-connector-bridge/middleware"
	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
	"github.com/TheThingsNetwork/ttn/api/router"
	. "github.com/smartystreets/goconvey/convey"
)

func TestDeduplicate(t *testing.T) {
	Convey("Given a new Deduplicate", t, func(c C) {
		i := NewDeduplicate()

		up := &types.UplinkMessage{GatewayID: "test", Message: &router.UplinkMessage{
			Payload: []byte{1, 2, 3, 4},
		}}
		upDup := &types.UplinkMessage{GatewayID: "test", Message: &router.UplinkMessage{
			Payload: []byte{1, 2, 3, 4},
		}}
		nextUp := &types.UplinkMessage{GatewayID: "test", Message: &router.UplinkMessage{
			Payload: []byte{1, 2, 3, 4, 5},
		}}

		Convey("When sending an UplinkMessage", func() {
			Reset(func() {
				i.HandleDisconnect(middleware.NewContext(), &types.DisconnectMessage{GatewayID: "test"})
			})
			err := i.HandleUplink(middleware.NewContext(), up)
			Convey("There should be no error", func() {
				So(err, ShouldBeNil)
			})
			Convey("When sending a duplicate of that UplinkMessage", func() {
				err := i.HandleUplink(middleware.NewContext(), upDup)
				Convey("There should be an error", func() {
					So(err, ShouldEqual, ErrDuplicateMessage)
				})
			})
			Convey("When sending another UplinkMessage", func() {
				err := i.HandleUplink(middleware.NewContext(), nextUp)
				Convey("There should be no error", func() {
					So(err, ShouldBeNil)
				})
			})

		})
	})
}
