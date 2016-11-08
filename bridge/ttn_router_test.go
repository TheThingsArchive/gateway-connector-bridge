// Copyright © 2016 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package bridge

import (
	"bytes"
	"testing"

	"github.com/TheThingsNetwork/ttn/api/gateway"
	"github.com/TheThingsNetwork/ttn/api/router"
	"github.com/apex/log"
	"github.com/apex/log/handlers/text"
	. "github.com/smartystreets/goconvey/convey"
)

var devToken = "eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9.eyJpc3MiOiJ0dG4tYWNjb3VudC1wcmV2aWV3Iiwic3ViIjoiZGV2IiwidHlwZSI6ImdhdGV3YXkiLCJpYXQiOjE0NzY0Mzk0Mzh9.kEOiLe9j4qRElZOt_bAXmZlva1nV6duIL0MDVa3bx2SEWC3qredaBWXWq4FmV4PKeI_zndovQtOoValH0B_6MW6vXuWL1wYzV6foTH5gQdxmn-iuQ1AmAIYbZeyHl9a-NPqDgkXLwKmo2iB1hUi9wV6HXfIOalguDtGJbmMfJ2tommsgmuNCXd-2zqhStSy8ArpROFXPm7voGDTcgm4hfchr7zhn-Er76R-eJa3RZ1Seo9BsiWrQ0N3VDSuh7ycCakZtkaLD4OTutAemcbzbrNJSOCvvZr8Asn-RmMkjKUdTN4Bgn3qlacIQ9iZikPLT8XyjFkj-8xjs3KAobWg40A"

func TestTTNRouter(t *testing.T) {
	Convey("Given a new Bridge", t, func(c C) {
		b := new(Bridge)

		b.tokens = map[string]string{
			"dev": devToken,
		}

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

		Convey("When calling SetupTTNRouter", func() {
			err := b.SetupTTNRouter(TTNRouterConfig{
				DiscoveryServer: "discovery.thethingsnetwork.org:1900",
				RouterID:        "ttn-router-eu",
			})
			Convey("There should be no error", func() {
				So(err, ShouldBeNil)
			})
			Convey("The bridge should now have TTNRouter", func() {
				So(b.ttnRouter, ShouldNotBeNil)
			})

			Convey("When calling Connect on TTNRouter", func() {
				err := b.ttnRouter.Connect()
				Convey("There should be no error", func() {
					So(err, ShouldBeNil)
				})
				Convey("We can also call Disconnect", func() {
					b.ttnRouter.Disconnect()
				})

				Convey("When publishing an uplink message", func() {
					b.ttnRouter.PublishUplink(&UplinkMessage{
						gatewayID: "dev",
						message:   &router.UplinkMessage{},
					})
					Convey("There should be no error", func() {
						So(err, ShouldBeNil)
					})
					b.ttnRouter.CleanupGateway("dev")
				})

				Convey("When publishing a status message", func() {
					b.ttnRouter.PublishStatus(&StatusMessage{
						gatewayID: "dev",
						message:   &gateway.Status{},
					})
					Convey("There should be no error", func() {
						So(err, ShouldBeNil)
					})
					b.ttnRouter.CleanupGateway("dev")
				})
			})
		})
	})
}
