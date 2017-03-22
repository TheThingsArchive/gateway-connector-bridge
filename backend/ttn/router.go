// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package ttn

import (
	"context"
	"sync"
	"time"

	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
	"github.com/TheThingsNetwork/ttn/api/auth"
	"github.com/TheThingsNetwork/ttn/api/discovery"
	"github.com/TheThingsNetwork/ttn/api/pool"
	"github.com/TheThingsNetwork/ttn/api/router"
	"github.com/TheThingsNetwork/ttn/api/trace"
	"github.com/apex/log"
	"google.golang.org/grpc"
)

func init() {
	grpc.EnableTracing = false
}

// RouterConfig contains configuration for the TTN Router
type RouterConfig struct {
	DiscoveryServer string
	RouterID        string
}

// Router side of the bridge
type Router struct {
	config RouterConfig
	Ctx    log.Interface
	conn   *grpc.ClientConn
	client *router.Client

	pool *pool.Pool

	mu       sync.Mutex
	gateways map[string]*gatewayConn
}

// New sets up a new TTN Router
func New(config RouterConfig, ctx log.Interface, tokenFunc func(string) string) (*Router, error) {
	router := &Router{
		config:   config,
		Ctx:      ctx.WithField("Connector", "TTN Router"),
		pool:     pool.NewPool(context.Background(), append(pool.DefaultDialOptions, auth.WithTokenFunc(tokenFunc).DialOption())...),
		gateways: make(map[string]*gatewayConn),
	}
	return router, nil
}

// Connect to the TTN Router
func (r *Router) Connect() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	discovery, err := discovery.NewClient(r.config.DiscoveryServer, &discovery.Announcement{
		ServiceName: "bridge",
	}, func() string { return "" })
	if err != nil {
		return err
	}
	announcement, err := discovery.Get("router", r.config.RouterID)
	if err != nil {
		return err
	}
	if announcement.GetCertificate() == "" {
		r.conn, err = announcement.Dial(nil)
	} else {
		r.conn, err = announcement.Dial(r.pool)
	}
	if err != nil {
		return err
	}
	r.client = router.NewClient(router.DefaultClientConfig)
	r.client.AddServer(r.config.RouterID, r.conn)
	return nil
}

// Disconnect from the TTN Router and clean up gateway connections
func (r *Router) Disconnect() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.gateways = make(map[string]*gatewayConn)
	r.pool.Close()
	return nil
}

type gatewayConn struct {
	stream     router.GenericStream
	lastActive time.Time
}

func (r *Router) getGateway(gatewayID string) *gatewayConn {
	r.mu.Lock()
	defer r.mu.Unlock()
	if gtw, ok := r.gateways[gatewayID]; ok {
		gtw.lastActive = time.Now()
		return gtw
	}
	r.gateways[gatewayID] = &gatewayConn{
		stream:     r.client.NewGatewayStreams(gatewayID, ""),
		lastActive: time.Now(),
	}
	return r.gateways[gatewayID]
}

// CleanupGateway cleans up gateway clients that are no longer needed
func (r *Router) CleanupGateway(gatewayID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if gtw, ok := r.gateways[gatewayID]; ok {
		gtw.stream.Close()
		delete(r.gateways, gatewayID)
	}
}

// PublishUplink publishes uplink messages to the TTN Router
func (r *Router) PublishUplink(message *types.UplinkMessage) error {
	message.Message.Trace = message.Message.Trace.WithEvent(trace.ForwardEvent, "backend", "ttn")
	r.getGateway(message.GatewayID).stream.Uplink(message.Message)
	return nil
}

// PublishStatus publishes status messages to the TTN Router
func (r *Router) PublishStatus(message *types.StatusMessage) error {
	r.getGateway(message.GatewayID).stream.Status(message.Message)
	return nil
}

// SubscribeDownlink handles downlink messages for the given gateway ID
func (r *Router) SubscribeDownlink(gatewayID string) (<-chan *types.DownlinkMessage, error) {
	downlink := make(chan *types.DownlinkMessage)

	gtw := r.getGateway(gatewayID)
	ctx := r.Ctx.WithField("GatewayID", gatewayID)

	go func() {
		for in := range gtw.stream.Downlink() {
			ctx.Debug("Downlink message received")
			in.Trace = in.Trace.WithEvent(trace.ReceiveEvent, "backend", "ttn")
			downlink <- &types.DownlinkMessage{GatewayID: gatewayID, Message: in}
		}
		close(downlink)
	}()

	return downlink, nil
}

// UnsubscribeDownlink should unsubscribe from downlink, but in practice just disconnects the entire gateway
func (r *Router) UnsubscribeDownlink(gatewayID string) error {
	r.CleanupGateway(gatewayID)
	return nil
}
