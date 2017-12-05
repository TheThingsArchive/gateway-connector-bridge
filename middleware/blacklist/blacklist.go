// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package blacklist

import (
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"sync"

	"github.com/TheThingsNetwork/gateway-connector-bridge/middleware"
	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v2"
)

type blacklistedItem struct {
	Gateway string `yaml:"gateway"`
	IP      string `yaml:"ip"`
}

// NewBlacklist returns a middleware that filters traffic from blacklisted gateways
func NewBlacklist(lists ...string) (b *Blacklist, err error) {
	b = &Blacklist{
		lists:    make(map[string][]blacklistedItem),
		ipLookup: make(map[string]bool),
		idLookup: make(map[string]bool),
	}
	b.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	for _, location := range lists {
		b.addList(location) // ignore errors for mvp
	}
	b.FetchRemotes()
	go func() {
		for e := range b.watcher.Events {
			if e.Op&fsnotify.Write == fsnotify.Write {
				b.read(e.Name) // ignore errors for mvp
			}
		}
	}()
	return b, nil
}

// Blacklist middleware
type Blacklist struct {
	watcher *fsnotify.Watcher
	urls    []string

	mu       sync.RWMutex
	lists    map[string][]blacklistedItem
	ipLookup map[string]bool
	idLookup map[string]bool
}

func (b *Blacklist) addList(location string) error {
	url, err := url.Parse(location)
	if err != nil {
		return err
	}
	switch url.Scheme {
	case "", "file":
		return b.addFile(location)
	case "http", "https":
		return b.addURL(url)
	}
	return errors.New("blacklist: unknown list type")
}

func (b *Blacklist) addFile(filename string) (err error) {
	filename, err = filepath.Abs(filename)
	if err != nil {
		return err
	}
	if err = b.watcher.Add(filename); err != nil {
		return err
	}
	return b.read(filename)
}

func (b *Blacklist) addURL(url *url.URL) error {
	b.urls = append(b.urls, url.String())
	return nil
}

// FetchRemotes fetches remote blacklists
func (b *Blacklist) FetchRemotes() error {
	for _, url := range b.urls {
		b.fetch(url) // ignore errors for mvp
	}
	return nil
}

// Close the blacklist watcher
func (b *Blacklist) Close() {
	b.watcher.Close()
}

func (b *Blacklist) read(filename string) error {
	contents, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	var blacklist []blacklistedItem
	err = yaml.Unmarshal(contents, &blacklist)
	if err != nil {
		return err
	}
	b.mu.Lock()
	b.lists[filename] = blacklist
	b.updateLookup()
	b.mu.Unlock()
	return nil
}

func (b *Blacklist) fetch(location string) error {
	resp, err := http.Get(location)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var blacklist []blacklistedItem
	err = yaml.Unmarshal(body, &blacklist)
	if err != nil {
		return err
	}
	b.mu.Lock()
	b.lists[location] = blacklist
	b.updateLookup()
	b.mu.Unlock()
	return nil
}

func (b *Blacklist) updateLookup() {
	var n int
	for _, blacklist := range b.lists {
		n += len(blacklist)
	}
	b.ipLookup = make(map[string]bool, n)
	b.idLookup = make(map[string]bool, n)
	for _, blacklist := range b.lists {
		for _, item := range blacklist {
			if item.IP != "" {
				b.ipLookup[item.IP] = true
			}
			if item.Gateway != "" {
				b.idLookup[item.Gateway] = true
			}
		}
	}
}

// Blacklist errors
var (
	ErrBlacklistedID = errors.New("blacklist: Gateway ID is blacklisted")
	ErrBlacklistedIP = errors.New("blacklist: Gateway IP is blacklisted")
)

func (b *Blacklist) check(id string, ip net.Addr) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.idLookup[id] {
		return ErrBlacklistedID
	}
	if ip != nil {
		ip, _, err := net.SplitHostPort(ip.String())
		if err != nil {
			return err
		}
		if b.ipLookup[ip] {
			return ErrBlacklistedIP
		}
	}
	return nil
}

// HandleUplink blocks uplink messages from blacklisted gateways
func (b *Blacklist) HandleUplink(_ middleware.Context, msg *types.UplinkMessage) error {
	return b.check(msg.GatewayID, msg.GatewayAddr)
}

// HandleStatus blocks status messages from blacklisted gateways
func (b *Blacklist) HandleStatus(ctx middleware.Context, msg *types.StatusMessage) error {
	return b.check(msg.GatewayID, msg.GatewayAddr)
}
