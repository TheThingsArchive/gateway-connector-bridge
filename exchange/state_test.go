// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package exchange

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/apex/log"
	"github.com/apex/log/handlers/text"
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

func TestExchangeState(t *testing.T) {
	Convey("Given a new Context and Backends", t, func(c C) {

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

		Convey("When creating a new Exchange", func() {
			b := New(ctx, 0)

			Convey("When calling InitRedisState", func() {
				gatewayIDs := b.InitRedisState(getRedisClient(), "")
				Convey("It should not return any gateways", func() {
					So(gatewayIDs, ShouldBeEmpty)
				})

				Convey("When adding a gateway", func() {
					b.gateways.Add("dev")
					Reset(func() {
						b.gateways.Remove("dev")
					})
					Convey("When calling InitRedisState on another Exchange", func() {
						gatewayIDs := New(ctx, 0).InitRedisState(getRedisClient(), "")
						Convey("It should return the gateway", func() {
							So(gatewayIDs, ShouldContain, "dev")
						})
					})
					Convey("When removing that gateway", func() {
						b.gateways.Remove("dev")
						Reset(func() {
							b.gateways.Add("dev")
						})
						Convey("When calling InitRedisState on another Exchange", func() {
							gatewayIDs := New(ctx, 0).InitRedisState(getRedisClient(), "")
							Convey("It should not return the gateway", func() {
								So(gatewayIDs, ShouldNotContain, "dev")
							})
						})
					})
				})
			})

		})
	})
}
