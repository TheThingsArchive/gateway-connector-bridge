// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package lorafilter

import (
	"testing"

	"github.com/TheThingsNetwork/api/router"
	"github.com/TheThingsNetwork/gateway-connector-bridge/middleware"
	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
	. "github.com/smartystreets/goconvey/convey"
)

func TestLoraFilter(t *testing.T) {
	Convey("Given a new Filter", t, func(c C) {
		f := NewFilter()

		Convey("When sending an UplinkMessage without payload", func() {
			err := f.HandleUplink(middleware.NewContext(), &types.UplinkMessage{Message: &router.UplinkMessage{
				Payload: []byte{},
			}})
			Convey("There should be an error", func() {
				So(err, ShouldNotBeNil)
			})
		})

		Convey("When sending an UplinkMessage with an invalid LoRaWAN Major version", func() {
			err := f.HandleUplink(middleware.NewContext(), &types.UplinkMessage{Message: &router.UplinkMessage{
				Payload: []byte{1, 2, 3, 4, 5},
			}})
			Convey("There should be an error", func() {
				So(err, ShouldNotBeNil)
			})
		})

		Convey("When sending an UplinkMessage with a JoinAccept payload", func() {
			err := f.HandleUplink(middleware.NewContext(), &types.UplinkMessage{Message: &router.UplinkMessage{
				Payload: []byte{0x20, 2, 3, 4, 5},
			}})
			Convey("There should be an error", func() {
				So(err, ShouldNotBeNil)
			})
		})

		Convey("When sending an UplinkMessage with a Downlink payload", func() {
			err := f.HandleUplink(middleware.NewContext(), &types.UplinkMessage{Message: &router.UplinkMessage{
				Payload: []byte{0x60, 2, 3, 4, 5},
			}})
			Convey("There should be an error", func() {
				So(err, ShouldNotBeNil)
			})
		})

		Convey("When sending an UplinkMessage with a valid message type", func() {
			err := f.HandleUplink(middleware.NewContext(), &types.UplinkMessage{Message: &router.UplinkMessage{
				Payload: []byte{0x40, 2, 3, 4, 5},
			}})
			Convey("There should be no error", func() {
				So(err, ShouldBeNil)
			})
		})
	})
}
