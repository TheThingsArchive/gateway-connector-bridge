package pktfwd

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
	"github.com/TheThingsNetwork/go-utils/log"
	pb_gateway "github.com/TheThingsNetwork/ttn/api/gateway"
	pb_protocol "github.com/TheThingsNetwork/ttn/api/protocol"
	pb_lorawan "github.com/TheThingsNetwork/ttn/api/protocol/lorawan"
	pb_router "github.com/TheThingsNetwork/ttn/api/router"
	"github.com/TheThingsNetwork/ttn/api/trace"
	"github.com/brocaar/lorawan"
	"github.com/brocaar/lorawan/band"
)

var errGatewayDoesNotExist = errors.New("gateway does not exist")
var gatewayCleanupDuration = -1 * time.Minute
var loRaDataRateRegex = regexp.MustCompile(`SF(\d+)BW(\d+)`)

type udpPacket struct {
	addr *net.UDPAddr
	data []byte
}

type gateway struct {
	addr            *net.UDPAddr
	lastSeen        time.Time
	protocolVersion uint8
}

type gateways struct {
	sync.RWMutex
	gateways map[lorawan.EUI64]gateway
	onNew    func(lorawan.EUI64) error
	onDelete func(lorawan.EUI64) error
}

func getMac(id string) (mac lorawan.EUI64) {
	id = strings.TrimPrefix(id, "eui-")
	mac.UnmarshalText([]byte(id))
	return
}

func getID(mac lorawan.EUI64) string {
	txt, _ := mac.MarshalText()
	return "eui-" + string(txt)
}

func (c *gateways) get(mac lorawan.EUI64) (gateway, error) {
	defer c.RUnlock()
	c.RLock()
	gw, ok := c.gateways[mac]
	if !ok {
		return gw, errGatewayDoesNotExist
	}
	return gw, nil
}

func (c *gateways) set(mac lorawan.EUI64, gw gateway) error {
	defer c.Unlock()
	c.Lock()
	_, ok := c.gateways[mac]
	if !ok && c.onNew != nil {
		if err := c.onNew(mac); err != nil {
			return err
		}
	}
	c.gateways[mac] = gw
	return nil
}

func (c *gateways) cleanup() error {
	defer c.Unlock()
	c.Lock()
	for mac := range c.gateways {
		if c.gateways[mac].lastSeen.Before(time.Now().Add(gatewayCleanupDuration)) {
			if c.onDelete != nil {
				if err := c.onDelete(mac); err != nil {
					return err
				}
			}
			delete(c.gateways, mac)
		}
	}
	return nil
}

// Backend implements a Semtech gateway backend.
type Backend struct {
	log          log.Interface
	conn         *net.UDPConn
	rxChan       chan *types.UplinkMessage
	statsChan    chan *types.StatusMessage
	udpSendChan  chan udpPacket
	closed       bool
	gateways     gateways
	wg           sync.WaitGroup
	skipCRCCheck bool
}

// NewBackend creates a new backend.
func NewBackend(bind string, onNew func(lorawan.EUI64) error, onDelete func(lorawan.EUI64) error, skipCRCCheck bool) (*Backend, error) {
	addr, err := net.ResolveUDPAddr("udp", bind)
	if err != nil {
		return nil, err
	}
	log.Get().WithField("addr", addr).Info("Starting gateway udp listener")
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}

	b := &Backend{
		log:          log.Get(),
		skipCRCCheck: skipCRCCheck,
		conn:         conn,
		rxChan:       make(chan *types.UplinkMessage),
		statsChan:    make(chan *types.StatusMessage),
		udpSendChan:  make(chan udpPacket),
		gateways: gateways{
			gateways: make(map[lorawan.EUI64]gateway),
			onNew:    onNew,
			onDelete: onDelete,
		},
	}

	go func() {
		for {
			if err := b.gateways.cleanup(); err != nil {
				b.log.Errorf("Gateways cleanup failed: %s", err)
			}
			time.Sleep(time.Minute)
		}
	}()

	go func() {
		b.wg.Add(1)
		err := b.readPackets()
		if !b.closed {
			b.log.WithError(err).Fatal("Error in readPackets")
		}
		b.wg.Done()
	}()

	go func() {
		b.wg.Add(1)
		err := b.sendPackets()
		if !b.closed {
			b.log.WithError(err).Fatal("Error in sendPackets")
		}
		b.wg.Done()
	}()

	return b, nil
}

// Close closes the backend.
func (b *Backend) Close() error {
	b.log.Info("Closing gateway backend")
	b.closed = true
	close(b.udpSendChan)
	if err := b.conn.Close(); err != nil {
		return err
	}
	b.log.Info("Handling last packets")
	b.wg.Wait()
	return nil
}

