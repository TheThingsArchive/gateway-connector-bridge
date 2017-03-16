// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package middleware

import (
	"errors"
	"testing"

	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
	. "github.com/smartystreets/goconvey/convey"
)

type something struct{}

func TestContext(t *testing.T) {
	some := new(something)

	Convey("Given a new Context", t, func(c C) {
		ctx := NewContext()
		Convey("When setting some items in the Context", func() {
			ctx.Set(1, 2)
			ctx.Set("str", "hi")
			ctx.Set(some, some)
			Convey("Then getting those items from the Context should return the items", func() {
				So(ctx.Get(1), ShouldEqual, 2)
				So(ctx.Get("str"), ShouldEqual, "hi")
				So(ctx.Get(some), ShouldEqual, some)
			})
		})
		Convey("Getting an item that is not in the context, returns nil", func() {
			So(ctx.Get("other"), ShouldBeNil)
		})
	})
}

type testMiddleware struct {
	err error

	connect    int
	disconnect int
	uplink     int
	status     int
	downlink   int
}

func (c *testMiddleware) HandleConnect(ctx Context, msg *types.ConnectMessage) error {
	c.connect++
	return c.err
}
func (c *testMiddleware) HandleDisconnect(ctx Context, msg *types.DisconnectMessage) error {
	c.disconnect++
	return c.err
}
func (c *testMiddleware) HandleUplink(ctx Context, msg *types.UplinkMessage) error {
	c.uplink++
	return c.err
}
func (c *testMiddleware) HandleStatus(ctx Context, msg *types.StatusMessage) error {
	c.status++
	return c.err
}
func (c *testMiddleware) HandleDownlink(ctx Context, msg *types.DownlinkMessage) error {
	c.downlink++
	return c.err
}

func TestMiddleware(t *testing.T) {
	Convey("Given a new Middleware Chain", t, func(c C) {
		m := new(testMiddleware)
		chain := Chain{m}

		Convey("When executing it on a ConnectMessage", func() {
			err := chain.Execute(NewContext(), &types.ConnectMessage{})
			Convey("There should be no error", func() {
				So(err, ShouldBeNil)
			})
			Convey("The middleware should have been called", func() {
				So(m.connect, ShouldEqual, 1)
			})
		})

		Convey("When executing it on a DisconnectMessage", func() {
			err := chain.Execute(NewContext(), &types.DisconnectMessage{})
			Convey("There should be no error", func() {
				So(err, ShouldBeNil)
			})
			Convey("The middleware should have been called", func() {
				So(m.disconnect, ShouldEqual, 1)
			})
		})

		Convey("When executing it on a UplinkMessage", func() {
			err := chain.Execute(NewContext(), &types.UplinkMessage{})
			Convey("There should be no error", func() {
				So(err, ShouldBeNil)
			})
			Convey("The middleware should have been called", func() {
				So(m.uplink, ShouldEqual, 1)
			})
		})

		Convey("When executing it on a StatusMessage", func() {
			err := chain.Execute(NewContext(), &types.StatusMessage{})
			Convey("There should be no error", func() {
				So(err, ShouldBeNil)
			})
			Convey("The middleware should have been called", func() {
				So(m.status, ShouldEqual, 1)
			})
		})

		Convey("When executing it on a DownlinkMessage", func() {
			err := chain.Execute(NewContext(), &types.DownlinkMessage{})
			Convey("There should be no error", func() {
				So(err, ShouldBeNil)
			})
			Convey("The middleware should have been called", func() {
				So(m.downlink, ShouldEqual, 1)
			})
		})

		Convey("When the middleware would return an error", func() {
			m.err = errors.New("some error")

			Convey("When executing it on a ConnectMessage", func() {
				err := chain.Execute(NewContext(), &types.ConnectMessage{})
				Convey("There should be an error in the chain", func() {
					So(err, ShouldNotBeNil)
				})
			})

			Convey("When executing it on a DisconnectMessage", func() {
				err := chain.Execute(NewContext(), &types.DisconnectMessage{})
				Convey("There should be an error in the chain", func() {
					So(err, ShouldNotBeNil)
				})
			})

			Convey("When executing it on a UplinkMessage", func() {
				err := chain.Execute(NewContext(), &types.UplinkMessage{})
				Convey("There should be an error in the chain", func() {
					So(err, ShouldNotBeNil)
				})
			})

			Convey("When executing it on a StatusMessage", func() {
				err := chain.Execute(NewContext(), &types.StatusMessage{})
				Convey("There should be an error in the chain", func() {
					So(err, ShouldNotBeNil)
				})
			})

			Convey("When executing it on a DownlinkMessage", func() {
				err := chain.Execute(NewContext(), &types.DownlinkMessage{})
				Convey("There should be an error in the chain", func() {
					So(err, ShouldNotBeNil)
				})
			})

		})

		Convey("When executing it on any other type", func() {
			err := chain.Execute(NewContext(), "hello")
			Convey("There should be no error", func() {
				So(err, ShouldBeNil)
			})
			Convey("The middleware should not have been called", func() {
				So(m.connect, ShouldEqual, 0)
				So(m.disconnect, ShouldEqual, 0)
				So(m.uplink, ShouldEqual, 0)
				So(m.status, ShouldEqual, 0)
				So(m.downlink, ShouldEqual, 0)
			})
		})

	})
}
