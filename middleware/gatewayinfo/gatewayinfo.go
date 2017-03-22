// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package gatewayinfo

import (
	"strings"
	"sync"
	"time"

	"github.com/TheThingsNetwork/gateway-connector-bridge/middleware"
	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
	"github.com/TheThingsNetwork/go-account-lib/account"
	"github.com/TheThingsNetwork/go-utils/log"
	"github.com/TheThingsNetwork/ttn/api/gateway"
)

// NewPublic returns a middleware that injects public gateway information
func NewPublic(accountServer string) *Public {
	return &Public{
		log:     log.Get(),
		account: account.New(accountServer),
		info:    make(map[string]*info),
	}
}

// WithExpire adds an expiration to gateway information. Information is re-fetched if expired
func (p *Public) WithExpire(duration time.Duration) *Public {
	p.expire = duration
	return p
}

// Public gateway information will be injected
type Public struct {
	log     log.Interface
	account *account.Account
	expire  time.Duration

	mu   sync.Mutex
	info map[string]*info
}

type info struct {
	lastUpdated time.Time
	gateway     account.Gateway
}

func (p *Public) fetch(gatewayID string) error {
	gateway, err := p.account.FindGateway(gatewayID)
	if err != nil {
		return err
	}
	p.set(gatewayID, gateway)
	return nil
}

func (p *Public) set(gatewayID string, gateway account.Gateway) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.info[gatewayID] = &info{
		lastUpdated: time.Now(),
		gateway:     gateway,
	}
}

func (p *Public) get(gatewayID string) (gateway account.Gateway) {
	p.mu.Lock()
	defer p.mu.Unlock()
	info, ok := p.info[gatewayID]
	if !ok {
		return gateway
	}
	if p.expire != 0 && time.Since(info.lastUpdated) > p.expire {
		info.lastUpdated = time.Now()
		go p.fetch(gatewayID)
	}
	return info.gateway
}

func (p *Public) unset(gatewayID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.info, gatewayID)
}

// HandleConnect fetches public gateway information in the background when a ConnectMessage is received
func (p *Public) HandleConnect(ctx middleware.Context, msg *types.ConnectMessage) error {
	go func() {
		log := p.log.WithField("GatewayID", msg.GatewayID)
		err := p.fetch(msg.GatewayID)
		if err != nil {
			log.WithError(err).Warn("Could not get public Gateway information")
		} else {
			log.Debug("Got public Gateway information")
		}
	}()
	return nil
}

// HandleDisconnect cleans up
func (p *Public) HandleDisconnect(ctx middleware.Context, msg *types.DisconnectMessage) error {
	go p.unset(msg.GatewayID)
	return nil
}

// HandleUplink inserts metadata if set in info, but not present in message
func (p *Public) HandleUplink(ctx middleware.Context, msg *types.UplinkMessage) error {
	info := p.get(msg.GatewayID)
	meta := msg.Message.GetGatewayMetadata()
	if meta.Gps == nil && info.AntennaLocation != nil {
		meta.Gps = &gateway.GPSMetadata{
			Latitude:  float32(info.AntennaLocation.Latitude),
			Longitude: float32(info.AntennaLocation.Longitude),
			Altitude:  int32(info.AntennaLocation.Altitude),
		}
	}
	return nil
}

// HandleStatus inserts metadata if set in info, but not present in message
func (p *Public) HandleStatus(ctx middleware.Context, msg *types.StatusMessage) error {
	info := p.get(msg.GatewayID)
	if msg.Message.Gps == nil && info.AntennaLocation != nil {
		msg.Message.Gps = &gateway.GPSMetadata{
			Latitude:  float32(info.AntennaLocation.Latitude),
			Longitude: float32(info.AntennaLocation.Longitude),
			Altitude:  int32(info.AntennaLocation.Altitude),
		}
	}
	if msg.Message.FrequencyPlan == "" && info.FrequencyPlan != "" {
		msg.Message.FrequencyPlan = info.FrequencyPlan
	}
	if msg.Message.Platform == "" {
		platform := []string{}
		if info.Attributes.Brand != nil {
			platform = append(platform, *info.Attributes.Brand)
		}
		if info.Attributes.Model != nil {
			platform = append(platform, *info.Attributes.Model)
		}
		msg.Message.Platform = strings.Join(platform, " ")
	}
	if msg.Message.Description == "" && info.Attributes.Description != nil {
		msg.Message.Description = *info.Attributes.Description
	}
	return nil
}
