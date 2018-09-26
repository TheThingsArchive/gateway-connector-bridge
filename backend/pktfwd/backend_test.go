package pktfwd

import (
	"encoding/base64"
	"net"
	"testing"
	"time"

	pb_gateway "github.com/TheThingsNetwork/api/gateway"
	pb_protocol "github.com/TheThingsNetwork/api/protocol"
	pb_lorawan "github.com/TheThingsNetwork/api/protocol/lorawan"
	pb_router "github.com/TheThingsNetwork/api/router"
	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
	"github.com/brocaar/lorawan"
	. "github.com/smartystreets/goconvey/convey"
)

func TestBackend(t *testing.T) {
	Convey("Given a new Backend binding at a random port", t, func() {
		backend, err := NewBackend(Config{Bind: "127.0.0.1:0"}, nil, nil, false)
		So(err, ShouldBeNil)

		backendAddr, err := net.ResolveUDPAddr("udp", backend.conn.LocalAddr().String())
		So(err, ShouldBeNil)

		Convey("Given a fake gateway UDP publisher", func() {
			gwAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
			So(err, ShouldBeNil)
			gwConn, err := net.ListenUDP("udp", gwAddr)
			So(err, ShouldBeNil)
			defer gwConn.Close()
			So(gwConn.SetDeadline(time.Now().Add(time.Second)), ShouldBeNil)

			Convey("When sending a PULL_DATA packet", func() {
				p := PullDataPacket{
					ProtocolVersion: ProtocolVersion2,
					RandomToken:     12345,
					GatewayMAC:      [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
				}
				b, err := p.MarshalBinary()
				So(err, ShouldBeNil)
				_, err = gwConn.WriteToUDP(b, backendAddr)
				So(err, ShouldBeNil)

				Convey("Then an ACK packet is returned", func() {
					buf := make([]byte, 65507)
					i, _, err := gwConn.ReadFromUDP(buf)
					So(err, ShouldBeNil)
					var ack PullACKPacket
					So(ack.UnmarshalBinary(buf[:i]), ShouldBeNil)
					So(ack.RandomToken, ShouldEqual, p.RandomToken)
					So(ack.ProtocolVersion, ShouldEqual, p.ProtocolVersion)
				})
			})

			Convey("When sending a PUSH_DATA packet with stats", func() {
				p := PushDataPacket{
					ProtocolVersion: ProtocolVersion2,
					RandomToken:     1234,
					GatewayMAC:      [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
					Payload: PushDataPayload{
						Stat: &Stat{
							Time: ExpandedTime(time.Time{}.UTC()),
							Lati: 1.234,
							Long: 2.123,
							Alti: 123,
							RXNb: 1,
							RXOK: 2,
							RXFW: 3,
							ACKR: 33.3,
							DWNb: 4,
							TXNb: 3,
						},
					},
				}
				b, err := p.MarshalBinary()
				So(err, ShouldBeNil)
				_, err = gwConn.WriteToUDP(b, backendAddr)
				So(err, ShouldBeNil)

				Convey("Then an ACK packet is returned", func() {
					buf := make([]byte, 65507)
					i, _, err := gwConn.ReadFromUDP(buf)
					So(err, ShouldBeNil)
					var ack PushACKPacket
					So(ack.UnmarshalBinary(buf[:i]), ShouldBeNil)
					So(ack.RandomToken, ShouldEqual, p.RandomToken)
					So(ack.ProtocolVersion, ShouldEqual, p.ProtocolVersion)

					Convey("Then the gateway stats are returned by the stats channel", func() {
						stats := <-backend.StatsChan()
						So(stats.GatewayID, ShouldEqual, "eui-0102030405060708")
					})
				})
			})

			Convey("Given skipCRCCheck=false", func() {
				backend.skipCRCCheck = false

				Convey("When sending a PUSH_DATA packet with RXPK (CRC OK)", func() {
					p := PushDataPacket{
						ProtocolVersion: ProtocolVersion2,
						RandomToken:     1234,
						GatewayMAC:      [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
						Payload: PushDataPayload{
							RXPK: []RXPK{
								{
									Time: CompactTime(time.Now().UTC()),
									Tmst: 708016819,
									Freq: 868.5,
									Chan: 2,
									RFCh: 1,
									Stat: 1,
									Modu: "LORA",
									DatR: DatR{LoRa: "SF7BW125"},
									CodR: "4/5",
									RSSI: -51,
									LSNR: 7,
									Size: 16,
									Data: "QAEBAQGAAAABVfdjR6YrSw==",
								},
							},
						},
					}
					b, err := p.MarshalBinary()
					So(err, ShouldBeNil)
					_, err = gwConn.WriteToUDP(b, backendAddr)
					So(err, ShouldBeNil)

					Convey("Then an ACK packet is returned", func() {
						buf := make([]byte, 65507)
						i, _, err := gwConn.ReadFromUDP(buf)
						So(err, ShouldBeNil)
						var ack PushACKPacket
						So(ack.UnmarshalBinary(buf[:i]), ShouldBeNil)
						So(ack.RandomToken, ShouldEqual, p.RandomToken)
						So(ack.ProtocolVersion, ShouldEqual, p.ProtocolVersion)
					})

					Convey("Then the packet is returned by the RX packet channel", func() {
						rxPacket := <-backend.RXPacketChan()

						rxPacket2, err := newRXPacketFromRXPK(p.GatewayMAC, p.Payload.RXPK[0])
						So(err, ShouldBeNil)
						rxPacket.Message.Trace = nil // Unset the trace, we're not testing that
						rxPacket.GatewayAddr = nil
						So(rxPacket, ShouldResemble, rxPacket2)
					})
				})

				Convey("When sending a PUSH_DATA packet with RXPK (CRC not OK)", func() {
					p := PushDataPacket{
						ProtocolVersion: ProtocolVersion2,
						RandomToken:     1234,
						GatewayMAC:      [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
						Payload: PushDataPayload{
							RXPK: []RXPK{
								{
									Time: CompactTime(time.Now().UTC()),
									Tmst: 708016819,
									Freq: 868.5,
									Chan: 2,
									RFCh: 1,
									Stat: -1,
									Modu: "LORA",
									DatR: DatR{LoRa: "SF7BW125"},
									CodR: "4/5",
									RSSI: -51,
									LSNR: 7,
									Size: 16,
									Data: "QAEBAQGAAAABVfdjR6YrSw==",
								},
							},
						},
					}
					b, err := p.MarshalBinary()
					So(err, ShouldBeNil)
					_, err = gwConn.WriteToUDP(b, backendAddr)
					So(err, ShouldBeNil)

					Convey("Then an ACK packet is returned", func() {
						buf := make([]byte, 65507)
						i, _, err := gwConn.ReadFromUDP(buf)
						So(err, ShouldBeNil)
						var ack PushACKPacket
						So(ack.UnmarshalBinary(buf[:i]), ShouldBeNil)
						So(ack.RandomToken, ShouldEqual, p.RandomToken)
						So(ack.ProtocolVersion, ShouldEqual, p.ProtocolVersion)
					})

					Convey("Then the packet is not returned by the RX packet channel", func() {
						So(backend.RXPacketChan(), ShouldHaveLength, 0)
					})
				})
			})

			Convey("Given skipCRCCheck=true", func() {
				backend.skipCRCCheck = true

				Convey("When sending a PUSH_DATA packet with RXPK (CRC OK)", func() {
					p := PushDataPacket{
						ProtocolVersion: ProtocolVersion2,
						RandomToken:     1234,
						GatewayMAC:      [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
						Payload: PushDataPayload{
							RXPK: []RXPK{
								{
									Time: CompactTime(time.Now().UTC()),
									Tmst: 708016819,
									Freq: 868.5,
									Chan: 2,
									RFCh: 1,
									Stat: 1,
									Modu: "LORA",
									DatR: DatR{LoRa: "SF7BW125"},
									CodR: "4/5",
									RSSI: -51,
									LSNR: 7,
									Size: 16,
									Data: "QAEBAQGAAAABVfdjR6YrSw==",
								},
							},
						},
					}
					b, err := p.MarshalBinary()
					So(err, ShouldBeNil)
					_, err = gwConn.WriteToUDP(b, backendAddr)
					So(err, ShouldBeNil)

					Convey("Then an ACK packet is returned", func() {
						buf := make([]byte, 65507)
						i, _, err := gwConn.ReadFromUDP(buf)
						So(err, ShouldBeNil)
						var ack PushACKPacket
						So(ack.UnmarshalBinary(buf[:i]), ShouldBeNil)
						So(ack.RandomToken, ShouldEqual, p.RandomToken)
						So(ack.ProtocolVersion, ShouldEqual, p.ProtocolVersion)
					})

					Convey("Then the packet is returned by the RX packet channel", func() {
						rxPacket := <-backend.RXPacketChan()

						rxPacket2, err := newRXPacketFromRXPK(p.GatewayMAC, p.Payload.RXPK[0])
						So(err, ShouldBeNil)
						rxPacket.Message.Trace = nil // Unset the trace, we're not testing that
						rxPacket.GatewayAddr = nil
						So(rxPacket, ShouldResemble, rxPacket2)
					})
				})

				Convey("When sending a PUSH_DATA packet with RXPK (CRC not OK)", func() {
					p := PushDataPacket{
						ProtocolVersion: ProtocolVersion2,
						RandomToken:     1234,
						GatewayMAC:      [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
						Payload: PushDataPayload{
							RXPK: []RXPK{
								{
									Time: CompactTime(time.Now().UTC()),
									Tmst: 708016819,
									Freq: 868.5,
									Chan: 2,
									RFCh: 1,
									Stat: -1,
									Modu: "LORA",
									DatR: DatR{LoRa: "SF7BW125"},
									CodR: "4/5",
									RSSI: -51,
									LSNR: 7,
									Size: 16,
									Data: "QAEBAQGAAAABVfdjR6YrSw==",
								},
							},
						},
					}
					b, err := p.MarshalBinary()
					So(err, ShouldBeNil)
					_, err = gwConn.WriteToUDP(b, backendAddr)
					So(err, ShouldBeNil)

					Convey("Then an ACK packet is returned", func() {
						buf := make([]byte, 65507)
						i, _, err := gwConn.ReadFromUDP(buf)
						So(err, ShouldBeNil)
						var ack PushACKPacket
						So(ack.UnmarshalBinary(buf[:i]), ShouldBeNil)
						So(ack.RandomToken, ShouldEqual, p.RandomToken)
						So(ack.ProtocolVersion, ShouldEqual, p.ProtocolVersion)
					})

					Convey("Then the packet is returned by the RX packet channel", func() {
						rxPacket := <-backend.RXPacketChan()

						rxPacket2, err := newRXPacketFromRXPK(p.GatewayMAC, p.Payload.RXPK[0])
						So(err, ShouldBeNil)
						rxPacket.Message.Trace = nil // Unset the trace, we're not testing that
						rxPacket.GatewayAddr = nil
						So(rxPacket, ShouldResemble, rxPacket2)
					})
				})
			})

			Convey("Given a TXPacket", func() {
				txPacket := &types.DownlinkMessage{
					GatewayID: "eui-0102030405060708",
					Message: &pb_router.DownlinkMessage{
						ProtocolConfiguration: pb_protocol.TxConfiguration{
							Protocol: &pb_protocol.TxConfiguration_LoRaWAN{
								LoRaWAN: &pb_lorawan.TxConfiguration{
									Modulation: pb_lorawan.Modulation_LORA,
									DataRate:   "SF12BW250",
									CodingRate: "4/5",
								},
							},
						},
						GatewayConfiguration: pb_gateway.TxConfiguration{
							Timestamp:             12345,
							Frequency:             868100000,
							Power:                 14,
							PolarizationInversion: true,
						},
						Payload: []byte{1, 2, 3, 4},
					},
				}

				Convey("When sending the TXPacket and the gateway is not known to the backend", func() {
					err := backend.Send(txPacket)
					Convey("Then the backend returns an error", func() {
						So(err, ShouldEqual, errGatewayDoesNotExist)
					})
				})

				Convey("When sending the TXPacket when the gateway is known to the backend", func() {
					// sending a ping should register the gateway to the backend
					p := PullDataPacket{
						ProtocolVersion: ProtocolVersion2,
						RandomToken:     12345,
						GatewayMAC:      [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
					}
					b, err := p.MarshalBinary()
					So(err, ShouldBeNil)
					_, err = gwConn.WriteToUDP(b, backendAddr)
					So(err, ShouldBeNil)
					buf := make([]byte, 65507)
					i, _, err := gwConn.ReadFromUDP(buf)
					So(err, ShouldBeNil)
					var ack PullACKPacket
					So(ack.UnmarshalBinary(buf[:i]), ShouldBeNil)
					So(ack.RandomToken, ShouldEqual, p.RandomToken)
					So(ack.ProtocolVersion, ShouldEqual, p.ProtocolVersion)

					err = backend.Send(txPacket)

					Convey("Then no error is returned", func() {
						So(err, ShouldBeNil)
					})

					Convey("Then the data is received by the gateway", func() {
						i, _, err := gwConn.ReadFromUDP(buf)
						So(err, ShouldBeNil)
						So(i, ShouldBeGreaterThan, 0)
						var pullResp PullRespPacket
						So(pullResp.UnmarshalBinary(buf[:i]), ShouldBeNil)
						pullResp.RandomToken = 0

						So(pullResp, ShouldResemble, PullRespPacket{
							ProtocolVersion: p.ProtocolVersion,
							Payload: PullRespPayload{
								TXPK: TXPK{
									Imme: false,
									Tmst: 12345,
									Freq: 868.1,
									Powe: 14,
									Modu: "LORA",
									DatR: DatR{
										LoRa: "SF12BW250",
									},
									CodR: "4/5",
									Size: uint16(len([]byte{1, 2, 3, 4})),
									Data: base64.StdEncoding.EncodeToString([]byte{1, 2, 3, 4}),
									IPol: true,
									NCRC: true,
								},
							},
						})
					})
				})
			})
		})
	})
}

func TestNewGatewayStatPacket(t *testing.T) {
	Convey("Given a (Semtech) Stat struct and gateway MAC", t, func() {
		now := time.Now().UTC()
		stat := Stat{
			Time: ExpandedTime(now),
			Lati: 1.234,
			Long: 2.123,
			Alti: 234,
			RXNb: 1,
			RXOK: 2,
			RXFW: 3,
			ACKR: 33.3,
			DWNb: 4,
			TXNb: 3,
		}
		mac := [8]byte{1, 2, 3, 4, 5, 6, 7, 8}

		Convey("When calling newGatewayStatsPacket", func() {
			gwStats := newGatewayStatsPacket(mac, stat)
			Convey("Then all fields are set correctly", func() {
				So(gwStats.GatewayID, ShouldEqual, "eui-0102030405060708")
				So(gwStats.Message, ShouldResemble, &pb_gateway.Status{
					Time: now.UnixNano(),
					Location: &pb_gateway.LocationMetadata{
						Latitude:  1.234,
						Longitude: 2.123,
						Altitude:  234,
					},
					RxIn: 1,
					RxOk: 2,
					TxIn: 4,
					TxOk: 3,
				})
			})
		})

	})
}

func TestNewTXPKFromTXPacket(t *testing.T) {
	Convey("Given a TXPacket", t, func() {
		txPacket := &types.DownlinkMessage{
			GatewayID: "eui-0102030405060708",
			Message: &pb_router.DownlinkMessage{
				ProtocolConfiguration: pb_protocol.TxConfiguration{
					Protocol: &pb_protocol.TxConfiguration_LoRaWAN{
						LoRaWAN: &pb_lorawan.TxConfiguration{
							Modulation: pb_lorawan.Modulation_LORA,
							DataRate:   "SF9BW250",
							CodingRate: "4/5",
						},
					},
				},
				GatewayConfiguration: pb_gateway.TxConfiguration{
					Timestamp:             12345,
					Frequency:             868100000,
					Power:                 14,
					PolarizationInversion: true,
				},
				Payload: []byte{1, 2, 3, 4},
			},
		}

		Convey("Then te expected TXPK is returned", func() {
			txpk, err := newTXPKFromTXPacket(txPacket)
			So(err, ShouldBeNil)
			So(txpk, ShouldResemble, TXPK{
				Imme: false,
				Tmst: 12345,
				Freq: 868.1,
				Powe: 14,
				Modu: "LORA",
				DatR: DatR{
					LoRa: "SF9BW250",
				},
				CodR: "4/5",
				Size: 4,
				Data: "AQIDBA==",
				IPol: true,
				NCRC: true,
			})
		})

		Convey("Given IPol is requested to false", func() {
			txPacket.Message.GatewayConfiguration.PolarizationInversion = false

			Convey("Then the TXPK IPol is set to false", func() {
				txpk, err := newTXPKFromTXPacket(txPacket)
				So(err, ShouldBeNil)
				So(txpk.IPol, ShouldBeFalse)
			})
		})
	})
}

func TestNewRXPacketFromRXPK(t *testing.T) {
	Convey("Given a (Semtech) RXPK and gateway MAC", t, func() {
		now := time.Now().UTC()
		rxpk := RXPK{
			Time: CompactTime(now),
			Tmst: 708016819,
			Freq: 868.5,
			Chan: 2,
			RFCh: 1,
			Stat: 1,
			Modu: "LORA",
			DatR: DatR{LoRa: "SF7BW125"},
			CodR: "4/5",
			RSSI: -54,
			LSNR: 6.5,
			Size: 16,
			Data: base64.StdEncoding.EncodeToString([]byte{1, 2, 3, 4}),
			RSig: []RSig{
				RSig{Ant: 0, Chan: 2, RSSIS: -54, LSNR: 6.5},
				RSig{Ant: 1, Chan: 2, RSSIS: -51, LSNR: 7},
			},
		}
		mac := [8]byte{1, 2, 3, 4, 5, 6, 7, 8}

		Convey("When calling newRXPacketFromRXPK(", func() {
			rxPacket, err := newRXPacketFromRXPK(mac, rxpk)
			So(err, ShouldBeNil)

			Convey("Then all fields are set correctly", func() {
				So(rxPacket.GatewayID, ShouldEqual, "eui-0102030405060708")

				So(rxPacket.Message.Payload, ShouldResemble, []byte{1, 2, 3, 4})

				So(rxPacket.Message.ProtocolMetadata.GetLoRaWAN(), ShouldResemble, &pb_lorawan.Metadata{
					Modulation: pb_lorawan.Modulation_LORA,
					DataRate:   "SF7BW125",
					CodingRate: "4/5",
				})

				So(rxPacket.Message.GatewayMetadata, ShouldResemble, pb_gateway.RxMetadata{
					GatewayID: "eui-0102030405060708",
					Time:      now.UnixNano(),
					Timestamp: 708016819,
					Channel:   2,
					RfChain:   1,
					Frequency: 868500000,
					RSSI:      -51,
					SNR:       7,
					Antennas: []*pb_gateway.RxMetadata_Antenna{
						&pb_gateway.RxMetadata_Antenna{Antenna: 0, Channel: 2, RSSI: -54, SNR: 6.5},
						&pb_gateway.RxMetadata_Antenna{Antenna: 1, Channel: 2, RSSI: -51, SNR: 7},
					},
				})
			})
		})
	})
}

func TestGatewaysCallbacks(t *testing.T) {
	Convey("Given a new gateways registry", t, func() {
		gw := gateways{
			gateways: make(map[lorawan.EUI64]gateway),
		}

		mac := [8]byte{1, 2, 3, 4, 5, 6, 7, 8}

		Convey("Given a onNew and onDelete callback", func() {
			var onNewCalls int
			var onDeleteCalls int

			gw.onNew = func(mac lorawan.EUI64) error {
				onNewCalls = onNewCalls + 1
				return nil
			}

			gw.onDelete = func(mac lorawan.EUI64) error {
				onDeleteCalls = onDeleteCalls + 1
				return nil
			}

			Convey("When adding a new gateway", func() {
				So(gw.set(mac, gateway{}), ShouldBeNil)

				Convey("Then onNew callback is called once", func() {
					So(onNewCalls, ShouldEqual, 1)
				})

				Convey("When updating the same gateway", func() {
					So(gw.set(mac, gateway{}), ShouldBeNil)

					Convey("Then onNew has not been called", func() {
						So(onNewCalls, ShouldEqual, 1)
					})
				})

				Convey("When cleaning up the gateways", func() {
					So(gw.cleanup(), ShouldBeNil)

					Convey("Then onDelete has been called once", func() {
						So(onDeleteCalls, ShouldEqual, 1)
					})
				})
			})
		})
	})
}
