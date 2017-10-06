// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package amqp

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/TheThingsNetwork/api/gateway"
	"github.com/TheThingsNetwork/api/router"
	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
	"github.com/apex/log"
	"github.com/apex/log/handlers/text"
	"github.com/gogo/protobuf/proto"
	. "github.com/smartystreets/goconvey/convey"
)

var host string

func init() {
	host = os.Getenv("AMQP_ADDRESS")
	if host == "" {
		host = "localhost:5672"
	}
}

func TestAMQP(t *testing.T) {
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

		Convey("When creating a new AMQP", func() {
			amqp, err := New(Config{
				Address: host,
			}, ctx)
			Convey("There should be no error", func() {
				So(err, ShouldBeNil)
			})
			Convey("amqp should be there", func() {
				So(amqp, ShouldNotBeNil)
			})

			Convey("When calling Connect", func() {
				amqp.Connect()
				time.Sleep(10 * time.Millisecond)

				Reset(func() { amqp.Disconnect() })

				Convey("When calling Disconnect", func() {
					amqp.Disconnect()
					time.Sleep(10 * time.Millisecond)
				})

				Convey("When getting a channel", func() {
					ch, err := amqp.channel()
					Convey("There should be no error", func() {
						So(err, ShouldBeNil)
					})
					ch.Close()
				})

				Convey("When subscribing to a routing key", func() {
					msg, err := amqp.subscribe("some-key")
					time.Sleep(10 * time.Millisecond)
					Reset(func() {
						amqp.unsubscribe("some-key")
					})

					Convey("There should be no error", func() {
						So(err, ShouldBeNil)
					})

					Convey("When publishing a message", func() {
						err := amqp.Publish("some-key", []byte{1, 2, 3, 4})
						Convey("There should be no error", func() {
							So(err, ShouldBeNil)
						})
						Convey("The subscribe chan should receive a message", func() {
							select {
							case <-time.After(time.Second):
								So("Timeout Exceeded", ShouldBeFalse)
							case msg, ok := <-msg:
								So(ok, ShouldBeTrue)
								So(msg, ShouldNotBeNil)
							}
						})
					})

					Convey("When unsubscribing from the routing key", func() {
						err := amqp.unsubscribe("some-key")
						Convey("There should be no error", func() {
							So(err, ShouldBeNil)
						})
						Convey("The channel should be closed", func() {
							select {
							case <-time.After(time.Second):
								So("Timeout Exceeded", ShouldBeFalse)
							case _, ok := <-msg:
								So(ok, ShouldBeFalse)
							}
						})
					})
				})

				Convey("When subscribing to connect messages", func() {
					msg, err := amqp.SubscribeConnect()
					time.Sleep(10 * time.Millisecond)
					Reset(func() {
						amqp.UnsubscribeConnect()
					})
					Convey("There should be no error", func() {
						So(err, ShouldBeNil)
					})
					Convey("When publishing a connect message", func() {
						bytes, _ := (&types.ConnectMessage{GatewayID: "dev", Key: "key"}).Marshal()
						err := amqp.Publish(ConnectRoutingKeyFormat, bytes)
						Convey("There should be no error", func() {
							So(err, ShouldBeNil)
						})
						Convey("The connect chan should receive a message", func() {
							select {
							case <-time.After(time.Second):
								So("Timeout Exceeded", ShouldBeFalse)
							case msg, ok := <-msg:
								So(ok, ShouldBeTrue)
								So(msg.GatewayID, ShouldEqual, "dev")
								So(msg.Key, ShouldEqual, "key")
							}
						})
					})
				})

				Convey("When subscribing to disconnect messages", func() {
					msg, err := amqp.SubscribeDisconnect()
					time.Sleep(10 * time.Millisecond)
					Reset(func() {
						amqp.UnsubscribeDisconnect()
					})
					Convey("There should be no error", func() {
						So(err, ShouldBeNil)
					})
					Convey("When publishing a disconnect message", func() {
						bytes, _ := (&types.DisconnectMessage{GatewayID: "dev", Key: "key"}).Marshal()
						err := amqp.Publish(DisconnectRoutingKeyFormat, bytes)
						Convey("There should be no error", func() {
							So(err, ShouldBeNil)
						})
						Convey("The disconnect chan should receive a message", func() {
							select {
							case <-time.After(time.Second):
								So("Timeout Exceeded", ShouldBeFalse)
							case msg, ok := <-msg:
								So(ok, ShouldBeTrue)
								So(msg, ShouldNotBeNil)
								So(msg.GatewayID, ShouldEqual, "dev")
								So(msg.Key, ShouldEqual, "key")
							}
						})
					})
				})

				Convey("When subscribing to uplink messages", func() {
					msg, err := amqp.SubscribeUplink("dev")
					time.Sleep(10 * time.Millisecond)
					Reset(func() {
						amqp.UnsubscribeUplink("dev")
					})
					Convey("There should be no error", func() {
						So(err, ShouldBeNil)
					})
					Convey("When publishing a uplink message", func() {
						bytes, _ := proto.Marshal(new(router.UplinkMessage))
						err := amqp.Publish(fmt.Sprintf(UplinkRoutingKeyFormat, "dev"), bytes)
						Convey("There should be no error", func() {
							So(err, ShouldBeNil)
						})
						Convey("The uplink chan should receive a message", func() {
							select {
							case <-time.After(time.Second):
								So("Timeout Exceeded", ShouldBeFalse)
							case msg, ok := <-msg:
								So(ok, ShouldBeTrue)
								So(msg, ShouldNotBeNil)
							}
						})
					})
				})

				Convey("When subscribing to status messages", func() {
					msg, err := amqp.SubscribeStatus("dev")
					time.Sleep(10 * time.Millisecond)
					Reset(func() {
						amqp.UnsubscribeStatus("dev")
					})
					Convey("There should be no error", func() {
						So(err, ShouldBeNil)
					})
					Convey("When publishing a status message", func() {
						bytes, _ := proto.Marshal(new(gateway.Status))
						err := amqp.Publish(fmt.Sprintf(StatusRoutingKeyFormat, "dev"), bytes)
						Convey("There should be no error", func() {
							So(err, ShouldBeNil)
						})
						Convey("The status chan should receive a message", func() {
							select {
							case <-time.After(time.Second):
								So("Timeout Exceeded", ShouldBeFalse)
							case msg, ok := <-msg:
								So(ok, ShouldBeTrue)
								So(msg, ShouldNotBeNil)
							}
						})
					})
				})

				Convey("When subscribing to downlink messages key", func() {
					msg, err := amqp.subscribe(fmt.Sprintf(DownlinkRoutingKeyFormat, "dev"))
					time.Sleep(10 * time.Millisecond)
					Reset(func() {
						amqp.unsubscribe(fmt.Sprintf(DownlinkRoutingKeyFormat, "dev"))
					})

					Convey("There should be no error", func() {
						So(err, ShouldBeNil)
					})

					Convey("When publishing a downlink message", func() {
						downlinkMessage := new(router.DownlinkMessage)
						err := amqp.PublishDownlink(&types.DownlinkMessage{
							GatewayID: "dev",
							Message:   downlinkMessage,
						})
						Convey("There should be no error", func() {
							So(err, ShouldBeNil)
						})

						Convey("The downlink chan should receive a message", func() {
							select {
							case <-time.After(time.Second):
								So("Timeout Exceeded", ShouldBeFalse)
							case msg, ok := <-msg:
								So(ok, ShouldBeTrue)
								So(msg, ShouldNotBeNil)
							}
						})
					})
				})

			})
		})
	})
}
