// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package statusserver

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/TheThingsNetwork/gateway-connector-bridge/status"
	metrics "github.com/rcrowley/go-metrics"
	. "github.com/smartystreets/goconvey/convey"
	"google.golang.org/grpc"
)

func TestStatusServer(t *testing.T) {
	Convey("Given an empty default StatusServer", t, func(c C) {

		global = &statusServer{
			uplink:            metrics.NewMeter(),
			downlink:          metrics.NewMeter(),
			gatewayStatus:     metrics.NewMeter(),
			connectedGateways: metrics.NewCounter(),
		}

		rand.Seed(time.Now().UnixNano())
		port := rand.Intn(1000) + 10000

		// Set up the status server
		lis, err := net.Listen("tcp", fmt.Sprint(":", port))
		if err != nil {
			panic("Could not start StatusServer")
		}
		srv := grpc.NewServer()
		Register(srv)
		go srv.Serve(lis)

		Convey("Given a StatusClient", func() {
			conn, err := grpc.Dial(fmt.Sprint("localhost:", port), grpc.WithBlock(), grpc.WithInsecure())
			if err != nil {
				panic(err)
			}
			cli := status.NewStatusClient(conn)
			Convey("When requesting the status", func() {
				res, err := cli.GetStatus(context.Background(), &status.StatusRequest{})
				Convey("There should be no error", func() {
					So(err, ShouldBeNil)
				})
				Convey("Rates should be empty", func() {
					So(res.Uplink.Rate1, ShouldEqual, 0)
					So(res.Downlink.Rate1, ShouldEqual, 0)
					So(res.GatewayStatus.Rate1, ShouldEqual, 0)
					So(res.ConnectedGateways, ShouldEqual, 0)
				})
			})
		})

		Convey("When calling Uplink", func() {
			Uplink()
			Convey("Then the Count should have increased", func() {
				So(global.uplink.Count(), ShouldEqual, 1)
			})
		})

		Convey("When calling Downlink", func() {
			Downlink()
			Convey("Then the Count should have increased", func() {
				So(global.downlink.Count(), ShouldEqual, 1)
			})
		})

		Convey("When calling GatewayStatus", func() {
			GatewayStatus()
			Convey("Then the Count should have increased", func() {
				So(global.gatewayStatus.Count(), ShouldEqual, 1)
			})
		})

		Convey("When calling ConnectGateway", func() {
			ConnectGateway()
			Convey("Then the Count should have increased", func() {
				So(global.connectedGateways.Count(), ShouldEqual, 1)
			})
		})

		Convey("When calling DisconnectGateway", func() {
			DisconnectGateway()
			Convey("Then the Count should have decreased", func() {
				So(global.connectedGateways.Count(), ShouldEqual, -1)
			})
		})
	})
}
