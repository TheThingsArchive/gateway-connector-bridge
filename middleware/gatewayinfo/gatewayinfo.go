// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package gatewayinfo

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/TheThingsNetwork/gateway-connector-bridge/middleware"
	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
	"github.com/TheThingsNetwork/go-account-lib/account"
	"github.com/TheThingsNetwork/go-utils/log"
	"github.com/TheThingsNetwork/ttn/api/gateway"
	redis "gopkg.in/redis.v5"
)

// RequestInterval sets how often the account server may be queried
var RequestInterval = 50 * time.Millisecond

// RequestBurst sets the burst of requests to the account server
var RequestBurst = 50

// NewPublic returns a middleware that injects public gateway information
func NewPublic(accountServer string) *Public {
	p := &Public{
		log:       log.Get(),
		account:   account.New(accountServer),
		info:      make(map[string]*info),
		available: make(chan struct{}, RequestBurst),
	}
	for i := 0; i < RequestBurst; i++ {
		p.available <- struct{}{}
	}
	go func() {
		for range time.Tick(RequestInterval) {
			select {
			case p.available <- struct{}{}:
			default:
			}
		}
	}()
	return p
}

// WithRedis initializes the Redis store for persistence between restarts
func (p *Public) WithRedis(client *redis.Client, prefix string) (*Public, error) {
	p.redisPrefix = prefix

	// Initialize the data from the store
	var allKeys []string
	var cursor uint64
	for {
		keys, next, err := client.Scan(cursor, p.redisKey("*"), 0).Result()
		if err != nil {
			return nil, err
		}
		allKeys = append(allKeys, keys...)
		cursor = next
		if cursor == 0 {
			break
		}
	}

	for _, key := range allKeys {
		res, err := client.Get(key).Result()
		if err != nil {
			continue
		}
		var gateway account.Gateway
		err = json.Unmarshal([]byte(res), &gateway)
		if err != nil {
			continue
		}
		gatewayID := strings.TrimPrefix(key, p.redisKey(""))
		p.set(gatewayID, gateway)
	}

	// Now set the client
	p.redisClient = client

	return p, nil
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

	redisClient *redis.Client
	redisPrefix string

	mu   sync.Mutex
	info map[string]*info

	available chan struct{}
}

func (p *Public) redisKey(gatewayID string) string {
	return fmt.Sprintf("%s:%s", p.redisPrefix, gatewayID)
}

type info struct {
	lastUpdated time.Time
	err         error
	gateway     account.Gateway
}

func (p *Public) fetch(gatewayID string) error {
	<-p.available
	gateway, err := p.account.FindGateway(gatewayID)
	if err != nil {
		p.setErr(gatewayID, err)
		return err
	}
	p.set(gatewayID, gateway)
	return nil
}

func (p *Public) setErr(gatewayID string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if gtw, ok := p.info[gatewayID]; ok {
		gtw.lastUpdated = time.Now()
		gtw.err = err
	} else {
		p.info[gatewayID] = &info{
			lastUpdated: time.Now(),
			err:         err,
		}
	}
}

func (p *Public) set(gatewayID string, gateway account.Gateway) {
	log := p.log.WithField("GatewayID", gatewayID)
	p.mu.Lock()
	log.Debug("Setting public gateway info")
	p.info[gatewayID] = &info{
		lastUpdated: time.Now(),
		gateway:     gateway,
	}
	p.mu.Unlock()
	if p.redisClient != nil {
		data, _ := json.Marshal(gateway)
		if err := p.redisClient.Set(p.redisKey(gatewayID), string(data), p.expire).Err(); err != nil {
			log.WithError(err).Warn("Could not set public Gateway information in Redis")
		}
	}
}

func (p *Public) get(gatewayID string) (gateway account.Gateway, err error) {
	if gatewayID == "" {
		return
	}
	log := p.log.WithField("GatewayID", gatewayID)
	p.mu.Lock()
	defer p.mu.Unlock()
	info, ok := p.info[gatewayID]
	if ok {
		if p.expire == 0 || time.Since(info.lastUpdated) < p.expire {
			return info.gateway, info.err
		}
		info.lastUpdated = time.Now()
	}
	go func() {
		err := p.fetch(gatewayID)
		if err != nil {
			log.WithError(err).Warn("Could not get public Gateway information")
		} else {
			log.Debug("Got public Gateway information")
		}
	}()
	return gateway, nil
}

func (p *Public) unset(gatewayID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.info, gatewayID)
}

// HandleConnect fetches public gateway information in the background when a ConnectMessage is received
func (p *Public) HandleConnect(ctx middleware.Context, msg *types.ConnectMessage) error {
	p.get(msg.GatewayID)
	return nil
}

// HandleDisconnect cleans up
func (p *Public) HandleDisconnect(ctx middleware.Context, msg *types.DisconnectMessage) error {
	go p.unset(msg.GatewayID)
	return nil
}

// HandleUplink inserts metadata if set in info, but not present in message
func (p *Public) HandleUplink(ctx middleware.Context, msg *types.UplinkMessage) error {
	info, err := p.get(msg.GatewayID)
	if err != nil {
		msg.Message.Trace = msg.Message.Trace.WithEvent("unable to get gateway info", "error", err)
	}
	meta := msg.Message.GetGatewayMetadata()
	if meta.Gps == nil && info.AntennaLocation != nil {
		msg.Message.Trace = msg.Message.Trace.WithEvent("injecting gateway location")
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
	info, _ := p.get(msg.GatewayID)
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
