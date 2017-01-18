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

func TestAuth(t *testing.T) {
	Convey("Given a new Context and Exchanger", t, func(c C) {

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

		e := &AccountServerExchanger{
			ctx:           ctx,
			accountServer: "https://preview.account.thethingsnetwork.org",
		}

		Convey("Given a new auth.Memory", func() {
			a := NewMemory()
			Convey("When running the standardized test", standardizedTest(a, e))
		})

		Convey("Given a new auth.Redis", func() {
			a := NewRedis(getRedisClient(), "test-auth")
			Convey("When running the standardized test", standardizedTest(a, e))
		})

	})
}

func standardizedTest(a Interface, e Exchanger) func() {
	return func() {
		Convey("When getting the token for an unknown gateway", func() {
			_, err := a.GetToken("unknown-gateway")
			Convey("There should be a NotFound error", func() {
				So(err, ShouldNotBeNil)
				So(err, ShouldEqual, ErrGatewayNotFound)
			})
		})

		Convey("When setting a key", func() {
			err := a.SetKey("gateway-with-key", "the-key")
			Reset(func() {
				a.Delete("gateway-with-key")
			})
			Convey("There should be no error", func() {
				So(err, ShouldBeNil)
			})
			Convey("When deleting the gateway", func() {
				err := a.Delete("gateway-with-key")
				Convey("There should be no error", func() {
					So(err, ShouldBeNil)
				})
				Convey("When getting the token", func() {
					_, err := a.GetToken("gateway-with-key")
					Convey("There should be a NotFound error", func() {
						So(err, ShouldNotBeNil)
						So(err, ShouldEqual, ErrGatewayNotFound)
					})
				})
			})
			Convey("When getting the token", func() {
				_, err := a.GetToken("gateway-with-key")
				Convey("There should be a NoValidToken error", func() {
					So(err, ShouldNotBeNil)
					So(err, ShouldEqual, ErrGatewayNoValidToken)
				})
			})
			Convey("When updating the key", func() {
				err := a.SetKey("gateway-with-key", "the-new-key")
				Convey("There should be no error", func() {
					So(err, ShouldBeNil)
				})
				Convey("When getting the token", func() {
					_, err := a.GetToken("gateway-with-key")
					Convey("There should still be a NoValidToken error", func() {
						So(err, ShouldNotBeNil)
						So(err, ShouldEqual, ErrGatewayNoValidToken)
					})
				})
			})
		})

		Convey("When setting a token", func() {
			err := a.SetToken("gateway-with-token", "the-token", time.Now().Add(time.Second))
			Reset(func() {
				a.Delete("gateway-with-token")
			})
			Convey("There should be no error", func() {
				So(err, ShouldBeNil)
			})
			Convey("When deleting the gateway", func() {
				err := a.Delete("gateway-with-token")
				Convey("There should be no error", func() {
					So(err, ShouldBeNil)
				})
				Convey("When getting the token", func() {
					_, err := a.GetToken("gateway-with-token")
					Convey("There should be a NotFound error", func() {
						So(err, ShouldNotBeNil)
						So(err, ShouldEqual, ErrGatewayNotFound)
					})
				})
			})
			Convey("When getting the token", func() {
				token, err := a.GetToken("gateway-with-token")
				Convey("There should be no error", func() {
					So(err, ShouldBeNil)
				})
				Convey("The token should be the one we set", func() {
					So(token, ShouldEqual, "the-token")
				})
			})
			Convey("When updating the token", func() {
				err := a.SetToken("gateway-with-token", "the-new-token", time.Now().Add(time.Second))
				Convey("There should be no error", func() {
					So(err, ShouldBeNil)
				})
				Convey("When getting the token", func() {
					token, err := a.GetToken("gateway-with-token")
					Convey("There should be no error", func() {
						So(err, ShouldBeNil)
					})
					Convey("The token should be the one we updated", func() {
						So(token, ShouldEqual, "the-new-token")
					})
				})
			})
		})

		gatewayID := os.Getenv("GATEWAY_ID")
		gatewayKey := os.Getenv("GATEWAY_KEY")

		if gatewayID != "" && gatewayKey != "" {
			Convey("When setting the Exchanger", func() {
				a.SetExchanger(e)

				Convey("When setting a key", func() {
					err := a.SetKey(gatewayID, gatewayKey)
					Reset(func() {
						a.Delete(gatewayID)
					})
					Convey("There should be no error", func() {
						So(err, ShouldBeNil)
					})
					Convey("When validating the correct key", func() {
						err := a.ValidateKey(gatewayID, gatewayKey)
						Convey("There should be no error", func() {
							So(err, ShouldBeNil)
						})
					})
					Convey("When validating an incorrect key", func() {
						err := a.ValidateKey(gatewayID, "incorrect")
						Convey("There should be an error", func() {
							So(err, ShouldNotBeNil)
						})
					})
					Convey("When getting the token", func() {
						token, err := a.GetToken(gatewayID)
						Convey("There should be no error", func() {
							So(err, ShouldBeNil)
						})
						Convey("There should be a token", func() {
							So(token, ShouldNotBeEmpty)
						})
					})
					Convey("When expiring the token", func() {
						a.SetToken(gatewayID, "expired-token", time.Now())
						Convey("When getting the token again", func() {
							token, err := a.GetToken(gatewayID)
							Convey("There should be no error", func() {
								So(err, ShouldBeNil)
							})
							Convey("There should be a new token", func() {
								So(token, ShouldNotEqual, "expired-token")
							})
						})
					})
				})

			})
		}
	}
}
