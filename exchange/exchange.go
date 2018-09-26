// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package exchange

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/TheThingsNetwork/api/gateway"
	"github.com/TheThingsNetwork/api/protocol"
	"github.com/TheThingsNetwork/api/protocol/lorawan"
	"github.com/TheThingsNetwork/api/trace"
	"github.com/TheThingsNetwork/gateway-connector-bridge/auth"
	"github.com/TheThingsNetwork/gateway-connector-bridge/backend"
	"github.com/TheThingsNetwork/gateway-connector-bridge/middleware"
	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
	"github.com/apex/log"
	"github.com/deckarep/golang-set"
	"github.com/spf13/viper"
)

// Exchange routes messages between northbound backends (servers that are up the chain)
// and southbound backends gateways or servers that are down the chain.
//
// When a connection message is received on the southbound backend:
// - Uplink messages are routed from the southbound backends to the northbound backends
// - Downlink messages are routed from the northbound backends to the southbound backends
// - Status messages are routed from the southbound backends to the northbound backends
// until a disconnection message is received on the southbound backend
type Exchange struct {
	ctx  log.Interface
	mu   sync.Mutex
	done chan struct{}

	id   string
	auth auth.Interface

	middleware middleware.Chain

	northboundBackends []backend.Northbound
	southboundBackends []backend.Southbound
	backendInit        sync.WaitGroup

	northboundDone map[string][]chan struct{}
	southboundDone map[string][]chan struct{}
	doneLock       sync.Mutex

	connect    chan *types.ConnectMessage
	disconnect chan *types.DisconnectMessage
	uplink     chan *types.UplinkMessage
	status     chan *types.StatusMessage
	downlink   chan *types.DownlinkMessage

	killWhenIdleFor time.Duration
	idleWatchdog    *time.Timer

	gateways gatewayState
}

// New initializes a new Exchange
func New(ctx log.Interface, killWhenIdleFor time.Duration) *Exchange {
	e := &Exchange{
		ctx:             ctx,
		done:            make(chan struct{}),
		northboundDone:  make(map[string][]chan struct{}),
		southboundDone:  make(map[string][]chan struct{}),
		connect:         make(chan *types.ConnectMessage),
		disconnect:      make(chan *types.DisconnectMessage),
		uplink:          make(chan *types.UplinkMessage),
		status:          make(chan *types.StatusMessage),
		downlink:        make(chan *types.DownlinkMessage),
		gateways:        mapset.NewSet(),
		killWhenIdleFor: killWhenIdleFor,
	}
	info.WithLabelValues(viper.GetString("buildDate"), viper.GetString("gitCommit"), viper.GetString("id"), viper.GetString("version")).Set(1)
	if killWhenIdleFor > 0 {
		e.idleWatchdog = time.AfterFunc(killWhenIdleFor, func() {
			ctx.Fatalf("Exchange was idle for more than %v", killWhenIdleFor)
		})
	}
	return e
}

// SetID sets the id of this bridge
func (b *Exchange) SetID(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.id = id
	trace.SetComponent("bridge", id)
}

// SetMiddleware sets the middleware to be executed
func (b *Exchange) SetMiddleware(chain middleware.Chain) {
	b.middleware = chain
}

// SetAuth sets the authentication component
func (b *Exchange) SetAuth(auth auth.Interface) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.auth = auth
}

// AddNorthbound adds a new northbound backend (server that is up the chain)
func (b *Exchange) AddNorthbound(backend ...backend.Northbound) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.northboundBackends = append(b.northboundBackends, backend...)
}

func (b *Exchange) subscribeNorthbound(backend backend.Northbound) {
	if err := backend.Connect(); err != nil {
		b.ctx.WithError(err).Errorf("Could not set up backend %v", backend)
	}
	b.backendInit.Done()
	for {
		select {
		case <-b.done:
			return
		}
	}
}

