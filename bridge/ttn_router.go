// Copyright © 2016 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package bridge

import (
	"errors"
	"sync"
	"time"

	"github.com/TheThingsNetwork/ttn/api/discovery"
	"github.com/TheThingsNetwork/ttn/api/router"
	"github.com/apex/log"
)

// SetupTTNRouter sets up the TTN Router
func (b *Bridge) SetupTTNRouter(config TTNRouterConfig) error {
	router := new(TTNRouter)

	router.Ctx = b.Ctx.WithField("Connector", "TTN Router")
	router.config = config
	router.gateways = make(map[string]*gatewayConn)
	router.tokenFunc = b.gatewayToken

	b.ttnRouter = router
	return nil
}

// TTNRouterConfig contains configuration for the TTN Router
type TTNRouterConfig struct {
	DiscoveryServer string
	RouterID        string
}

type gatewayConn struct {
	client     router.GatewayClient
	lastActive time.Time
}

// TTNRouter side of the bridge
type TTNRouter struct {
	config    TTNRouterConfig
	Ctx       log.Interface
	client    *router.Client
	tokenFunc func(string) string
	gateways  map[string]*gatewayConn
	mu        sync.Mutex
}

func (r *TTNRouter) getGateway(gatewayID string) router.GatewayClient {
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
func (r *TTNRouter) CleanupGateway(gatewayID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if gtw, ok := r.gateways[gatewayID]; ok {
		gtw.client.Close()
		delete(r.gateways, gatewayID)
	}
}

// Connect to the TTN Router
func (r *TTNRouter) Connect() error {
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
func (r *TTNRouter) Disconnect() {
	r.client.Close()
}

// PublishUplink publishes uplink messages to the TTN Router
func (r *TTNRouter) PublishUplink(message *UplinkMessage) error {
	return r.getGateway(message.gatewayID).SendUplink(message.message)
}

// PublishStatus publishes status messages to the TTN Router
func (r *TTNRouter) PublishStatus(message *StatusMessage) error {
	return r.getGateway(message.gatewayID).SendGatewayStatus(message.message)
}

// SubscribeDownlink handles downlink messages for the given gateway ID
func (r *TTNRouter) SubscribeDownlink(gatewayID string) (<-chan *DownlinkMessage, error) {
	// TODO: wait for https://github.com/TheThingsNetwork/ttn/issues/352 to be resolved
	return nil, errors.New("Not implemented")
}
