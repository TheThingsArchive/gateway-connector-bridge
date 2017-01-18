// Copyright © 2016 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package auth

import (
	"errors"
	"time"
)

// Interface for gateway authentication
type Interface interface {
	SetToken(gatewayID string, token string, expires time.Time) error
	SetKey(gatewayID string, key string) error

	ValidateKey(gatewayID string, key string) error

	Delete(gatewayID string) error

	// GetToken returns the access token for a gateway; it exchanges the key for an access token if necessary
	GetToken(gatewayID string) (token string, err error)
	SetExchanger(Exchanger)
}

// ErrGatewayNotFound is returned when a gateway was not found
var ErrGatewayNotFound = errors.New("Gateway not found")

// ErrGatewayNoValidToken is returned when a gateway does not have a valid access token
var ErrGatewayNoValidToken = errors.New("Gateway does not have a valid access token")
