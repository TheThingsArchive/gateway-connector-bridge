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

	pb_gateway "github.com/TheThingsNetwork/api/gateway"
	pb_protocol "github.com/TheThingsNetwork/api/protocol"
	pb_lorawan "github.com/TheThingsNetwork/api/protocol/lorawan"
	pb_router "github.com/TheThingsNetwork/api/router"
	"github.com/TheThingsNetwork/api/trace"
	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
	"github.com/TheThingsNetwork/go-utils/log"
	"github.com/TheThingsNetwork/go-utils/pseudorandom"
	"github.com/brocaar/lorawan"
	"github.com/brocaar/lorawan/band"
)

var errGatewayDoesNotExist = errors.New("gateway does not exist")
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
	session time.Duration
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
		if c.gateways[mac].lastSeen.Before(time.Now().Add(-1 * c.session)) {
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

	pushSources *sourceLocks
	pullSources *sourceLocks
	ackSources  *sourceLocks
}

// NewBackend creates a new backend.
func NewBackend(config Config, onNew func(lorawan.EUI64) error, onDelete func(lorawan.EUI64) error, skipCRCCheck bool) (*Backend, error) {
	addr, err := net.ResolveUDPAddr("udp", config.Bind)
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
			session:  config.Session,
			gateways: make(map[lorawan.EUI64]gateway),
			onNew:    onNew,
			onDelete: onDelete,
		},
	}

	if config.LockIP {
		b.pushSources = newSourceLocks(config.LockPort, config.Session)
		b.pullSources = newSourceLocks(config.LockPort, config.Session)
		b.ackSources = newSourceLocks(config.LockPort, config.Session)
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
		RandomToken:     uint16(pseudorandom.Intn(1 << 16)),
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
		_, err := GetPacketType(p.data)
		if err != nil {
			b.log.WithFields(log.Fields{
				"addr":        p.addr,
				"data_base64": base64.StdEncoding.EncodeToString(p.data),
			}).Error("Not sending unknown packet to gateway")
			continue
		}

		if p.addr.Port < 1 || p.addr.Port > 65535 {
			b.log.WithFields(log.Fields{
				"addr": p.addr,
			}).Error("Not sending to invalid udp port number")
			continue
		}

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

	switch pt {
	case PushData:
		return b.handlePushData(addr, data)
	case PullData:
		return b.handlePullData(addr, data)
	case TXACK:
		return b.handleTXACK(addr, data)
	default:
		b.log.WithFields(log.Fields{
			"addr":             addr,
			"type":             pt,
			"protocol_version": data[0],
		}).Debug("Received unknown packet from gateway")
		return fmt.Errorf("Unknown packet type: %s", pt)
	}
}

func (b *Backend) handlePullData(addr *net.UDPAddr, data []byte) error {
	var p PullDataPacket
	if err := p.UnmarshalBinary(data); err != nil {
		return err
	}
	if b.pullSources != nil {
		if err := b.pullSources.Set(p.GatewayMAC, addr); err != nil {
			return err
		}
	}

	b.log.WithFields(log.Fields{
		"addr": addr,
		"mac":  p.GatewayMAC,
	}).Debug("Handle PullData")

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
	if b.pushSources != nil {
		if err := b.pushSources.Set(p.GatewayMAC, addr); err != nil {
			return err
		}
	}

	b.log.WithFields(log.Fields{
		"addr": addr,
		"mac":  p.GatewayMAC,
		"rxpk": len(p.Payload.RXPK),
		"stat": p.Payload.Stat != nil,
	}).Debug("Handle PushData")

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
	gwStats.GatewayAddr = addr
	gwStats.Message.IP = append(gwStats.Message.IP, addr.IP.String())
	b.statsChan <- gwStats
}

func (b *Backend) handleRXPacket(addr *net.UDPAddr, mac lorawan.EUI64, rxpk RXPK) error {
	rxPacket, err := newRXPacketFromRXPK(mac, rxpk)
	if err != nil {
		return err
	}
	rxPacket.GatewayAddr = addr
	rxPacket.Message.Trace = rxPacket.Message.Trace.WithEvent(trace.ReceiveEvent, "backend", "packet-forwarder")
	b.rxChan <- rxPacket
	return nil
}

