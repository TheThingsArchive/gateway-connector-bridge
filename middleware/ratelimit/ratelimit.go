// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package ratelimit

import (
	"errors"
	"fmt"
	"sync"
	"time"

	redis "gopkg.in/redis.v5"

	"github.com/TheThingsNetwork/gateway-connector-bridge/middleware"
	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
	"github.com/TheThingsNetwork/go-utils/log"
	"github.com/TheThingsNetwork/go-utils/rate"
)

// Limits per minute
type Limits struct {
	Uplink   int
	Downlink int
	Status   int
}

// NewRateLimit returns a middleware that rate-limits uplink, downlink and status messages per gateway
func NewRateLimit(conf Limits) *RateLimit {
	return &RateLimit{
		log:      log.Get(),
		limits:   conf,
		gateways: make(map[string]*limits),
	}
}

// NewRedisRateLimit returns a middleware that rate-limits uplink, downlink and status messages per gateway
func NewRedisRateLimit(client *redis.Client, conf Limits) *RateLimit {
	l := NewRateLimit(conf)
	l.client = client
	return l
}

// RateLimit uplink, downlink and status messages per gateway
type RateLimit struct {
	log    log.Interface
	limits Limits
	client *redis.Client

	mu       sync.RWMutex
	gateways map[string]*limits
}

func (l *RateLimit) newLimits(gatewayID string) *limits {
	limits := new(limits)

	if l.limits.Uplink != 0 {
		var uplink rate.Counter
		if l.client != nil {
			uplink = rate.NewRedisCounter(l.client, fmt.Sprintf("ratelimit:%s:uplink", gatewayID), time.Second, time.Minute)
		} else {
			uplink = rate.NewCounter(time.Second, time.Minute)
		}
		limits.uplink = rate.NewLimiter(uplink, time.Minute, uint64(l.limits.Uplink))
	}

	if l.limits.Downlink != 0 {
		var downlink rate.Counter
		if l.client != nil {
			downlink = rate.NewRedisCounter(l.client, fmt.Sprintf("ratelimit:%s:downlink", gatewayID), time.Second, time.Minute)
		} else {
			downlink = rate.NewCounter(time.Second, time.Minute)
		}
		limits.downlink = rate.NewLimiter(downlink, time.Minute, uint64(l.limits.Downlink))
	}

	if l.limits.Status != 0 {
		var status rate.Counter
		if l.client != nil {
			status = rate.NewRedisCounter(l.client, fmt.Sprintf("ratelimit:%s:status", gatewayID), time.Second, time.Minute)
		} else {
			status = rate.NewCounter(time.Second, time.Minute)
		}
		limits.status = rate.NewLimiter(status, time.Minute, uint64(l.limits.Status))
	}

	return limits

}

type limits struct {
	uplink   rate.Limiter
	downlink rate.Limiter
	status   rate.Limiter
}

// HandleConnect initializes the rate limiter
func (l *RateLimit) HandleConnect(ctx middleware.Context, msg *types.ConnectMessage) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.gateways[msg.GatewayID] = l.newLimits(msg.GatewayID)
	return nil
}

// HandleDisconnect cleans up
func (l *RateLimit) HandleDisconnect(ctx middleware.Context, msg *types.DisconnectMessage) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.gateways, msg.GatewayID)
	return nil
}

func (l *RateLimit) get(gatewayID string) *limits {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if limits, ok := l.gateways[gatewayID]; ok {
		return limits
	}
	return nil
}

// ErrRateLimited is returned if the rate limit has been reached
var ErrRateLimited = errors.New("rate limit reached")

// HandleUplink rate-limits status messages
func (l *RateLimit) HandleUplink(ctx middleware.Context, msg *types.UplinkMessage) error {
	if limits := l.get(msg.GatewayID); limits != nil && limits.uplink != nil {
		limit, err := limits.uplink.Limit()
		if err != nil {
			return err
		}
		if limit {
			return ErrRateLimited
		}
	}
	return nil
}

// HandleDownlink rate-limits downlink messages
func (l *RateLimit) HandleDownlink(ctx middleware.Context, msg *types.DownlinkMessage) error {
	if limits := l.get(msg.GatewayID); limits != nil && limits.downlink != nil {
		limit, err := limits.downlink.Limit()
		if err != nil {
			return err
		}
		if limit {
			return ErrRateLimited
		}
	}
	return nil
}

// HandleStatus rate-limits status messages
func (l *RateLimit) HandleStatus(ctx middleware.Context, msg *types.StatusMessage) error {
	if limits := l.get(msg.GatewayID); limits != nil && limits.status != nil {
		limit, err := limits.status.Limit()
		if err != nil {
			return err
		}
		if limit {
			return ErrRateLimited
		}
	}
	return nil
}
