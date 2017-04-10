package pktfwd

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/brocaar/lorawan"
)

func newSourceLocks(withPort bool, cacheTime time.Duration) *sourceLocks {
	c := &sourceLocks{
		withPort:  withPort,
		cacheTime: cacheTime,
		sources:   make(map[lorawan.EUI64]source),
	}
	if cacheTime > 0 {
		go func() {
			for range time.Tick(10 * cacheTime) {
				c.mu.Lock()
				var toDelete []lorawan.EUI64
				for mac, source := range c.sources {
					if time.Since(source.lastSeen) > cacheTime {
						toDelete = append(toDelete, mac)
					}
				}
				for _, mac := range toDelete {
					delete(c.sources, mac)
				}
				c.mu.Unlock()
			}
		}()
	}
	return c
}

type sourceLocks struct {
	withPort  bool
	cacheTime time.Duration

	mu      sync.Mutex
	sources map[lorawan.EUI64]source
}

func (c *sourceLocks) Set(mac lorawan.EUI64, addr *net.UDPAddr) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if existing, ok := c.sources[mac]; ok {
		if time.Since(existing.lastSeen) < c.cacheTime {
			if c.withPort && existing.addr.Port != addr.Port {
				return fmt.Errorf("security: inconsistent port for gateway %s: %d (expected %d)", mac, addr.Port, existing.addr.Port)
			}
			if !existing.addr.IP.Equal(addr.IP) {
				return fmt.Errorf("security: inconsistent IP address for gateway %s: %s (expected %s)", mac, addr.IP, existing.addr)
			}
		}
	}
	c.sources[mac] = source{time.Now(), addr}
	return nil
}

type source struct {
	lastSeen time.Time
	addr     *net.UDPAddr
}
