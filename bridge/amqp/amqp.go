// Copyright © 2016 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package amqp

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"sync"
	"time"

	"github.com/TheThingsNetwork/gateway-connector-bridge/bridge/types"
	"github.com/TheThingsNetwork/ttn/api/gateway"
	"github.com/TheThingsNetwork/ttn/api/router"
	"github.com/apex/log"
	"github.com/gogo/protobuf/proto"
	"github.com/streadway/amqp"
)

// New returns a new AMQP
func New(config Config, ctx log.Interface) (*AMQP, error) {
	amqp := new(AMQP)

	if config.ExchangeName == "" {
		config.ExchangeName = "amq.topic"
	}

	if config.QueuePrefix == "" {
		config.QueuePrefix = "bridge"
	}

	if config.ConsumerPrefix == "" {
		config.ConsumerPrefix = "bridge"
		if user, err := user.Current(); err == nil {
			config.ConsumerPrefix += "-" + user.Username
		}
		if hostname, err := os.Hostname(); err == nil {
			config.ConsumerPrefix += "@" + hostname
		}
	}

	amqp.ctx = ctx.WithField("Connector", "AMQP")
	amqp.config = config
	amqp.publish.ch = make(chan publishMessage, BufferSize)
	amqp.subscriptions = make(map[string]*subscription)
	amqp.connection.Add(1)

	return amqp, nil
}

// BufferSize indicates the maximum number of AMQP messages that should be buffered
var BufferSize = 10

// Routing Key formats for connect, disconnect, uplink, downlink and status messages
var (
	ConnectRoutingKeyFormat    = "connect"
	DisconnectRoutingKeyFormat = "disconnect"
	UplinkRoutingKeyFormat     = "%s.up"
	DownlinkRoutingKeyFormat   = "%s.down"
	StatusRoutingKeyFormat     = "%s.status"
)

// Config contains configuration for AMQP
type Config struct {
	Address        string
	Username       string
	Password       string
	VHost          string
	ExchangeName   string
	QueuePrefix    string
	ConsumerPrefix string
	TLSConfig      *tls.Config
}

func (c Config) url() (url string) {
	if c.TLSConfig != nil {
		url += "amqps://"
	} else {
		url += "amqp://"
	}
	if c.Username != "" {
		url += c.Username
		if c.Password != "" {
			url += ":" + c.Password
		}
		url += "@"
	}
	url += c.Address
	if c.VHost != "" {
		url += "/" + c.VHost
	}
	return
}

type publishMessage struct {
	routingKey string
	message    []byte
}

type subscribeMessage struct {
	routingKey string
	message    []byte
}

type subscription struct {
	channel      *amqp.Channel
	consumerName string
	setupOnce    sync.Once
	cancelOnce   *sync.Once
	sync.WaitGroup
}

func (s *subscription) cancel() (err error) {
	s.cancelOnce.Do(func() {
		err = s.channel.Cancel(s.consumerName, false)
	})
	return
}

// AMQP side of the bridge
type AMQP struct {
	config     Config
	ctx        log.Interface
	connection struct {
		*amqp.Connection
		sync.RWMutex
		sync.WaitGroup
		once sync.Once
	}
	publish struct {
		ch      chan publishMessage
		channel *amqp.Channel
		sync.Mutex
		once sync.Once
	}
	subscriptions    map[string]*subscription
	subscriptionLock sync.RWMutex
}

var (
	// ConnectRetries says how many times the client should retry a failed connection
	ConnectRetries = 10
	// ConnectRetryDelay says how long the client should wait between retries
	ConnectRetryDelay = time.Second
)

func (c *AMQP) connect() (err error) {
	var conn *amqp.Connection
	if c.config.TLSConfig != nil {
		conn, err = amqp.DialTLS(c.config.url(), c.config.TLSConfig)
	} else {
		conn, err = amqp.Dial(c.config.url())
	}
	c.connection.Lock()
	c.connection.Connection = conn
	c.connection.Unlock()
	c.connection.once.Do(func() {
		c.connection.Done()
	})
	if err != nil {
		return err
	}
	if err := c.setup(); err != nil {
		return err
	}
	return nil
}

func (c *AMQP) channel() (*amqp.Channel, error) {
	c.connection.Wait()
	c.connection.RLock()
	defer c.connection.RUnlock()
	return c.connection.Channel()
}