// AddSouthbound adds a new southbound backend (gateway or server that is down the chain)
func (b *Exchange) AddSouthbound(backend ...backend.Southbound) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.southboundBackends = append(b.southboundBackends, backend...)
}

func (b *Exchange) subscribeSouthbound(backend backend.Southbound) {
	if err := backend.Connect(); err != nil {
		b.ctx.WithError(err).Errorf("Could not set up backend %v", backend)
	}
	connect, err := backend.SubscribeConnect()
	if err != nil {
		b.ctx.WithError(err).Errorf("Could not subscribe to connect from backend %v", backend)
	}
	disconnect, err := backend.SubscribeDisconnect()
	if err != nil {
		b.ctx.WithError(err).Errorf("Could not subscribe to disconnect from backend %v", backend)
	}
	b.backendInit.Done()
loop:
	for {
		select {
		case <-b.done:
			break loop
		case connectMessage := <-connect:
			b.connect <- connectMessage
		case disconnectMessage := <-disconnect:
			b.disconnect <- disconnectMessage
		}
	}
	if err := backend.UnsubscribeConnect(); err != nil {
		b.ctx.WithError(err).Errorf("Could not unsubscribe from connect on backend %v", backend)
	}
	if err := backend.UnsubscribeDisconnect(); err != nil {
		b.ctx.WithError(err).Errorf("Could not unsubscribe from disconnect on backend %v", backend)
	}
}

// ConnectGateway force-connects gateways with the given IDs
func (b *Exchange) ConnectGateway(gatewayID ...string) {
	for _, gatewayID := range gatewayID {
		if !b.gateways.Add(gatewayID) {
			continue
		}
		if gatewayID != "" {
			for _, backend := range b.northboundBackends {
				go b.activateNorthbound(backend, gatewayID)
			}
		}
		for _, backend := range b.southboundBackends {
			go b.activateSouthbound(backend, gatewayID)
		}
		connectedGateways.Inc()
	}
}

var errClosedChannel = errors.New("closed channel")

