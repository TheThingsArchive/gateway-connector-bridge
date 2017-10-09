// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package gatewayinfo

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/TheThingsNetwork/api/gateway"
	"github.com/TheThingsNetwork/gateway-connector-bridge/middleware"
	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
	"github.com/TheThingsNetwork/go-account-lib/account"
	. "github.com/smartystreets/goconvey/convey"
	redis "gopkg.in/redis.v5"
)

func getRedisClient() *redis.Client {
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		host = "localhost"
	}
	return redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:6379", host),
		Password: "", // no password set
		DB:       1,  // use default DB
	})
}

func TestPublic(t *testing.T) {
	Convey("Given a new Public GatewayInfo", t, func(c C) {
		p := NewPublic("https://account.thethingsnetwork.org")
		gatewayID := "eui-0000024b08060112"

		Convey("When fetching the info of a non-existent Gateway", func() {
			err := p.fetch("dev")
			Convey("There should be an error", func() {
				So(err, ShouldNotBeNil)
			})
			gateway, err := p.get("dev")
			Convey("The info should not be stored", func() {
				So(gateway.ID, ShouldBeEmpty)
			})
			Convey("An error should be stored", func() {
				So(err, ShouldNotBeNil)
			})
		})

		Convey("When fetching the info of a Gateway", func() {
			err := p.fetch(gatewayID)
			Convey("There should be no error", func() {
				So(err, ShouldBeNil)
			})
			Convey("The info should be stored", func() {
				gateway, _ := p.get(gatewayID)
				So(gateway.ID, ShouldEqual, gatewayID)
			})
		})

		Convey("When handling ConnectMessages", func() {
			Convey("For a non-existent Gateway", func() {
				err := p.HandleConnect(middleware.NewContext(), &types.ConnectMessage{
					GatewayID: "dev",
				})
				Convey("There should be no error (we don't want to break on this)", func() {
					So(err, ShouldBeNil)
				})
			})

			err := p.HandleConnect(middleware.NewContext(), &types.ConnectMessage{
				GatewayID: gatewayID,
			})
			Convey("There should be no error", func() {
				So(err, ShouldBeNil)
			})

			time.Sleep(500 * time.Millisecond)
			Convey("The info should be stored", func() {
				gateway, _ := p.get(gatewayID)
				So(gateway.ID, ShouldEqual, gatewayID)
			})
		})

		Convey("Given some stored gateway information", func() {
			strptr := func(s string) *string { return &s }

			gatewayID := "gateway-id"
			p.set(gatewayID, account.Gateway{
				ID:            gatewayID,
				FrequencyPlan: "EU_868",
				AntennaLocation: &account.Location{
					Latitude:  12.34,
					Longitude: 56.78,
				},
				Attributes: account.GatewayAttributes{
					Brand:       strptr("Test"),
					Model:       strptr("Gateway"),
					Description: strptr("My Test Gateway"),
				},
			})

			Convey("When sending a DisconnectMessage", func() {
				err := p.HandleDisconnect(middleware.NewContext(), &types.DisconnectMessage{
					GatewayID: gatewayID,
				})
				Convey("There should be no error", func() {
					So(err, ShouldBeNil)
				})
				time.Sleep(10 * time.Millisecond)
				Convey("The info should no longer be stored", func() {
					gateway, _ := p.get(gatewayID)
					So(gateway.ID, ShouldBeEmpty)
				})
			})

			Convey("When sending a StatusMessage", func() {
				status := &types.StatusMessage{
					GatewayID: gatewayID,
					Message:   &gateway.Status{},
				}
				err := p.HandleStatus(middleware.NewContext(), status)
				Convey("There should be no error", func() {
					So(err, ShouldBeNil)
				})
				Convey("The StatusMessage should have Metadata", func() {
					So(status.Message.GetLocation(), ShouldNotBeNil)
					So(status.Message.GetLocation().Latitude, ShouldEqual, 12.34)
					So(status.Message.Description, ShouldEqual, "My Test Gateway")
					So(status.Message.Platform, ShouldEqual, "Test Gateway")
					So(status.Message.FrequencyPlan, ShouldEqual, "EU_868")
				})
			})
		})

	})

	Convey("Given a new Public GatewayInfo that Expires", t, func(c C) {
		p := NewPublic("https://account.thethingsnetwork.org").WithExpire(10 * time.Millisecond)
		gatewayID := "eui-0000024b08060112"

		Convey("When setting the info of a Gateway", func() {
			p.set(gatewayID, account.Gateway{})
			lastUpdated := p.info[gatewayID].lastUpdated
			Convey("When getting the info of a Gateway some time later", func() {
				time.Sleep(20 * time.Millisecond)
				p.get(gatewayID)
				time.Sleep(500 * time.Millisecond)
				Convey("It should have updated", func() {
					So(p.info[gatewayID].lastUpdated, ShouldNotEqual, lastUpdated)
				})
			})
		})
	})

	Convey("Given a new Public GatewayInfo with Redis", t, func(c C) {
		p, _ := NewPublic("https://account.thethingsnetwork.org").WithRedis(getRedisClient(), "test-public")
		gatewayID := "eui-0000024b08060112"
		Reset(func() {
			getRedisClient().Del(p.redisKey(gatewayID)).Err()
		})
		Convey("When setting the info of a Gateway", func() {
			p.set(gatewayID, account.Gateway{})
			Convey("It should be stored in Redis", func() {
				So(getRedisClient().Exists(p.redisKey(gatewayID)).Val(), ShouldBeTrue)
			})
		})
		Convey("After re-initializing", func() {
			getRedisClient().Set(p.redisKey(gatewayID), `{"activated":true}`, 0).Err()
			p.info = make(map[string]*info)
			_, err := p.WithRedis(getRedisClient(), "test-public")
			Convey("There should be no error", func() {
				So(err, ShouldBeNil)
			})
			Convey("When getting the info of the Gateway", func() {
				gateway, err := p.get(gatewayID)
				Convey("There should be no error", func() {
					So(err, ShouldBeNil)
				})
				Convey("It should be restored from Redis", func() {
					So(gateway.Activated, ShouldBeTrue)
				})
			})
		})
	})
}