func (c *AMQP) setup() (err error) {
	ch, err := c.channel()
	if err != nil {
		return err
	}
	defer ch.Close()
	if err := ch.ExchangeDeclarePassive(c.config.ExchangeName, "topic", true, false, false, false, nil); err != nil {
		c.ctx.WithError(err).Warnf("Exchange %s does not exist, trying to create...", c.config.ExchangeName)
		ch, err := c.channel()
		if err != nil {
			return err
		}
		defer ch.Close()
		if err := ch.ExchangeDeclare(c.config.ExchangeName, "topic", true, false, false, false, nil); err != nil {
			return err
		}
	}
	return nil
}

// Connect to AMQP
func (c *AMQP) Connect() error {
	go c.autoReconnect()
	return nil
}

// AutoReconnect connects to AMQP and automatically reconnects when the connection is lost
func (c *AMQP) autoReconnect() (err error) {
	for {
		retries := ConnectRetries
		for {
			err = c.connect()
			if err == nil {
				break // Connected, break without err
			}
			c.ctx.WithError(err).Warn("Error trying to connect")
			retries--
			if retries <= 0 {
				break // Out of retries, break with err
			}
			time.Sleep(ConnectRetryDelay)
		}
		if err != nil {
			break // Unable to connect, stop trying
		}

		c.ctx.Info("Connected")

		// Monitor the connection and reconnect on error
		ch := make(chan *amqp.Error)
		c.connection.NotifyClose(ch)
		if mqttErr, hasErr := <-ch; hasErr {
			err = errors.New(mqttErr.Error())
		} else {
			break
		}
		c.ctx.WithError(err).Warn("Connection closed")
		time.Sleep(ConnectRetryDelay)
	}
	if err != nil {
		c.ctx.WithError(err).Error("Could not connect")
	} else {
		c.ctx.Info("Connection closed")
	}
	return
}

// Disconnect from AMQP
func (c *AMQP) Disconnect() error {
	return c.connection.Close()
}

func (c *AMQP) autoRecreatePublishChannel() (err error) {
	var channel *amqp.Channel
	for {
		retries := ConnectRetries
		for {
			channel, err = c.channel()
			if err == nil {
				break // Got channel, break without err
			}
			c.ctx.WithError(err).Warn("Error trying to get channel")
			retries--
			if retries <= 0 {
				break // Out of retries, break with err
			}
			time.Sleep(ConnectRetryDelay)
		}
		if err != nil {
			break // Unable to get channel, stop trying
		}

		c.ctx.Info("Got publish channel")
		c.publish.channel = channel

		// Monitor the channel
		ch := make(chan *amqp.Error)
		channel.NotifyClose(ch)

	handle:
		for {
			select {
			case mqttErr, hasErr := <-ch:
				if hasErr {
					err = errors.New(mqttErr.Error())
					break handle
				}
				break handle
			case msg, ok := <-c.publish.ch:
				if !ok {
					break handle
				}
				ctx := c.ctx.WithField("RoutingKey", msg.routingKey)
				err := c.publish.channel.Publish(c.config.ExchangeName, msg.routingKey, false, false, amqp.Publishing{
					DeliveryMode: amqp.Persistent,
					Timestamp:    time.Now(),
					ContentType:  "application/octet-stream",
					Body:         msg.message,
				})
				if err != nil {
					ctx.WithError(err).Warn("Error during publish")
				} else {
					ctx.Debug("Published message")
				}
			}
		}
		if err == nil {
			break
		}
		c.ctx.WithError(err).Warn("Publish channel closed")
		time.Sleep(ConnectRetryDelay)
	}
	if err != nil {
		c.ctx.WithError(err).Error("Error in publish channel")
	} else {
		c.ctx.Info("Publish channel closed")
	}
	return
}

// Publish a message to a routing key
func (c *AMQP) Publish(routingKey string, message []byte) error {
	c.publish.once.Do(func() {
		go c.autoRecreatePublishChannel()
	})
	select {
	case c.publish.ch <- publishMessage{routingKey: routingKey, message: message}:
	default:
		c.ctx.Warn("Not publishing message [buffer full]")
	}
	return nil
}

