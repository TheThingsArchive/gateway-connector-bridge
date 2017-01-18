// Copyright © 2016 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package dummy

import (
	"bytes"
	"testing"
	"time"

	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
	"github.com/apex/log"
	"github.com/apex/log/handlers/text"
	. "github.com/smartystreets/goconvey/convey"
)

func TestDummy(t *testing.T) {
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

		Convey("When creating a new Dummy", func() {
			dummy := New(ctx)

			Convey("When calling Connect on Dummy", func() {
				err := dummy.Connect()
				Convey("There should be no error", func() {
					So(err, ShouldBeNil)
				})
				Convey("We can also call Disconnect", func() {
					dummy.Disconnect()
				})

				Convey("When subscribing to gateway connections", func() {
					connect, err := dummy.SubscribeConnect()
					Convey("There should be no error", func() {
						So(err, ShouldBeNil)
					})
					Convey("When publishing a gateway connection", func() {
						dummy.PublishConnect(&types.ConnectMessage{
							GatewayID: "dev",
							Key:       "key",
						})
						Convey("There should be a corresponding ConnectMessage in the channel", func() {
							select {
							case <-time.After(time.Second):
								So("Timeout Exceeded", ShouldBeFalse)
							case msg := <-connect:
								So(msg.GatewayID, ShouldEqual, "dev")
								So(msg.Key, ShouldEqual, "key")
							}
						})
					})
					Convey("When unsubscribing from gateway connections", func() {
						err := dummy.UnsubscribeConnect()
						Convey("There should be no error", func() {
							So(err, ShouldBeNil)
						})
						Convey("The channel should be closed", func() {
							for range connect {
							}
						})
					})
				})

				Convey("When subscribing to gateway disconnections", func() {
					disconnect, err := dummy.SubscribeDisconnect()
					Convey("There should be no error", func() {
						So(err, ShouldBeNil)
					})
					Convey("When publishing a gateway disconnection", func() {
						dummy.PublishDisconnect(&types.DisconnectMessage{
							GatewayID: "dev",
							Key:       "key",
						})
						Convey("There should be a corresponding ConnectMessage in the channel", func() {
							select {
							case <-time.After(time.Second):
								So("Timeout Exceeded", ShouldBeFalse)
							case msg := <-disconnect:
								So(msg.GatewayID, ShouldEqual, "dev")
								So(msg.Key, ShouldEqual, "key")
							}
						})
					})
					Convey("When unsubscribing from gateway disconnections", func() {
						err := dummy.UnsubscribeDisconnect()
						Convey("There should be no error", func() {
							So(err, ShouldBeNil)
						})
						Convey("The channel should be closed", func() {
							for range disconnect {
							}
						})
					})
				})

				Convey("When subscribing to gateway uplink", func() {
					uplink, err := dummy.SubscribeUplink("dev")
					Convey("There should be no error", func() {
						So(err, ShouldBeNil)
					})
					Convey("When publishing a gateway uplink", func() {
						dummy.PublishUplink(&types.UplinkMessage{GatewayID: "dev"})
						Convey("There should be a corresponding UplinkMessage in the channel", func() {
							select {
							case <-time.After(time.Second):
								So("Timeout Exceeded", ShouldBeFalse)
							case _, ok := <-uplink:
								So(ok, ShouldBeTrue)
							}
						})
					})
					Convey("When unsubscribing from gateway uplink", func() {
						err := dummy.UnsubscribeUplink("dev")
						Convey("There should be no error", func() {
							So(err, ShouldBeNil)
						})
						Convey("The channel should be closed", func() {
							for range uplink {
							}
						})
					})
				})

				Convey("When subscribing to gateway status", func() {
					status, err := dummy.SubscribeStatus("dev")
					Convey("There should be no error", func() {
						So(err, ShouldBeNil)
					})
					Convey("When publishing a gateway status", func() {
						dummy.PublishStatus(&types.StatusMessage{GatewayID: "dev"})
						Convey("There should be a corresponding StatusMessage in the channel", func() {
							select {
							case <-time.After(time.Second):
								So("Timeout Exceeded", ShouldBeFalse)
							case _, ok := <-status:
								So(ok, ShouldBeTrue)
							}
						})
					})
					Convey("When unsubscribing from gateway status", func() {
						err := dummy.UnsubscribeStatus("dev")
						Convey("There should be no error", func() {
							So(err, ShouldBeNil)
						})
						Convey("The channel should be closed", func() {
							select {
							case <-time.After(time.Second):
								So("Timeout Exceeded", ShouldBeFalse)
							case _, ok := <-status:
								So(ok, ShouldBeFalse)
							}
						})
					})
				})

				Convey("When subscribing to gateway downlink", func() {
					downlink, err := dummy.SubscribeDownlink("dev")
					Convey("There should be no error", func() {
						So(err, ShouldBeNil)
					})
					Convey("When publishing a gateway downlink", func() {
						dummy.PublishDownlink(&types.DownlinkMessage{GatewayID: "dev"})
						Convey("There should be a corresponding DownlinkMessage in the channel", func() {
							select {
							case <-time.After(time.Second):
								So("Timeout Exceeded", ShouldBeFalse)
							case _, ok := <-downlink:
								So(ok, ShouldBeTrue)
							}
						})
					})
					Convey("When unsubscribing from gateway downlink", func() {
						err := dummy.UnsubscribeDownlink("dev")
						Convey("There should be no error", func() {
							So(err, ShouldBeNil)
						})
						Convey("The channel should be closed", func() {
							for range downlink {
							}
						})
					})
				})

			})

		})
	})
}