func (b *Exchange) handleChannels() (err error) {
	errCh := make(chan error)
	defer close(errCh)
	doneCh := make(chan struct{})
	defer close(doneCh)
	var curMsg string
	watchdog := newWatchdog(func() {
		errCh <- fmt.Errorf("handleChannels stuck in %s", curMsg)
	})
	defer watchdog.Stop()
	go func() {
		var curStart time.Time
		var curCtx log.Interface
		start := func(ctx log.Interface, msg string) {
			watchdog.Kick()
			curStart = time.Now()
			curCtx = ctx
			curMsg = msg
		}
		var err error
		for {
			if curMsg != "" && err == nil {
				curCtx.WithField("Duration", time.Since(curStart)).Infof("Routed %s", curMsg)
			}
			err = nil
			select {
			case <-doneCh:
				return
			case <-time.After(watchdogExpire - 100*time.Millisecond):
				start(b.ctx, "")
			case connectMessage, ok := <-b.connect:
				if !ok {
					err = errClosedChannel
					continue
				}
				gatewayID := strings.ToLower(connectMessage.GatewayID)
				ctx := b.ctx.WithField("GatewayID", gatewayID)
				start(ctx, "connect")
				if b.auth != nil {
					if connectMessage.Key != "" {
						ctx.Debug("Got access key")
						if err := b.auth.SetKey(gatewayID, connectMessage.Key); err != nil {
							ctx.WithError(err).Warn("Could not set gateway key")
						}
					}
				}
				if !b.gateways.Add(gatewayID) {
					ctx.Debug("Got connect message from already-connected gateway")
					err = errors.New("Got connect message from already-connected gateway")
					continue
				}
				if err = b.middleware.Execute(middleware.NewContext(), connectMessage); err != nil {
					ctx.WithError(err).Warn("Error in middleware")
					continue
				}
				for _, backend := range b.northboundBackends {
					go b.activateNorthbound(backend, gatewayID)
				}
				for _, backend := range b.southboundBackends {
					go b.activateSouthbound(backend, gatewayID)
				}
				connectedGateways.Inc()
			case disconnectMessage, ok := <-b.disconnect:
				if !ok {
					err = errClosedChannel
					continue
				}
				gatewayID := strings.ToLower(disconnectMessage.GatewayID)
				ctx := b.ctx.WithField("GatewayID", gatewayID)
				start(ctx, "disconnect")
				if !b.gateways.Contains(gatewayID) {
					ctx.Debug("Got disconnect message from not-connected gateway")
					continue
				}
				if err = b.auth.ValidateKey(gatewayID, disconnectMessage.Key); err != nil {
					ctx.WithError(err).Warn("Got disconnect message with invalid Key")
					continue
				}
				if err = b.middleware.Execute(middleware.NewContext(), disconnectMessage); err != nil {
					ctx.WithError(err).Warn("Error in middleware")
					continue
				}
				b.deactivateNorthbound(gatewayID)
				b.deactivateSouthbound(gatewayID)
				b.gateways.Remove(gatewayID)
				connectedGateways.Dec()
			case uplinkMessage, ok := <-b.uplink:
				if !ok {
					err = errClosedChannel
					continue
				}
				ctx := b.ctx.WithFields(log.Fields{
					"GatewayID":   uplinkMessage.GatewayID,
					"GatewayAddr": uplinkMessage.GatewayAddr,
				})
				ctx = ctxWithMessageFields(ctx, uplinkMessage.Message)
				start(ctx, "uplink")
				if err = b.middleware.Execute(middleware.NewContext(), uplinkMessage); err != nil {
					ctx.WithError(err).Warn("Error in middleware")
					continue
				}
				uplinkMessage.Message.GatewayMetadata.GatewayID = uplinkMessage.GatewayID
				published := 0
				for _, backend := range b.northboundBackends {
					ctx := ctx.WithField("Backend", fmt.Sprintf("%T", backend))
					err := backend.PublishUplink(uplinkMessage)
					if err == nil {
						ctx.Debug("Published uplink")
						if b.killWhenIdleFor > 0 && b.idleWatchdog.Stop() {
							b.idleWatchdog.Reset(b.killWhenIdleFor)
						}
						published++
					} else {
						ctx.WithError(err).Debug("Did not publish uplink")
					}
				}
				if published > 0 {
					registerHandled(uplinkMessage.Message)
				} else {
					ctx.Warn("Uplink not accepted by any northbound backend")
					err = errors.New("Uplink not accepted by any northbound backend")
				}
			case downlinkMessage, ok := <-b.downlink:
				if !ok {
					err = errClosedChannel
					continue
				}
				ctx := b.ctx.WithFields(log.Fields{
					"GatewayID": downlinkMessage.GatewayID,
				})
				ctx = ctxWithMessageFields(ctx, downlinkMessage.Message)
				start(ctx, "downlink")
				if err = b.middleware.Execute(middleware.NewContext(), downlinkMessage); err != nil {
					ctx.WithError(err).Warn("Error in middleware")
					continue
				}
				published := 0
				for _, backend := range b.southboundBackends {
					ctx := ctx.WithField("Backend", fmt.Sprintf("%T", backend))
					err := backend.PublishDownlink(downlinkMessage)
					if err == nil {
						ctx.Debug("Published downlink")
						published++
					} else {
						ctx.WithError(err).Debug("Did not publish downlink")
					}
				}
				if published > 0 {
					registerHandled(downlinkMessage.Message)
				} else {
					ctx.Warn("Downlink not accepted by any southbound backend")
					err = errors.New("Downlink not accepted by any southbound backend")
				}
			case statusMessage, ok := <-b.status:
				if !ok {
					err = errClosedChannel
					continue
				}
				ctx := b.ctx.WithFields(log.Fields{"GatewayID": statusMessage.GatewayID, "GatewayAddr": statusMessage.GatewayAddr})
				start(ctx, "status")
				if err = b.middleware.Execute(middleware.NewContext(), statusMessage); err != nil {
					ctx.WithError(err).Warn("Error in middleware")
					continue
				}
				published := 0
				for _, backend := range b.northboundBackends {
					ctx := ctx.WithField("Backend", fmt.Sprintf("%T", backend))
					err := backend.PublishStatus(statusMessage)
					if err == nil {
						ctx.Debug("Published status")
						published++
					} else {
						ctx.WithError(err).Debug("Did not publish status")
					}
				}
				if published > 0 {
					registerStatus()
				} else {
					ctx.Warn("Status not accepted by any northbound backend")
					err = errors.New("Status not accepted by any northbound backend")
				}
			}
		}
	}()
	select {
	case <-b.done:
		return nil
	case err := <-errCh:
		return err
	}
}

