// Copyright © 2016 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package bridge

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"time"

	"github.com/TheThingsNetwork/ttn/api/gateway"
	"github.com/TheThingsNetwork/ttn/api/router"
	"github.com/apex/log"
	"github.com/apex/log/handlers/text"
	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/gogo/protobuf/proto"
	. "github.com/smartystreets/goconvey/convey"
)

var host string

func init() {
	host = os.Getenv("MQTT_ADDRESS")
	if host == "" {
		host = "localhost:1883"
	}
}

func TestSetupMQTT(t *testing.T) {
	Convey("Given a new Bridge", t, func(c C) {
		b := new(Bridge)

		var logs bytes.Buffer
		b.Ctx = &log.Logger{
			Handler: text.New(&logs),
			Level:   log.DebugLevel,
		}
		defer func() {
			if logs.Len() > 0 {
				c.Printf("\n%s", logs.String())
			}
		}()

		Convey("When calling SetupMQTT", func() {
			err := b.SetupMQTT(MQTTConfig{
				Brokers: []string{fmt.Sprintf("tcp://%s", host)},
			})
			Convey("There should be no error", func() {
				So(err, ShouldBeNil)
			})
			Convey("The bridge should now have MQTT", func() {
				So(b.mqtt, ShouldNotBeNil)
			})

			Convey("When calling Connect on MQTT", func() {
				err := b.mqtt.Connect()
				Convey("There should be no error", func() {
					So(err, ShouldBeNil)
				})
				Convey("We can also call Disconnect", func() {
					b.mqtt.Disconnect()
				})

				Convey("When subscribing to gateway connections", func() {
					connect, err := b.mqtt.SubscribeConnect()
					Convey("There should be no error", func() {
						So(err, ShouldBeNil)
					})
					Convey("When publishing a gateway connection", func() {
						b.mqtt.publish(ConnectTopicFormat, []byte(`{"id":"dev","token":"token"}`))
						Convey("There should be a corresponding ConnectMessage in the channel", func() {
							msg := <-connect
							So(msg.GatewayID, ShouldEqual, "dev")
							So(msg.Token, ShouldEqual, "token")
						})
					})
					Convey("When unsubscribing from gateway connections", func() {
						err := b.mqtt.UnsubscribeConnect()
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
					disconnect, err := b.mqtt.SubscribeDisconnect()
					Convey("There should be no error", func() {
						So(err, ShouldBeNil)
					})
					Convey("When publishing a gateway disconnection", func() {
						b.mqtt.publish(DisconnectTopicFormat, []byte(`{"id":"dev"}`))
						Convey("There should be a corresponding ConnectMessage in the channel", func() {
							msg := <-disconnect
							So(msg.GatewayID, ShouldEqual, "dev")
						})
					})
					Convey("When unsubscribing from gateway disconnections", func() {
						err := b.mqtt.UnsubscribeDisconnect()
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
					uplink, err := b.mqtt.SubscribeUplink("dev")
					Convey("There should be no error", func() {
						So(err, ShouldBeNil)
					})
					Convey("When publishing a gateway uplink", func() {
						uplinkMessage := new(router.UplinkMessage)
						uplinkMessage.Payload = []byte{1, 2, 3, 4}
						bin, _ := proto.Marshal(uplinkMessage)
						b.mqtt.publish(fmt.Sprintf(UplinkTopicFormat, "dev"), bin)
						Convey("There should be a corresponding UplinkMessage in the channel", func() {
							msg := <-uplink
							So(msg.message.Payload, ShouldResemble, []byte{1, 2, 3, 4})
						})
					})
					Convey("When unsubscribing from gateway uplink", func() {
						err := b.mqtt.UnsubscribeUplink("dev")
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
					status, err := b.mqtt.SubscribeStatus("dev")
					Convey("There should be no error", func() {
						So(err, ShouldBeNil)
					})
					Convey("When publishing a gateway status", func() {
						statusMessage := new(gateway.Status)
						statusMessage.Description = "Awesome Description"
						bin, _ := proto.Marshal(statusMessage)
						b.mqtt.publish(fmt.Sprintf(StatusTopicFormat, "dev"), bin)
						Convey("There should be a corresponding StatusMessage in the channel", func() {
							msg := <-status
							So(msg.message.Description, ShouldEqual, "Awesome Description")
						})
					})
					Convey("When unsubscribing from gateway status", func() {
						err := b.mqtt.UnsubscribeStatus("dev")
						Convey("There should be no error", func() {
							So(err, ShouldBeNil)
						})
						Convey("The channel should be closed", func() {
							for range status {
							}
						})
					})
				})

				Convey("When subscribing to gateway downlink", func() {
					var payload []byte
					b.mqtt.subscribe(fmt.Sprintf(DownlinkTopicFormat, "dev"), func(_ paho.Client, msg paho.Message) {
						payload = msg.Payload()
					}, func() {})

					Convey("When publishing a downlink message", func() {
						downlinkMessage := new(router.DownlinkMessage)
						downlinkMessage.Payload = []byte{1, 2, 3, 4}
						err := b.mqtt.PublishDownlink(&DownlinkMessage{
							gatewayID: "dev",
							message:   downlinkMessage,
						})
						Convey("There should be no error", func() {
							So(err, ShouldBeNil)
						})
						Convey("The payload should be received within 100ms", func() {
							time.Sleep(100 * time.Millisecond)
							So(payload, ShouldNotBeEmpty)
						})
					})
				})

			})
		})

	})
}