func (b *Backend) handleTXACK(addr *net.UDPAddr, data []byte) error {
	var p TXACKPacket
	if err := p.UnmarshalBinary(data); err != nil {
		return err
	}
	if b.ackSources != nil {
		if err := b.ackSources.Set(p.GatewayMAC, addr); err != nil {
			return err
		}
	}
	var errBool bool

	logFields := log.Fields{
		"addr": addr,
		"mac":  p.GatewayMAC,
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
	var gps *pb_gateway.LocationMetadata
	if stat.Lati != 0 || stat.Long != 0 || stat.Alti != 0 {
		gps = &pb_gateway.LocationMetadata{
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

	bootTime := time.Time(stat.Boot)
	if bootTime.IsZero() || bootTime.Before(time.Unix(0, 0)) {
		bootTime = time.Unix(0, 0)
	}

	status := &types.StatusMessage{
		GatewayID: getID(mac),
		Message: &pb_gateway.Status{
			Time:         gatewayTime.UnixNano(),
			BootTime:     bootTime.UnixNano(),
			Location:     gps,
			Platform:     stat.Pfrm,
			ContactEmail: stat.Mail,
			Description:  stat.Desc,
			FPGA:         stat.FPGA,
			DSP:          stat.DSP,
			HAL:          stat.HAL,
			RxIn:         stat.RXNb,
			RxOk:         stat.RXOK,
			TxIn:         stat.DWNb,
			TxOk:         stat.TXNb,
			LmOk:         stat.LMOK,
			LmSt:         stat.LMST,
			LmNw:         stat.LMNW,
			LPPS:         stat.LPPS,
		},
	}

	if stat.FPGA != 0 || stat.DSP != 0 || stat.HAL != "" {
		status.Message.Platform += fmt.Sprintf("(FPGA %d, DSP %d, HAL %s)", stat.FPGA, stat.DSP, stat.HAL)
	}

	if stat.Temp != 0 {
		status.Message.OS = &pb_gateway.Status_OSMetrics{
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

	b, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(rxpk.Data, "="))
	if err != nil {
		return nil, fmt.Errorf("Could not base64 decode data: %s", err)
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
			ProtocolMetadata: pb_protocol.RxMetadata{
				Protocol: &pb_protocol.RxMetadata_LoRaWAN{
					LoRaWAN: &pb_lorawan.Metadata{
						Modulation: modulation,
						BitRate:    uint32(datr.BitRate),
						DataRate:   dataRate,
						CodingRate: rxpk.CodR,
					},
				},
			},
			GatewayMetadata: pb_gateway.RxMetadata{
				GatewayID: getID(mac),
				Timestamp: rxpk.Tmst,
				Time:      gatewayTime.UnixNano(),
				RfChain:   uint32(rxpk.RFCh),
				Channel:   uint32(rxpk.Chan),
				Frequency: uint64(rxpk.Freq * 1000000),
				RSSI:      float32(rxpk.RSSI),
				SNR:       float32(rxpk.LSNR),
			},
		},
	}

	// Use LSNR and RSSI from RSig if present
	if len(rxpk.RSig) > 0 {
		rxPacket.Message.GatewayMetadata.SNR = float32(rxpk.RSig[0].LSNR)
		rxPacket.Message.GatewayMetadata.RSSI = float32(rxpk.RSig[0].RSSIS)
		for _, sig := range rxpk.RSig {
			if float32(sig.LSNR) > rxPacket.Message.GatewayMetadata.SNR {
				rxPacket.Message.GatewayMetadata.SNR = float32(sig.LSNR)
				rxPacket.Message.GatewayMetadata.RSSI = float32(sig.RSSIS)
			} else if float32(sig.LSNR) == rxPacket.Message.GatewayMetadata.SNR && float32(sig.RSSIS) > rxPacket.Message.GatewayMetadata.RSSI {
				rxPacket.Message.GatewayMetadata.RSSI = float32(sig.RSSIS)
			}
			antenna := &pb_gateway.RxMetadata_Antenna{
				Antenna:     uint32(sig.Ant),
				Channel:     uint32(sig.Chan),
				ChannelRSSI: float32(sig.RSSIC),
				RSSI:        float32(sig.RSSIS),
				RSSIStandardDeviation: float32(sig.RSSISD),
				SNR:             float32(sig.LSNR),
				FrequencyOffset: int64(sig.FOff),
				FineTime:        sig.FTime,
			}
			if eTime, err := base64.StdEncoding.DecodeString(sig.ETime); err == nil && len(eTime) > 0 {
				antenna.EncryptedTime = eTime
			}
			rxPacket.Message.GatewayMetadata.Antennas = append(rxPacket.Message.GatewayMetadata.Antennas, antenna)
		}
	}

	return rxPacket, nil
}

// newTXPKFromTXPacket transforms a DownlinkMessage into a Semtech compatible packet.
func newTXPKFromTXPacket(txPacket *types.DownlinkMessage) (TXPK, error) {
	protocol := txPacket.Message.ProtocolConfiguration.GetLoRaWAN()
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
		NCRC: protocol.Modulation == pb_lorawan.Modulation_LORA,
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
