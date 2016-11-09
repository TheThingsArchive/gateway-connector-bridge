// Copyright © 2016 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package ttn

import (
	"errors"
	"sync"
	"time"

	"github.com/TheThingsNetwork/gateway-connector-bridge/bridge/types"
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
	client     router.GatewayClient
	lastActive time.Time
}

// Router side of the bridge
type Router struct {
	config    RouterConfig
	Ctx       log.Interface
	client    *router.Client
	tokenFunc func(string) string
	gateways  map[string]*gatewayConn
	mu        sync.Mutex
}

func (r *Router) getGateway(gatewayID string) router.GatewayClient {
	r.mu.Lock()
	defer r.mu.Unlock()
	if gtw, ok := r.gateways[gatewayID]; ok {
		gtw.lastActive = time.Now()
		return gtw.client
	}
	r.gateways[gatewayID] = &gatewayConn{
		client: r.client.ForGateway(gatewayID, func() string {
			return r.tokenFunc(gatewayID)
		}),
		lastActive: time.Now(),
	}
	return r.gateways[gatewayID].client
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
	client, err := router.NewClient(announcement)
	if err != nil {
		return err
	}
	r.client = client
	return nil
}

// Disconnect from the TTN Router
func (r *Router) Disconnect() {
	r.client.Close()
}

// PublishUplink publishes uplink messages to the TTN Router
func (r *Router) PublishUplink(message *types.UplinkMessage) error {
	return r.getGateway(message.GatewayID).SendUplink(message.Message)
}

// PublishStatus publishes status messages to the TTN Router
func (r *Router) PublishStatus(message *types.StatusMessage) error {
	return r.getGateway(message.GatewayID).SendGatewayStatus(message.Message)
}

// SubscribeDownlink handles downlink messages for the given gateway ID
func (r *Router) SubscribeDownlink(gatewayID string) (<-chan *types.DownlinkMessage, error) {
	// TODO: wait for https://github.com/TheThingsNetwork/ttn/issues/352 to be resolved
	return nil, errors.New("Not implemented")
}