// RXPacketChan returns the channel containing the received RX packets.
func (b *Backend) RXPacketChan() chan *types.UplinkMessage {
	return b.rxChan
}

// StatsChan returns the channel containg the received gateway stats.
func (b *Backend) StatsChan() chan *types.StatusMessage {
	return b.statsChan
}

// Send sends the given packet to the gateway.
func (b *Backend) Send(txPacket *types.DownlinkMessage) error {
	gw, err := b.gateways.get(getMac(txPacket.GatewayID))
	if err != nil {
		return err
	}
	txpk, err := newTXPKFromTXPacket(txPacket)
	if err != nil {
		return err
	}
	pullResp := PullRespPacket{
		ProtocolVersion: gw.protocolVersion,
		Payload: PullRespPayload{
			TXPK: txpk,
		},
	}
	bytes, err := pullResp.MarshalBinary()
	if err != nil {
		return fmt.Errorf("JSON marshal PullRespPacket error: %s", err)
	}
	b.udpSendChan <- udpPacket{
		data: bytes,
		addr: gw.addr,
	}
	return nil
}

func (b *Backend) readPackets() error {
	buf := make([]byte, 65507) // max udp data size
	for {
		i, addr, err := b.conn.ReadFromUDP(buf)
		if err != nil {
			return fmt.Errorf("Read from udp error: %s", err)
		}
		data := make([]byte, i)
		copy(data, buf[:i])
		go func(data []byte) {
			if err := b.handlePacket(addr, data); err != nil {
				b.log.WithFields(log.Fields{
					"data_base64": base64.StdEncoding.EncodeToString(data),
					"addr":        addr,
				}).Errorf("Could not handle packet: %s", err)
			}
		}(data)
	}
}

func (b *Backend) sendPackets() error {
	for p := range b.udpSendChan {
		pt, err := GetPacketType(p.data)
		if err != nil {
			b.log.WithFields(log.Fields{
				"addr":        p.addr,
				"data_base64": base64.StdEncoding.EncodeToString(p.data),
			}).Error("Unknown packet type")
			continue
		}

		if p.addr.Port < 1 || p.addr.Port > 65535 {
			b.log.WithFields(log.Fields{
				"addr": p.addr,
			}).Error("Not sending to invalid udp port number")
			continue
		}

		b.log.WithFields(log.Fields{
			"addr":             p.addr,
			"type":             pt,
			"protocol_version": p.data[0],
		}).Debug("Sending udp packet to gateway")

		if _, err := b.conn.WriteToUDP(p.data, p.addr); err != nil {
			return err
		}
	}
	return nil
}

func (b *Backend) handlePacket(addr *net.UDPAddr, data []byte) error {
	pt, err := GetPacketType(data)
	if err != nil {
		return err
	}
	b.log.WithFields(log.Fields{
		"addr":             addr,
		"type":             pt,
		"protocol_version": data[0],
	}).Debug("Received udp packet from gateway")

	switch pt {
	case PushData:
		return b.handlePushData(addr, data)
	case PullData:
		return b.handlePullData(addr, data)
	case TXACK:
		return b.handleTXACK(addr, data)
	default:
		return fmt.Errorf("Unknown packet type: %s", pt)
	}
}

func (b *Backend) handlePullData(addr *net.UDPAddr, data []byte) error {
	var p PullDataPacket
	if err := p.UnmarshalBinary(data); err != nil {
		return err
	}
	ack := PullACKPacket{
		ProtocolVersion: p.ProtocolVersion,
		RandomToken:     p.RandomToken,
	}
	bytes, err := ack.MarshalBinary()
	if err != nil {
		return err
	}

	err = b.gateways.set(p.GatewayMAC, gateway{
		addr:            addr,
		lastSeen:        time.Now().UTC(),
		protocolVersion: p.ProtocolVersion,
	})
	if err != nil {
		return err
	}

	b.udpSendChan <- udpPacket{
		addr: addr,
		data: bytes,
	}
	return nil
}

func (b *Backend) handlePushData(addr *net.UDPAddr, data []byte) error {
	var p PushDataPacket
	if err := p.UnmarshalBinary(data); err != nil {
		return err
	}

	// ack the packet
	ack := PushACKPacket{
		ProtocolVersion: p.ProtocolVersion,
		RandomToken:     p.RandomToken,
	}
	bytes, err := ack.MarshalBinary()
	if err != nil {
		return err
	}
	b.udpSendChan <- udpPacket{
		addr: addr,
		data: bytes,
	}

	// gateway stats
	if p.Payload.Stat != nil {
		b.handleStat(addr, p.GatewayMAC, *p.Payload.Stat)
	}

	// rx packets
	for _, rxpk := range p.Payload.RXPK {
		if err := b.handleRXPacket(addr, p.GatewayMAC, rxpk); err != nil {
			return err
		}
	}
	return nil
}