func ctxWithMessageFields(ctx *log.Entry, m message) *log.Entry {
	fields := make(log.Fields)
	fields["PayloadSize"] = len(m.GetPayload())

	m.UnmarshalPayload()
	msg := m.GetMessage()
	if msg == nil {
		return ctx.WithFields(fields)
	}

	if msg := msg.GetLoRaWAN(); msg != nil {
		fields["MType"] = msg.MType.String()
		switch msg.MType {
		case lorawan.MType_JOIN_REQUEST:
			msg := msg.GetJoinRequestPayload()
			fields["AppEUI"] = msg.AppEUI
			fields["DevEUI"] = msg.DevEUI
		case lorawan.MType_UNCONFIRMED_UP, lorawan.MType_CONFIRMED_UP,
			lorawan.MType_UNCONFIRMED_DOWN, lorawan.MType_CONFIRMED_DOWN:
			msg := msg.GetMACPayload()
			fields["DevAddr"] = msg.DevAddr
			fields["FCnt"] = msg.FCnt
			fields["FPort"] = msg.FPort
		}
	}

	if msg, ok := m.(interface {
		GetProtocolConfiguration() protocol.TxConfiguration
		GetGatewayConfiguration() gateway.TxConfiguration
	}); ok {
		fields["Timestamp"] = msg.GetGatewayConfiguration().Timestamp
		fields["Frequency"] = msg.GetGatewayConfiguration().Frequency
		protocol := msg.GetProtocolConfiguration()
		if lorawan := (&protocol).GetLoRaWAN(); lorawan != nil {
			fields["DataRate"] = lorawan.DataRate
		}
	}

	if msg, ok := m.(interface {
		GetProtocolMetadata() protocol.RxMetadata
		GetGatewayMetadata() gateway.RxMetadata
	}); ok {
		fields["Timestamp"] = msg.GetGatewayMetadata().Timestamp
		fields["Frequency"] = msg.GetGatewayMetadata().Frequency
		protocol := msg.GetProtocolMetadata()
		if lorawan := (&protocol).GetLoRaWAN(); lorawan != nil {
			fields["DataRate"] = lorawan.DataRate
		}
	}

	return ctx.WithFields(fields)
}

func (b *Exchange) activateNorthbound(backend backend.Northbound, gatewayID string) {
	begin := time.Now()
	ctx := b.ctx.WithField("GatewayID", gatewayID).WithField("Backend", fmt.Sprintf("%T", backend))
	downlink, err := backend.SubscribeDownlink(gatewayID)
	if err != nil {
		ctx.WithError(err).Error("Could not subscribe to downlink")
	}
	done := make(chan struct{})
	b.doneLock.Lock()
	b.northboundDone[gatewayID] = append(b.northboundDone[gatewayID], done)
	b.doneLock.Unlock()
	ctx.WithField("Duration", time.Since(begin)).Debug("Activated northbound")
loop:
	for {
		select {
		case <-done:
			break loop
		case downlinkMessage, ok := <-downlink:
			if !ok {
				continue
			}
			b.downlink <- downlinkMessage
		}
	}
	if err := backend.UnsubscribeDownlink(gatewayID); err != nil {
		ctx.WithError(err).Error("Could not unsubscribe from downlink")
	}
	ctx.WithField("SessionDuration", time.Since(begin)).Debug("Deactivated northbound")
}