func (c *AMQP) subscribe(routingKey string) (chan subscribeMessage, error) {
	channel, err := c.channel()
	if err != nil {
		return nil, err
	}
	defer channel.Close()
	queueName := fmt.Sprintf("%s.%s", c.config.QueuePrefix, routingKey)
	if _, err := channel.QueueDeclare(queueName, true, false, false, false, nil); err != nil {
		return nil, err
	}
	if err := channel.QueueBind(queueName, routingKey, c.config.ExchangeName, false, nil); err != nil {
		return nil, err
	}
	c.subscriptionLock.Lock()
	c.subscriptions[routingKey] = new(subscription)
	c.subscriptions[routingKey].Add(1)
	c.subscriptionLock.Unlock()
	subscribeMessages := make(chan subscribeMessage, BufferSize)
	go func() {
		var channel *amqp.Channel
		for {
			retries := ConnectRetries
			for {
				channel, err = c.channel()
				if err == nil {
					break // Got channel, break without err
				}
				c.ctx.WithError(err).Warn("Error trying to get channel")
				retries--
				if retries <= 0 {
					break // Out of retries, break with err
				}
				time.Sleep(ConnectRetryDelay)
			}
			if err != nil {
				break // Unable to get channel, stop trying
			}

			consumerName := c.config.ConsumerPrefix + "-" + queueName

			c.subscriptionLock.RLock()
			c.subscriptions[routingKey].channel = channel
			c.subscriptions[routingKey].cancelOnce = new(sync.Once)
			c.subscriptions[routingKey].consumerName = consumerName
			c.subscriptions[routingKey].setupOnce.Do(func() {
				c.subscriptions[routingKey].Done()
			})

			c.subscriptionLock.RUnlock()

			c.ctx.WithField("RoutingKey", routingKey).Info("Got subscribe channel")

			// Monitor the channel
			ch := make(chan *amqp.Error)
			channel.NotifyClose(ch)

			err = channel.Qos(1, 0, false)
			if err != nil {
				break
			}

			subscribe, cErr := channel.Consume(queueName, consumerName, false, false, false, false, nil)
			if cErr != nil {
				err = cErr
				break
			}

		handle:
			for {
				select {
				case mqttErr, hasErr := <-ch:
					if hasErr {
						err = errors.New(mqttErr.Error())
						break handle
					}
					break handle
				case msg, ok := <-subscribe:
					if !ok {
						break handle
					}
					c.ctx.WithField("RoutingKey", msg.RoutingKey).Debug("Receiving message")
					subscribeMessages <- subscribeMessage{routingKey: msg.RoutingKey, message: msg.Body}
					msg.Ack(false)
				}
			}
			if err == nil {
				break
			}
			c.ctx.WithError(err).Warn("Subscribe channel closed")
			time.Sleep(ConnectRetryDelay)
		}
		if err != nil {
			c.ctx.WithError(err).Error("Error in subscribe channel")
		} else {
			c.ctx.Info("Subscribe channel closed")
		}
		close(subscribeMessages)
	}()
	return subscribeMessages, nil
}

func (c *AMQP) unsubscribe(routingKey string) error {
	c.subscriptionLock.RLock()
	defer c.subscriptionLock.RUnlock()
	c.subscriptions[routingKey].Wait()
	if err := c.subscriptions[routingKey].cancel(); err != nil {
		return err
	}
	channel, err := c.channel()
	if err != nil {
		return err
	}
	defer channel.Close()
	queueName := fmt.Sprintf("%s.%s", c.config.QueuePrefix, routingKey)
	lost, err := channel.QueueDelete(queueName, true, false, false)
	if err != nil {
		return err
	}
	if lost > 0 {
		c.ctx.WithField("NumMessages", lost).Warn("Lost messages in unsubscribe")
	}
	return nil
}

// SubscribeConnect subscribes to connect messages
func (c *AMQP) SubscribeConnect() (<-chan *types.ConnectMessage, error) {
	messages := make(chan *types.ConnectMessage, BufferSize)
	connect, err := c.subscribe(ConnectRoutingKeyFormat)
	if err != nil {
		return nil, err
	}
	go func() {
		for msg := range connect {
			var connect types.ConnectMessage
			if err := json.Unmarshal(msg.message, &connect); err != nil {
				c.ctx.WithError(err).Warn("Could not unmarshal connect message")
				continue
			}
			select {
			case messages <- &connect:
				c.ctx.Debug("Received connect message")
			default:
				c.ctx.Warn("Could not handle connect message: buffer full")
			}
		}
		close(messages)
	}()
	return messages, nil
}