func (b *Backend) handleStat(addr *net.UDPAddr, mac lorawan.EUI64, stat Stat) {
	gwStats := newGatewayStatsPacket(mac, stat)
	gwStats.Message.Ip = append(gwStats.Message.Ip, addr.IP.String())
	b.log.WithFields(log.Fields{
		"addr": addr,
		"mac":  mac,
	}).Debug("stat packet received")
	b.statsChan <- gwStats
}

func (b *Backend) handleRXPacket(addr *net.UDPAddr, mac lorawan.EUI64, rxpk RXPK) error {
	logFields := log.Fields{
		"addr": addr,
		"mac":  mac,
		"data": rxpk.Data,
	}
	b.log.WithFields(logFields).Debug("rxpk packet received")

	// decode packet
	rxPacket, err := newRXPacketFromRXPK(mac, rxpk)
	if err != nil {
		return err
	}
	rxPacket.Message.Trace = rxPacket.Message.Trace.WithEvent(trace.ReceiveEvent, "backend", "packet-forwarder")
	b.rxChan <- rxPacket
	return nil
}

func (b *Backend) handleTXACK(addr *net.UDPAddr, data []byte) error {
	var p TXACKPacket
	if err := p.UnmarshalBinary(data); err != nil {
		return err
	}
	var errBool bool

	logFields := log.Fields{
		"mac":          p.GatewayMAC,
		"random_token": p.RandomToken,
	}
	if p.Payload != nil {
		if p.Payload.TXPKACK.Error != "NONE" {
			errBool = true
		}
		logFields["error"] = p.Payload.TXPKACK.Error
	}

	if errBool {
		b.log.WithFields(logFields).Error("tx ack received")
	} else {
		b.log.WithFields(logFields).Debug("tx ack received")
	}

	return nil
}

// newGatewayStatsPacket transforms a Semtech Stat packet into a StatusMessage.
func newGatewayStatsPacket(mac lorawan.EUI64, stat Stat) *types.StatusMessage {
	var gps *pb_gateway.GPSMetadata
	if stat.Lati != 0 || stat.Long != 0 || stat.Alti != 0 {
		gps = &pb_gateway.GPSMetadata{
			Latitude:  float32(stat.Lati),
			Longitude: float32(stat.Long),
			Altitude:  stat.Alti,
		}
	}

	gatewayTime := time.Time(stat.Time)
	if gatewayTime.Before(time.Now().Add(-24 * time.Hour)) {
		// Gateway time is definitely invalid if longer than 24 hours ago
		gatewayTime = time.Unix(0, 0)
	}

	status := &types.StatusMessage{
		GatewayID: getID(mac),
		Message: &pb_gateway.Status{
			Time:         gatewayTime.UnixNano(),
			Gps:          gps,
			RxIn:         uint32(stat.RXNb),
			RxOk:         uint32(stat.RXOK),
			TxIn:         uint32(stat.DWNb),
			TxOk:         uint32(stat.TXNb),
			Platform:     stat.Pfrm,
			ContactEmail: stat.Mail,
			Description:  stat.Desc,
		},
	}

	if stat.FPGA != 0 || stat.DSP != 0 || stat.HAL != "" {
		status.Message.Platform += fmt.Sprintf("(FPGA %d, DSP %d, HAL %s)", stat.FPGA, stat.DSP, stat.HAL)
	}

	if stat.Temp != 0 {
		status.Message.Os = &pb_gateway.Status_OSMetrics{
			Temperature: float32(stat.Temp),
		}
	}

	return status
}

