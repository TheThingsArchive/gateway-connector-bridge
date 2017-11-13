// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package mqtt

import (
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	"github.com/TheThingsNetwork/api/gateway"
	"github.com/TheThingsNetwork/api/router"
	"github.com/TheThingsNetwork/api/trace"
	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
	"github.com/TheThingsNetwork/ttn/utils/random"
	"github.com/apex/log"
	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/gogo/protobuf/proto"
)

// PublishTimeout is the timeout before returning from publish without checking error
var PublishTimeout = 50 * time.Millisecond

// New returns a new MQTT
func New(config Config, ctx log.Interface) (*MQTT, error) {
	mqtt := new(MQTT)

	mqtt.ctx = ctx.WithField("Connector", "MQTT")

	mqttOpts := paho.NewClientOptions()
	for _, broker := range config.Brokers {
		mqttOpts.AddBroker(broker)
	}
	if config.TLSConfig != nil {
		mqttOpts.SetTLSConfig(config.TLSConfig)
	}
	mqttOpts.SetClientID(fmt.Sprintf("bridge-%s", random.String(16)))
	mqttOpts.SetUsername(config.Username)
	mqttOpts.SetPassword(config.Password)
	mqttOpts.SetKeepAlive(30 * time.Second)
	mqttOpts.SetPingTimeout(10 * time.Second)
	mqttOpts.SetCleanSession(true)
	mqttOpts.SetDefaultPublishHandler(func(_ paho.Client, msg paho.Message) {
		mqtt.ctx.Warnf("Received unhandled message on MQTT: %v", msg)
	})

	mqtt.subscriptions = make(map[string]subscription)
	var reconnecting bool
	mqttOpts.SetConnectionLostHandler(func(_ paho.Client, err error) {
		mqtt.ctx.Warnf("Disconnected (%s). Reconnecting...", err.Error())
		reconnecting = true
	})
	mqttOpts.SetOnConnectHandler(func(_ paho.Client) {
		mqtt.ctx.Info("Connected")
		if reconnecting {
			mqtt.resubscribe()
			reconnecting = false
		}
	})

	mqtt.client = paho.NewClient(mqttOpts)

	return mqtt, nil
}

// QoS indicates the MQTT Quality of Service level.
// 0: The broker/client will deliver the message once, with no confirmation.
// 1: The broker/client will deliver the message at least once, with confirmation required.
// 2: The broker/client will deliver the message exactly once by using a four step handshake.
var (
	PublishQoS   byte = 0x00
	SubscribeQoS byte = 0x00
)

// BufferSize indicates the maximum number of MQTT messages that should be buffered
var BufferSize = 10

// Topic formats for connect, disconnect, uplink, downlink and status messages
var (
	ConnectTopicFormat    = "connect"
	DisconnectTopicFormat = "disconnect"
	UplinkTopicFormat     = "%s/up"
	DownlinkTopicFormat   = "%s/down"
	StatusTopicFormat     = "%s/status"
)

// Config contains configuration for MQTT
type Config struct {
	Brokers   []string
	Username  string
	Password  string
	TLSConfig *tls.Config
}

type subscription struct {
	handler paho.MessageHandler
	cancel  func()
}

// MQTT side of the bridge
type MQTT struct {
	ctx           log.Interface
	client        paho.Client
	subscriptions map[string]subscription
	mu            sync.Mutex
}

var (
	// ConnectRetries says how many times the client should retry a failed connection
	ConnectRetries = 10
	// ConnectRetryDelay says how long the client should wait between retries
	ConnectRetryDelay = time.Second
)

// Connect to MQTT
func (c *MQTT) Connect() error {
	var err error
	for retries := 0; retries < ConnectRetries; retries++ {
		token := c.client.Connect()
		finished := token.WaitTimeout(1 * time.Second)
		if !finished {
			c.ctx.Warn("MQTT connection took longer than expected...")
			token.Wait()
		}
		err = token.Error()
		if err == nil {
			break
		}
		c.ctx.Warnf("Could not connect to MQTT (%s). Retrying...", err.Error())
		<-time.After(ConnectRetryDelay)
	}
	if err != nil {
		return fmt.Errorf("Could not connect to MQTT (%s)", err)
	}
	return err
}

// Disconnect from MQTT
func (c *MQTT) Disconnect() error {
	c.client.Disconnect(100)
	return nil
}

func (c *MQTT) publish(topic string, msg []byte) paho.Token {
	return c.client.Publish(topic, PublishQoS, false, msg)
}

func (c *MQTT) subscribe(topic string, handler paho.MessageHandler, cancel func()) paho.Token {
	c.mu.Lock()
	defer c.mu.Unlock()
	wrappedHandler := func(client paho.Client, msg paho.Message) {
		if msg.Retained() {
			c.ctx.WithField("Topic", msg.Topic()).Debug("Ignore retained message")
			return
		}
		handler(client, msg)
	}
	c.subscriptions[topic] = subscription{wrappedHandler, cancel}
	return c.client.Subscribe(topic, SubscribeQoS, wrappedHandler)
}

func (c *MQTT) resubscribe() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for topic, subscription := range c.subscriptions {
		c.client.Subscribe(topic, SubscribeQoS, subscription.handler)
	}
}

func (c *MQTT) unsubscribe(topic string) paho.Token {
	c.mu.Lock()
	defer c.mu.Unlock()
	if subscription, ok := c.subscriptions[topic]; ok && subscription.cancel != nil {
		subscription.cancel()
	}
	delete(c.subscriptions, topic)
	return c.client.Unsubscribe(topic)
}

