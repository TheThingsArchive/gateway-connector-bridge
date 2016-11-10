// Copyright © 2016 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package auth

import (
	"time"

	redis "gopkg.in/redis.v5"
)

// Redis implements the authentication interface with a Redis backend
type Redis struct {
	prefix string
	client *redis.Client
	Exchanger
}

// DefaultRedisPrefix is used as prefix when no prefix is given
var DefaultRedisPrefix = "gateway:"

var redisKey = struct {
	token        string
	key          string
	tokenExpires string
}{
	token:        "token",
	key:          "key",
	tokenExpires: "token_expires",
}

// NewRedis returns a new authentication interface with a redis backend
func NewRedis(client *redis.Client, prefix string) Interface {
	if prefix == "" {
		prefix = DefaultRedisPrefix
	}
	return &Redis{
		client: client,
		prefix: prefix,
	}
}

// SetToken sets the access token for a gateway
func (r *Redis) SetToken(gatewayID string, token string, expires time.Time) error {
	data := map[string]string{
		redisKey.token: token,
	}
	if expires.IsZero() {
		data[redisKey.tokenExpires] = ""
	} else {
		data[redisKey.tokenExpires] = expires.Format(time.RFC3339)
	}
	return r.client.HMSet(r.prefix+gatewayID, data).Err()
}

// SetKey sets the access key for a gateway
func (r *Redis) SetKey(gatewayID string, key string) error {
	return r.client.HSet(r.prefix+gatewayID, "key", key).Err()
}

// Delete gateway key and token
func (r *Redis) Delete(gatewayID string) error {
	return r.client.Del(r.prefix + gatewayID).Err()
}

// GetToken returns an access token for the gateway
func (r *Redis) GetToken(gatewayID string) (string, error) {
	res, err := r.client.HGetAll(r.prefix + gatewayID).Result()
	if err == redis.Nil || len(res) == 0 {
		return "", ErrGatewayNotFound
	}
	if err != nil {
		return "", err
	}
	if token, ok := res[redisKey.token]; ok && token != "" {
		if expires, ok := res[redisKey.tokenExpires]; ok {
			expires, err := time.Parse(time.RFC3339, expires)
			if err != nil {
				return "", err
			}
			if expires.After(time.Now()) {
				return token, nil
			}
		}
	}
	if key, ok := res[redisKey.key]; ok && key != "" && r.Exchanger != nil {
		token, expires, err := r.Exchange(gatewayID, key)
		if err != nil {
			return "", err
		}
		if err := r.SetToken(gatewayID, token, expires); err != nil {
			// TODO: Print warning
		}
		return token, nil
	}
	return "", ErrGatewayNoValidToken
}

// SetExchanger sets the component that will exchange access keys for access tokens
func (r *Redis) SetExchanger(e Exchanger) {
	r.Exchanger = e
}
