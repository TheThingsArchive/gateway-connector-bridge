// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package ratelimit

import (
	"testing"

	"github.com/TheThingsNetwork/gateway-connector-bridge/middleware"
	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
	. "github.com/smartystreets/goconvey/convey"
)

func TestRateLimit(t *testing.T) {
	Convey("Given a new RateLimit", t, func(c C) {
		i := NewRateLimit(Limits{
			Uplink:   1,
			Downlink: 1,
			Status:   1,
		})

		Convey("When sending a ConnectMessage", func() {

			err := i.HandleConnect(middleware.NewContext(), &types.ConnectMessage{GatewayID: "test"})
			Convey("There should be no error", func() {
				So(err, ShouldBeNil)
			})
			Convey("The gateway limits should have been initialized", func() {
				So(i.gateways, ShouldContainKey, "test")
			})

			Convey("When sending an UplinkMessage", func() {
				err := i.HandleUplink(middleware.NewContext(), &types.UplinkMessage{GatewayID: "test"})
				Convey("There should be no error", func() {
					So(err, ShouldBeNil)
				})
				Convey("When sending another UplinkMessage", func() {
					err := i.HandleUplink(middleware.NewContext(), &types.UplinkMessage{GatewayID: "test"})
					Convey("There should be an error", func() {
						So(err, ShouldEqual, ErrRateLimited)
					})
				})
			})

			Convey("When sending a DownlinkMessage", func() {
				err := i.HandleDownlink(middleware.NewContext(), &types.DownlinkMessage{GatewayID: "test"})
				Convey("There should be no error", func() {
					So(err, ShouldBeNil)
				})
				Convey("When sending another DownlinkMessage", func() {
					err := i.HandleDownlink(middleware.NewContext(), &types.DownlinkMessage{GatewayID: "test"})
					Convey("There should be an error", func() {
						So(err, ShouldEqual, ErrRateLimited)
					})
				})
			})

			Convey("When sending a StatusMessage", func() {
				err := i.HandleStatus(middleware.NewContext(), &types.StatusMessage{GatewayID: "test"})
				Convey("There should be no error", func() {
					So(err, ShouldBeNil)
				})
				Convey("When sending a StatusMessage", func() {
					err := i.HandleStatus(middleware.NewContext(), &types.StatusMessage{GatewayID: "test"})
					Convey("There should be an error", func() {
						So(err, ShouldEqual, ErrRateLimited)
					})
				})
			})

			Convey("When sending a DisconnectMessage", func() {
				err := i.HandleDisconnect(middleware.NewContext(), &types.DisconnectMessage{GatewayID: "test"})
				Convey("There should be no error", func() {
					So(err, ShouldBeNil)
				})
				Convey("The gateway limits should have been unset", func() {
					So(i.gateways, ShouldNotContainKey, "test")
				})
			})

		})

	})
}
