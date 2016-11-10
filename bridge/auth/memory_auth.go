// Copyright © 2016 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package auth

import (
	"sync"
	"time"
)

type memoryGateway struct {
	key          string
	token        string
	tokenExpires time.Time
	sync.Mutex
}

// Memory implements the authentication interface with an in-memory backend
type Memory struct {
	gateways map[string]*memoryGateway
	mu       sync.RWMutex
	Exchanger
}

// NewMemory returns a new authentication interface with an in-memory backend
func NewMemory() Interface {
	return &Memory{
		gateways: make(map[string]*memoryGateway),
	}
}

// SetToken sets the access token for a gateway
func (m *Memory) SetToken(gatewayID string, token string, expires time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if gtw, ok := m.gateways[gatewayID]; ok {
		gtw.Lock()
		defer gtw.Unlock()
		gtw.token = token
		gtw.tokenExpires = expires
	} else {
		m.gateways[gatewayID] = &memoryGateway{
			token:        token,
			tokenExpires: expires,
		}
	}
	return nil
}

// SetKey sets the access key for a gateway
func (m *Memory) SetKey(gatewayID string, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if gtw, ok := m.gateways[gatewayID]; ok {
		gtw.Lock()
		defer gtw.Unlock()
		gtw.key = key
	} else {
		m.gateways[gatewayID] = &memoryGateway{
			key: key,
		}
	}
	return nil
}

// GetToken returns an access token for the gateway
func (m *Memory) GetToken(gatewayID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	gtw, ok := m.gateways[gatewayID]
	if !ok {
		return "", ErrGatewayNotFound
	}
	gtw.Lock()
	defer gtw.Unlock()
	if gtw.token != "" && gtw.tokenExpires.After(time.Now()) {
		return gtw.token, nil
	}
	if gtw.key != "" && m.Exchanger != nil {
		token, expires, err := m.Exchange(gatewayID, gtw.key)
		if err != nil {
			return "", err
		}
		gtw.token = token
		gtw.tokenExpires = expires
		return token, nil
	}
	return "", ErrGatewayNoValidToken
}

// SetExchanger sets the component that will exchange access keys for access tokens
func (m *Memory) SetExchanger(e Exchanger) {
	m.Exchanger = e
}
