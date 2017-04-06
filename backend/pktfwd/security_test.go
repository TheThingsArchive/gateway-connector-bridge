package pktfwd

import (
	"net"
	"testing"

	"time"

	"github.com/brocaar/lorawan"
	. "github.com/smartystreets/goconvey/convey"
)

func udpAddr(str string) *net.UDPAddr {
	addr, err := net.ResolveUDPAddr("udp", str)
	if err != nil {
		panic(err)
	}
	return addr
}

func TestSecurity(t *testing.T) {
	Convey("Given an empty sourceLocks with port checks off", t, func() {
		c := newSourceLocks(false, 20*time.Millisecond)

		Convey("When binding a gateway to an addr", func() {
			err := c.Set(lorawan.EUI64([8]byte{1, 2, 3, 4, 5, 6, 7, 8}), udpAddr("127.0.0.1:12345"))
			Convey("There should be no error", func() {
				So(err, ShouldBeNil)
			})
			Convey("When re-binding a gateway to the same addr", func() {
				err := c.Set(lorawan.EUI64([8]byte{1, 2, 3, 4, 5, 6, 7, 8}), udpAddr("127.0.0.1:12345"))
				Convey("There should be no error", func() {
					So(err, ShouldBeNil)
				})
			})
			Convey("When re-binding a gateway to a different addr", func() {
				err := c.Set(lorawan.EUI64([8]byte{1, 2, 3, 4, 5, 6, 7, 8}), udpAddr("127.0.0.2:12345"))
				Convey("There should be an error", func() {
					So(err, ShouldNotBeNil)
				})
			})
			Convey("When re-binding a gateway to a different addr after the expiry time", func() {
				time.Sleep(25 * time.Millisecond)
				err := c.Set(lorawan.EUI64([8]byte{1, 2, 3, 4, 5, 6, 7, 8}), udpAddr("127.0.0.2:12345"))
				Convey("There should be no error", func() {
					So(err, ShouldBeNil)
				})
			})
		})
	})

	Convey("Given an empty sourceLocks with port checks on", t, func() {
		c := newSourceLocks(true, 20*time.Millisecond)

		Convey("When binding a gateway to an addr", func() {
			err := c.Set(lorawan.EUI64([8]byte{1, 2, 3, 4, 5, 6, 7, 8}), udpAddr("127.0.0.1:12345"))
			Convey("There should be no error", func() {
				So(err, ShouldBeNil)
			})
			Convey("When re-binding a gateway to the same addr", func() {
				err := c.Set(lorawan.EUI64([8]byte{1, 2, 3, 4, 5, 6, 7, 8}), udpAddr("127.0.0.1:12345"))
				Convey("There should be no error", func() {
					So(err, ShouldBeNil)
				})
			})
			Convey("When re-binding a gateway to a different port", func() {
				err := c.Set(lorawan.EUI64([8]byte{1, 2, 3, 4, 5, 6, 7, 8}), udpAddr("127.0.0.1:12346"))
				Convey("There should be an error", func() {
					So(err, ShouldNotBeNil)
				})
			})
			Convey("When re-binding a gateway to a different port after the expiry time", func() {
				time.Sleep(25 * time.Millisecond)
				err := c.Set(lorawan.EUI64([8]byte{1, 2, 3, 4, 5, 6, 7, 8}), udpAddr("127.0.0.2:12346"))
				Convey("There should be no error", func() {
					So(err, ShouldBeNil)
				})
			})
		})
	})
}
