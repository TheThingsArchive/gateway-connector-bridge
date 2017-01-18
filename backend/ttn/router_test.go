// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package ttn

import (
	"bytes"
	"testing"

	"time"

	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
	pb_gateway "github.com/TheThingsNetwork/ttn/api/gateway"
	pb_router "github.com/TheThingsNetwork/ttn/api/router"
	"github.com/apex/log"
	"github.com/apex/log/handlers/text"
	. "github.com/smartystreets/goconvey/convey"
)

func devToken(string) string {
	return "eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9.eyJpc3MiOiJ0dG4tYWNjb3VudC1wcmV2aWV3Iiwic3ViIjoiZGV2IiwidHlwZSI6ImdhdGV3YXkiLCJpYXQiOjE0NzY0Mzk0Mzh9.kEOiLe9j4qRElZOt_bAXmZlva1nV6duIL0MDVa3bx2SEWC3qredaBWXWq4FmV4PKeI_zndovQtOoValH0B_6MW6vXuWL1wYzV6foTH5gQdxmn-iuQ1AmAIYbZeyHl9a-NPqDgkXLwKmo2iB1hUi9wV6HXfIOalguDtGJbmMfJ2tommsgmuNCXd-2zqhStSy8ArpROFXPm7voGDTcgm4hfchr7zhn-Er76R-eJa3RZ1Seo9BsiWrQ0N3VDSuh7ycCakZtkaLD4OTutAemcbzbrNJSOCvvZr8Asn-RmMkjKUdTN4Bgn3qlacIQ9iZikPLT8XyjFkj-8xjs3KAobWg40A"
}

func TestTTNRouter(t *testing.T) {
	Convey("Given a new Context", t, func(c C) {

		var logs bytes.Buffer
		ctx := &log.Logger{
			Handler: text.New(&logs),
			Level:   log.DebugLevel,
		}
		defer func() {
			if logs.Len() > 0 {
				c.Printf("\n%s", logs.String())
			}
		}()

		Convey("When creating a new TTN Router", func() {
			router, err := New(RouterConfig{
				DiscoveryServer: "discovery.thethingsnetwork.org:1900",
				RouterID:        "ttn-router-eu",
			}, ctx, devToken)
			Convey("There should be no error", func() {
				So(err, ShouldBeNil)
			})
			Convey("The bridge should now have TTNRouter", func() {
				So(router, ShouldNotBeNil)
			})

			Convey("When calling Connect on TTNRouter", func() {
				err := router.Connect()
				Convey("There should be no error", func() {
					So(err, ShouldBeNil)
				})
				Convey("We can also call Disconnect", func() {
					router.Disconnect()
				})

				Convey("When publishing an uplink message", func() {
					router.PublishUplink(&types.UplinkMessage{
						GatewayID: "dev",
						Message:   &pb_router.UplinkMessage{},
					})
					Convey("There should be no error", func() {
						So(err, ShouldBeNil)
					})
					router.CleanupGateway("dev")
				})

				Convey("When publishing a status message", func() {
					router.PublishStatus(&types.StatusMessage{
						GatewayID: "dev",
						Message:   &pb_gateway.Status{},
					})
					Convey("There should be no error", func() {
						So(err, ShouldBeNil)
					})
					router.CleanupGateway("dev")
				})

				Convey("When subscribing to downlink messages", func() {
					ch, err := router.SubscribeDownlink("dev")
					Convey("There should be no error", func() {
						So(err, ShouldBeNil)
					})
					time.Sleep(100 * time.Millisecond)
					Reset(func() {
						router.UnsubscribeDownlink("dev")
						time.Sleep(100 * time.Millisecond)
					})

					Convey("When unsubscribing from downlink messages", func() {
						err := router.UnsubscribeDownlink("dev")
						Convey("There should be no error", func() {
							So(err, ShouldBeNil)
						})
						Convey("The channel should be closed", func() {
							select {
							case <-time.After(time.Second):
								So("Timeout Exceeded", ShouldBeFalse)
							case _, ok := <-ch:
								So(ok, ShouldBeFalse)
							}
						})
						router.CleanupGateway("dev")
					})
					router.CleanupGateway("dev")
				})
			})
		})
	})
}
