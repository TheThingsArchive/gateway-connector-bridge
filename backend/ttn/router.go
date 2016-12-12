// Copyright © 2016 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package ttn

import (
	"sync"
	"time"

	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
	"github.com/TheThingsNetwork/go-utils/log/apex"
	"github.com/TheThingsNetwork/ttn/api/discovery"
	"github.com/TheThingsNetwork/ttn/api/router"
	"github.com/apex/log"
	"google.golang.org/grpc"
)

// New sets up a new TTN Router
func New(config RouterConfig, ctx log.Interface, tokenFunc func(string) string) (*Router, error) {
	router := new(Router)
	router.Ctx = ctx.WithField("Connector", "TTN Router")
	router.config = config
	router.gateways = make(map[string]*gatewayConn)
	router.tokenFunc = tokenFunc
	grpc.EnableTracing = false
	return router, nil
}

// RouterConfig contains configuration for the TTN Router
type RouterConfig struct {
	DiscoveryServer string
	RouterID        string
}

type gatewayConn struct {
	client     router.RouterClientForGateway
	uplink     router.UplinkStream
	status     router.GatewayStatusStream
	downlink   router.DownlinkStream
	lastActive time.Time
}

func (g *gatewayConn) Close() {
	if g.uplink != nil {
		g.uplink.Close()
	}
	if g.downlink != nil {
		g.downlink.Close()
	}
	if g.status != nil {
		g.status.Close()
	}
	g.client.Close()
}

// Router side of the bridge
type Router struct {
	config       RouterConfig
	Ctx          log.Interface
	routerConn   *grpc.ClientConn
	routerClient router.RouterClient
	tokenFunc    func(string) string
	gateways     map[string]*gatewayConn
	mu           sync.Mutex
}

func (r *Router) getGateway(gatewayID string) *gatewayConn {
	r.mu.Lock()
	defer r.mu.Unlock()
	if gtw, ok := r.gateways[gatewayID]; ok {
		gtw.lastActive = time.Now()
		return gtw
	}
	gateway := &gatewayConn{
		client:     router.NewRouterClientForGateway(r.routerClient, gatewayID, r.tokenFunc(gatewayID)),
		lastActive: time.Now(),
	}
	gateway.client.SetLogger(apex.Wrap(r.Ctx.WithField("GatewayID", gatewayID)))
	gateway.uplink = router.NewMonitoredUplinkStream(gateway.client)
	gateway.status = router.NewMonitoredGatewayStatusStream(gateway.client)
	r.gateways[gatewayID] = gateway
	return r.gateways[gatewayID]
}

// CleanupGateway cleans up gateway clients that are no longer needed
func (r *Router) CleanupGateway(gatewayID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if gtw, ok := r.gateways[gatewayID]; ok {
		gtw.client.Close()
		delete(r.gateways, gatewayID)
	}
}

// Connect to the TTN Router
func (r *Router) Connect() error {
	discovery, err := discovery.NewClient(r.config.DiscoveryServer, &discovery.Announcement{
		ServiceName: "bridge",
	}, func() string { return "" })
	if err != nil {
		return err
	}
	defer discovery.Close()
	announcement, err := discovery.Get("router", r.config.RouterID)
	if err != nil {
		return err
	}
	r.routerConn, err = announcement.Dial()
	if err != nil {
		return err
	}
	r.routerClient = router.NewRouterClient(r.routerConn)
	return nil
}

// Disconnect from the TTN Router
func (r *Router) Disconnect() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, gtw := range r.gateways {
		gtw.Close()
		delete(r.gateways, id)
	}
	r.routerConn.Close()
	return nil
}

// PublishUplink publishes uplink messages to the TTN Router
func (r *Router) PublishUplink(message *types.UplinkMessage) error {
	return r.getGateway(message.GatewayID).uplink.Send(message.Message)
}

// PublishStatus publishes status messages to the TTN Router
func (r *Router) PublishStatus(message *types.StatusMessage) error {
	return r.getGateway(message.GatewayID).status.Send(message.Message)
}

// SubscribeDownlink handles downlink messages for the given gateway ID
func (r *Router) SubscribeDownlink(gatewayID string) (<-chan *types.DownlinkMessage, error) {
	downlink := make(chan *types.DownlinkMessage)

	gtw := r.getGateway(gatewayID)
	ctx := r.Ctx.WithField("GatewayID", gatewayID)

	gtw.downlink = router.NewMonitoredDownlinkStream(gtw.client)

	go func() {
		for in := range gtw.downlink.Channel() {
			ctx.Debug("Downlink message received")
			downlink <- &types.DownlinkMessage{GatewayID: gatewayID, Message: in}
		}
		close(downlink)
	}()

	return downlink, nil
}

// UnsubscribeDownlink unsubscribes from downlink messages
func (r *Router) UnsubscribeDownlink(gatewayID string) error {
	gtw := r.getGateway(gatewayID)
	if gtw.downlink != nil {
		gtw.downlink.Close()
	}
	return nil
}