func (b *Exchange) activateSouthbound(backend backend.Southbound, gatewayID string) {
	begin := time.Now()
	ctx := b.ctx.WithField("GatewayID", gatewayID).WithField("Backend", fmt.Sprintf("%T", backend))
	uplink, err := backend.SubscribeUplink(gatewayID)
	if err != nil {
		ctx.WithError(err).Error("Could not subscribe to uplink")
	}
	status, err := backend.SubscribeStatus(gatewayID)
	if err != nil {
		ctx.WithError(err).Error("Could not subscribe to status")
	}
	done := make(chan struct{})
	b.doneLock.Lock()
	b.southboundDone[gatewayID] = append(b.southboundDone[gatewayID], done)
	b.doneLock.Unlock()
	ctx.WithField("Duration", time.Since(begin)).Debug("Activated southbound")
loop:
	for {
		select {
		case <-done:
			break loop
		case uplinkMessage, ok := <-uplink:
			if !ok {
				continue
			}
			b.uplink <- uplinkMessage
		case statusMessage, ok := <-status:
			if !ok {
				continue
			}
			b.status <- statusMessage
		}
	}
	if err := backend.UnsubscribeUplink(gatewayID); err != nil {
		ctx.WithError(err).Error("Could not unsubscribe from uplink")
	}
	if err := backend.UnsubscribeStatus(gatewayID); err != nil {
		ctx.WithError(err).Error("Could not unsubscribe from status")
	}
	ctx.WithField("SessionDuration", time.Since(begin)).Debug("Deactivated southbound")
}

func (b *Exchange) deactivateNorthbound(gatewayID string) {
	b.doneLock.Lock()
	defer b.doneLock.Unlock()
	if backends, ok := b.northboundDone[gatewayID]; ok {
		for _, done := range backends {
			close(done)
		}
		delete(b.northboundDone, gatewayID)
	}
}

func (b *Exchange) deactivateSouthbound(gatewayID string) {
	b.doneLock.Lock()
	defer b.doneLock.Unlock()
	if backends, ok := b.southboundDone[gatewayID]; ok {
		for _, done := range backends {
			close(done)
		}
		delete(b.southboundDone, gatewayID)
	}
}

// Start the Exchange
func (b *Exchange) Start(goroutines int, timeout time.Duration) (finishedWithinTimeout bool) {
	b.mu.Lock()
	for _, backend := range b.northboundBackends {
		b.backendInit.Add(1)
		go b.subscribeNorthbound(backend)
	}
	for _, backend := range b.southboundBackends {
		b.backendInit.Add(1)
		go b.subscribeSouthbound(backend)
	}
	c := make(chan struct{})
	go func() {
		defer close(c)
		b.backendInit.Wait()
	}()
	select {
	case <-c:
		finishedWithinTimeout = true
	case <-time.After(timeout):
		finishedWithinTimeout = false
	}
	for i := 0; i < goroutines; i++ {
		go func() {
			for {
				err := b.handleChannels()
				if err == nil {
					return
				}
				b.ctx.WithError(err).Error("Error in handleChannels")
			}
		}()
	}
	return
}

// Stop the Exchange
func (b *Exchange) Stop() {
	close(b.done) // This stops all new connections/disconnections
	b.doneLock.Lock()
	defer b.doneLock.Unlock()
	for _, backends := range b.northboundDone {
		for _, backend := range backends {
			close(backend)
		}
	}
	for _, backends := range b.southboundDone {
		for _, backend := range backends {
			close(backend)
		}
	}
	b.northboundDone = make(map[string][]chan struct{})
	b.southboundDone = make(map[string][]chan struct{})
	b.mu.Unlock()
}
