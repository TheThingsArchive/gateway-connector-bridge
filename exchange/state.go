// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package exchange

import redis "gopkg.in/redis.v5"

type gatewayState interface {
	// Adds an element to the set. Returns whether
	// the item was added.
	Add(i interface{}) bool

	// Returns whether the given items
	// are all in the set.
	Contains(i ...interface{}) bool

	// Remove a single element from the set.
	Remove(i interface{})

	// Returns the members of the set as a slice.
	ToSlice() []interface{}
}

// defaultRedisStateKey is used as key when no key is given
var defaultRedisStateKey = "gatewaystate"

// InitRedisState initializes Redis-backed connection state for the exchange and returns the state stored in the database
func (b *Exchange) InitRedisState(client *redis.Client, key string) (gatewayIDs []string) {
	if key == "" {
		key = defaultRedisStateKey
	}
	b.gateways = &gatewayStateWithRedisPersistence{
		gatewayState: b.gateways,
		client:       client,
		key:          key,
	}
	gatewayIDs, _ = client.SMembers(key).Result()
	return
}

type gatewayStateWithRedisPersistence struct {
	key    string
	client *redis.Client
	gatewayState
}

func (s *gatewayStateWithRedisPersistence) Add(i interface{}) bool {
	added := s.gatewayState.Add(i)
	if added {
		go s.client.SAdd(s.key, i).Result()
	}
	return added
}

func (s *gatewayStateWithRedisPersistence) Remove(i interface{}) {
	s.gatewayState.Remove(i)
	go s.client.SRem(s.key, i).Result()
}