// SubscribeConnect subscribes to connect messages
func (c *MQTT) SubscribeConnect() (<-chan *types.ConnectMessage, error) {
	messages := make(chan *types.ConnectMessage, BufferSize)
	token := c.subscribe(ConnectTopicFormat, func(_ paho.Client, msg paho.Message) {
		var connect types.ConnectMessage
		if err := proto.Unmarshal(msg.Payload(), &connect); err != nil {
			c.ctx.WithError(err).Warn("Could not unmarshal connect message")
			return
		}
		ctx := c.ctx.WithField("GatewayID", connect.GatewayID)
		select {
		case messages <- &connect:
			ctx.WithField("ProtoSize", len(msg.Payload())).Debug("Received connect message")
		default:
			ctx.Warn("Could not handle connect message: buffer full")
		}
	}, func() {
		close(messages)
	})
	token.Wait()
	return messages, token.Error()
}

// UnsubscribeConnect unsubscribes from connect messages
func (c *MQTT) UnsubscribeConnect() error {
	token := c.unsubscribe(ConnectTopicFormat)
	token.Wait()
	return token.Error()
}

// SubscribeDisconnect subscribes to disconnect messages
func (c *MQTT) SubscribeDisconnect() (<-chan *types.DisconnectMessage, error) {
	messages := make(chan *types.DisconnectMessage, BufferSize)
	token := c.subscribe(DisconnectTopicFormat, func(_ paho.Client, msg paho.Message) {
		var disconnect types.DisconnectMessage
		if err := proto.Unmarshal(msg.Payload(), &disconnect); err != nil {
			c.ctx.WithError(err).Warn("Could not unmarshal disconnect message")
			return
		}
		ctx := c.ctx.WithField("GatewayID", disconnect.GatewayID)
		select {
		case messages <- &disconnect:
			ctx.WithField("ProtoSize", len(msg.Payload())).Debug("Received disconnect message")
		default:
			ctx.Warn("Could not handle disconnect message: buffer full")
		}
	}, func() {
		close(messages)
	})
	token.Wait()
	return messages, token.Error()
}

// UnsubscribeDisconnect unsubscribes from disconnect messages
func (c *MQTT) UnsubscribeDisconnect() error {
	token := c.unsubscribe(DisconnectTopicFormat)
	token.Wait()
	return token.Error()
}

// SubscribeUplink handles uplink messages for the given gateway ID
func (c *MQTT) SubscribeUplink(gatewayID string) (<-chan *types.UplinkMessage, error) {
	ctx := c.ctx.WithField("GatewayID", gatewayID)
	messages := make(chan *types.UplinkMessage, BufferSize)
	token := c.subscribe(fmt.Sprintf(UplinkTopicFormat, gatewayID), func(_ paho.Client, msg paho.Message) {
		uplink := types.UplinkMessage{
			GatewayID: gatewayID,
			Message:   new(router.UplinkMessage),
		}
		if err := proto.Unmarshal(msg.Payload(), uplink.Message); err != nil {
			ctx.WithError(err).Warn("Could not unmarshal uplink message")
			return
		}
		uplink.Message.Trace = uplink.Message.Trace.WithEvent(trace.ReceiveEvent, "backend", "mqtt")
		select {
		case messages <- &uplink:
			ctx.WithField("ProtoSize", len(msg.Payload())).Debug("Received uplink message")
		default:
			ctx.Warn("Could not handle uplink message: buffer full")
		}
	}, func() {
		close(messages)
	})
	token.Wait()
	return messages, token.Error()
}

// UnsubscribeUplink unsubscribes from uplink messages for the given gateway ID
func (c *MQTT) UnsubscribeUplink(gatewayID string) error {
	token := c.unsubscribe(fmt.Sprintf(UplinkTopicFormat, gatewayID))
	token.Wait()
	return token.Error()
}

// SubscribeStatus handles status messages for the given gateway ID
func (c *MQTT) SubscribeStatus(gatewayID string) (<-chan *types.StatusMessage, error) {
	ctx := c.ctx.WithField("GatewayID", gatewayID)
	messages := make(chan *types.StatusMessage, BufferSize)
	token := c.subscribe(fmt.Sprintf(StatusTopicFormat, gatewayID), func(_ paho.Client, msg paho.Message) {
		status := types.StatusMessage{
			Backend:   "MQTT",
			GatewayID: gatewayID,
			Message:   new(gateway.Status),
		}
		if err := proto.Unmarshal(msg.Payload(), status.Message); err != nil {
			ctx.WithError(err).Warn("Could not unmarshal status message")
			return
		}
		select {
		case messages <- &status:
			ctx.WithField("ProtoSize", len(msg.Payload())).Debug("Received status message")
		default:
			ctx.Warn("Could not handle status message: buffer full")
		}
	}, func() {
		close(messages)
	})
	token.Wait()
	return messages, token.Error()
}

// UnsubscribeStatus unsubscribes from status messages for the given gateway ID
func (c *MQTT) UnsubscribeStatus(gatewayID string) error {
	token := c.unsubscribe(fmt.Sprintf(StatusTopicFormat, gatewayID))
	token.Wait()
	return token.Error()
}

// PublishDownlink publishes a downlink message
func (c *MQTT) PublishDownlink(message *types.DownlinkMessage) error {
	ctx := c.ctx.WithField("GatewayID", message.GatewayID)
	downlink := *message.Message
	downlink.Trace = nil
	msg, err := proto.Marshal(&downlink)
	if err != nil {
		return err
	}
	token := c.publish(fmt.Sprintf(DownlinkTopicFormat, message.GatewayID), msg)
	go func() {
		token.Wait()
		if err := token.Error(); err != nil {
			ctx.WithError(err).Warn("Could not publish downlink message")
			return
		}
		ctx.WithField("ProtoSize", len(msg)).Debug("Published downlink message")
	}()
	return nil
}