// newRXPacketFromRXPK transforms a Semtech packet into an UplinkMessage.
func newRXPacketFromRXPK(mac lorawan.EUI64, rxpk RXPK) (*types.UplinkMessage, error) {
	datr, err := newDataRateFromDatR(rxpk.DatR)
	if err != nil {
		return nil, fmt.Errorf("Could not get DataRate from DatR: %s", err)
	}

	modulation := pb_lorawan.Modulation_LORA
	dataRate := ""
	if datr.Modulation == band.FSKModulation {
		modulation = pb_lorawan.Modulation_FSK
	} else {
		dataRate = fmt.Sprintf("SF%dBW%d", datr.SpreadFactor, datr.Bandwidth)
	}

	b, err := base64.StdEncoding.DecodeString(rxpk.Data)
	if err != nil {
		return nil, fmt.Errorf("Could not base64 decode data: %s", err)
	}

	// For multi-antenna gateways, the LSNR and RSSI are those of the best reception
	for _, sig := range rxpk.RSig {
		if sig.LSNR > rxpk.LSNR {
			rxpk.LSNR = sig.LSNR
			rxpk.RSSI = sig.RSSIC
			continue
		}
		if sig.LSNR == rxpk.LSNR && sig.RSSIC > rxpk.RSSI {
			rxpk.RSSI = sig.RSSIC
		}
	}

	gatewayTime := time.Time(rxpk.Time)
	if gatewayTime.Before(time.Now().Add(-24 * time.Hour)) {
		// Gateway time is definitely invalid if longer than 24 hours ago
		gatewayTime = time.Unix(0, 0)
	}

	rxPacket := &types.UplinkMessage{
		GatewayID: getID(mac),
		Message: &pb_router.UplinkMessage{
			Payload: b,
			ProtocolMetadata: &pb_protocol.RxMetadata{
				Protocol: &pb_protocol.RxMetadata_Lorawan{
					Lorawan: &pb_lorawan.Metadata{
						Modulation: modulation,
						BitRate:    uint32(datr.BitRate),
						DataRate:   dataRate,
						CodingRate: rxpk.CodR,
					},
				},
			},
			GatewayMetadata: &pb_gateway.RxMetadata{
				GatewayId: getID(mac),
				Timestamp: rxpk.Tmst,
				Time:      gatewayTime.UnixNano(),
				RfChain:   uint32(rxpk.RFCh),
				Channel:   uint32(rxpk.Chan),
				Frequency: uint64(rxpk.Freq * 1000000),
				Rssi:      float32(rxpk.RSSI),
				Snr:       float32(rxpk.LSNR),
			},
		},
	}

	return rxPacket, nil
}

// newTXPKFromTXPacket transforms a DownlinkMessage into a Semtech compatible packet.
func newTXPKFromTXPacket(txPacket *types.DownlinkMessage) (TXPK, error) {
	protocol := txPacket.Message.GetProtocolConfiguration().GetLorawan()
	gateway := txPacket.Message.GetGatewayConfiguration()

	datr := DatR{
		LoRa: protocol.DataRate,
		FSK:  protocol.BitRate,
	}

	txpk := TXPK{
		Imme: gateway.Timestamp == 0,
		Tmst: gateway.Timestamp,
		Freq: float64(gateway.Frequency) / 1000000,
		Powe: uint8(gateway.Power),
		Modu: protocol.Modulation.String(),
		DatR: datr,
		CodR: protocol.CodingRate,
		IPol: gateway.PolarizationInversion,
		Size: uint16(len(txPacket.Message.Payload)),
		Data: base64.StdEncoding.EncodeToString(txPacket.Message.Payload),
	}

	if protocol.Modulation == pb_lorawan.Modulation_FSK {
		txpk.FDev = uint16(protocol.BitRate / 2)
	}

	return txpk, nil
}

func newDataRateFromDatR(d DatR) (band.DataRate, error) {
	var dr band.DataRate

	if d.LoRa != "" {
		// parse e.g. SF12BW250 into separate variables
		match := loRaDataRateRegex.FindStringSubmatch(d.LoRa)
		if len(match) != 3 {
			return dr, errors.New("Could not parse LoRa data rate")
		}

		// cast variables to ints
		sf, err := strconv.Atoi(match[1])
		if err != nil {
			return dr, fmt.Errorf("Could not convert spreading factor to int: %s", err)
		}
		bw, err := strconv.Atoi(match[2])
		if err != nil {
			return dr, fmt.Errorf("Could not convert bandwith to int: %s", err)
		}

		dr.Modulation = band.LoRaModulation
		dr.SpreadFactor = sf
		dr.Bandwidth = bw
		return dr, nil
	}

	if d.FSK != 0 {
		dr.Modulation = band.FSKModulation
		dr.BitRate = int(d.FSK)
		return dr, nil
	}

	return dr, errors.New("Could not convert DatR to DataRate, DatR is empty / modulation unknown")
}

func newDatRfromDataRate(d band.DataRate) DatR {
	if d.Modulation == band.LoRaModulation {
		return DatR{
			LoRa: fmt.Sprintf("SF%dBW%d", d.SpreadFactor, d.Bandwidth),
		}
	}

	return DatR{
		FSK: uint32(d.BitRate),
	}
}
