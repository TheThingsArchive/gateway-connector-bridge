// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package blacklist

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"

	"github.com/TheThingsNetwork/gateway-connector-bridge/middleware"
	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
	. "github.com/smartystreets/goconvey/convey"
)

func TestBlacklist(t *testing.T) {
	exampleBlacklist := os.ExpandEnv("$GOPATH/src/github.com/TheThingsNetwork/gateway-connector-bridge/assets/blacklist.example.yml")
	if _, err := os.Stat(exampleBlacklist); err != nil {
		panic(fmt.Errorf("blacklist example file not found: %s", err))
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/blacklist.yml", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, exampleBlacklist)
	})

	testExample := func(list string) {
		b, err := NewBlacklist(list)
		Convey("Then there should be no error", func() { So(err, ShouldBeNil) })
		Reset(func() { b.Close() })
		Convey("Then the blacklist should contain 2 items", func() {
			So(b.lists[list], ShouldHaveLength, 2)
		})
		Convey("When a gateway with a blacklisted ID sends a message", func() {
			err := b.HandleStatus(middleware.NewContext(), &types.StatusMessage{GatewayID: "malicious"})
			Convey("Then the BlacklistedID error should be returned", func() { So(err, ShouldEqual, ErrBlacklistedID) })
		})
		Convey("When a gateway with a blacklisted IP sends a message", func() {
			err := b.HandleStatus(middleware.NewContext(), &types.StatusMessage{GatewayAddr: &net.TCPAddr{IP: net.IP{8, 8, 8, 8}}})
			Convey("Then the BlacklistedIP error should be returned", func() { So(err, ShouldEqual, ErrBlacklistedIP) })
		})
	}

	Convey("When creating a new Blacklist using the example file", t, func(c C) {
		testExample(exampleBlacklist)
	})

	Convey("When creating a new Blacklist using the example file on an HTTP server", t, func(c C) {
		var lis net.Listener
		var err error
		for {
			lis, err = net.Listen("tcp", "127.0.0.1:0")
			if err == nil {
				break
			}
		}
		Reset(func() { lis.Close() })
		go http.Serve(lis, mux)
		testExample(fmt.Sprintf("http://%s/blacklist.yml", lis.Addr().String()))
	})
}