// UnsubscribeConnect unsubscribes from connect messages
func (c *AMQP) UnsubscribeConnect() error {
	return c.unsubscribe(ConnectRoutingKeyFormat)
}

// SubscribeDisconnect subscribes to disconnect messages
func (c *AMQP) SubscribeDisconnect() (<-chan *types.DisconnectMessage, error) {
	messages := make(chan *types.DisconnectMessage, BufferSize)
	disconnect, err := c.subscribe(DisconnectRoutingKeyFormat)
	if err != nil {
		return nil, err
	}
	go func() {
		for msg := range disconnect {
			var disconnect types.DisconnectMessage
			if err := json.Unmarshal(msg.message, &disconnect); err != nil {
				c.ctx.WithError(err).Warn("Could not unmarshal disconnect message")
				continue
			}
			select {
			case messages <- &disconnect:
				c.ctx.Debug("Received disconnect message")
			default:
				c.ctx.Warn("Could not handle disconnect message: buffer full")
			}
		}
		close(messages)
	}()
	return messages, nil
}

// UnsubscribeDisconnect unsubscribes from disconnect messages
func (c *AMQP) UnsubscribeDisconnect() error {
	return c.unsubscribe(DisconnectRoutingKeyFormat)
}

// SubscribeUplink handles uplink messages for the given gateway ID
func (c *AMQP) SubscribeUplink(gatewayID string) (<-chan *types.UplinkMessage, error) {
	ctx := c.ctx.WithField("GatewayID", gatewayID)
	messages := make(chan *types.UplinkMessage, BufferSize)
	uplink, err := c.subscribe(fmt.Sprintf(UplinkRoutingKeyFormat, gatewayID))
	if err != nil {
		return nil, err
	}
	go func() {
		for msg := range uplink {
			var uplink types.UplinkMessage
			uplink.Message = new(router.UplinkMessage)
			if err := proto.Unmarshal(msg.message, uplink.Message); err != nil {
				ctx.WithError(err).Warn("Could not unmarshal uplink message")
				continue
			}
			select {
			case messages <- &uplink:
				ctx.Debug("Received uplink message")
			default:
				ctx.Warn("Could not handle uplink message: buffer full")
			}
		}
		close(messages)
	}()
	return messages, nil
}

// UnsubscribeUplink unsubscribes from uplink messages for the given gateway ID
func (c *AMQP) UnsubscribeUplink(gatewayID string) error {
	return c.unsubscribe(fmt.Sprintf(UplinkRoutingKeyFormat, gatewayID))
}

// SubscribeStatus handles status messages for the given gateway ID
func (c *AMQP) SubscribeStatus(gatewayID string) (<-chan *types.StatusMessage, error) {
	ctx := c.ctx.WithField("GatewayID", gatewayID)
	messages := make(chan *types.StatusMessage, BufferSize)
	status, err := c.subscribe(fmt.Sprintf(StatusRoutingKeyFormat, gatewayID))
	if err != nil {
		return nil, err
	}
	go func() {
		for msg := range status {
			var status types.StatusMessage
			status.Message = new(gateway.Status)
			if err := proto.Unmarshal(msg.message, status.Message); err != nil {
				ctx.WithError(err).Warn("Could not unmarshal uplink message")
				continue
			}
			select {
			case messages <- &status:
				ctx.Debug("Received status message")
			default:
				ctx.Warn("Could not handle status message: buffer full")
			}
		}
		close(messages)
	}()
	return messages, nil
}

// UnsubscribeStatus unsubscribes from status messages for the given gateway ID
func (c *AMQP) UnsubscribeStatus(gatewayID string) error {
	return c.unsubscribe(fmt.Sprintf(StatusRoutingKeyFormat, gatewayID))
}

// PublishDownlink publishes a downlink message
func (c *AMQP) PublishDownlink(message *types.DownlinkMessage) error {
	ctx := c.ctx.WithField("GatewayID", message.GatewayID)
	msg, err := proto.Marshal(message.Message)
	if err != nil {
		return err
	}
	err = c.Publish(fmt.Sprintf(DownlinkRoutingKeyFormat, message.GatewayID), msg)
	if err != nil {
		return err
	}
	ctx.Debug("Scheduled downlink message")
	return nil
}
