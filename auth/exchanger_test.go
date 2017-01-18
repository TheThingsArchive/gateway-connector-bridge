// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package auth

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/apex/log"
	"github.com/apex/log/handlers/text"
	. "github.com/smartystreets/goconvey/convey"
)

func TestAccountServerExchanger(t *testing.T) {
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

		Convey("Given a new AccountServerExchanger", func() {
			e := &AccountServerExchanger{
				ctx:           ctx,
				accountServer: "https://preview.account.thethingsnetwork.org",
			}

			Convey("When calling Exchange for an invalid gateway ID", func() {
				_, _, err := e.Exchange("some-invalid-gateway-id-that-does-not-exist", "some-invalid-key")
				Convey("There should be an error", func() {
					So(err, ShouldNotBeNil)
				})
			})

			if gatewayID := os.Getenv("GATEWAY_ID"); gatewayID != "" {
				Convey("When calling Exchange for an invalid gateway Key", func() {
					_, _, err := e.Exchange(gatewayID, "some-invalid-key")
					Convey("There should be an error", func() {
						So(err, ShouldNotBeNil)
					})
				})

				if gatewayKey := os.Getenv("GATEWAY_KEY"); gatewayKey != "" {
					Convey("When calling Exchange with a valid gateway key", func() {
						token, expires, err := e.Exchange(gatewayID, gatewayKey)
						Convey("There should be no error", func() {
							So(err, ShouldBeNil)
						})
						Convey("A token should have been returned", func() {
							So(token, ShouldNotBeEmpty)
						})
						Convey("The token should expire in the future", func() {
							So(expires, ShouldHappenAfter, time.Now())
						})
					})
				}
			}

		})

	})

}
