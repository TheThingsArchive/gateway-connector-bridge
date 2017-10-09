// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package ttn

import (
	"bytes"
	"testing"
	"time"

	pb_gateway "github.com/TheThingsNetwork/api/gateway"
	pb_router "github.com/TheThingsNetwork/api/router"
	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
	ttnlog "github.com/TheThingsNetwork/go-utils/log"
	"github.com/TheThingsNetwork/go-utils/log/apex"
	"github.com/TheThingsNetwork/ttn/api"
	"github.com/TheThingsNetwork/ttn/api/pool"
	"github.com/apex/log"
	"github.com/apex/log/handlers/text"
	. "github.com/smartystreets/goconvey/convey"
)

func TestTTNRouter(t *testing.T) {
	Convey("Given a new Context", t, func(c C) {

		var logs bytes.Buffer
		ctx := &log.Logger{
			Handler: text.New(&logs),
			Level:   log.DebugLevel,
		}
		ttnlog.Set(apex.Wrap(ctx))
		defer func() {
			if logs.Len() > 0 {
				c.Printf("\n%s", logs.String())
			}
		}()

		api.WaitForStreams = time.Second

		Convey("When creating a new TTN Router", func() {
			router, err := New(RouterConfig{
				DiscoveryServer: "discovery.thethingsnetwork.org:1900",
				RouterID:        "ttn-router-eu",
			}, ctx, func(string) string {
				return "token"
			})
			Convey("There should be no error", func() {
				So(err, ShouldBeNil)
			})
			Convey("The bridge should now have TTNRouter", func() {
				So(router, ShouldNotBeNil)
			})
			Reset(func() {
				pool.Global.Close()
				router.pool.Close()
			})

			Convey("When calling Connect on TTNRouter", func() {
				err := router.Connect()
				Convey("There should be no error", func() {
					So(err, ShouldBeNil)
				})
				Convey("There should be a router client", func() {
					So(router.client, ShouldNotBeNil)
				})

				Convey("When publishing an uplink message", func() {
					router.PublishUplink(&types.UplinkMessage{
						GatewayID: "dev",
						Message:   &pb_router.UplinkMessage{},
					})
					oldStream := router.gateways["dev"].stream
					_, err := router.SubscribeDownlink("dev")
					Convey("There should be no error", func() {
						So(err, ShouldBeNil)
					})
					Convey("The streams should have been replaced", func() {
						So(router.gateways["dev"].stream, ShouldNotEqual, oldStream)
					})
				})

				Convey("When publishing a status message", func() {
					router.PublishStatus(&types.StatusMessage{
						GatewayID: "dev",
						Message:   &pb_gateway.Status{},
					})
				})

				Convey("When subscribing to downlink messages", func() {
					_, err := router.SubscribeDownlink("dev")
					Convey("There should be no error", func() {
						So(err, ShouldBeNil)
					})
				})
			})
		})
	})
}
