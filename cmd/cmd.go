// Copyright © 2016 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package cmd

import (
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/TheThingsNetwork/gateway-connector-bridge/auth"
	"github.com/TheThingsNetwork/gateway-connector-bridge/backend/amqp"
	"github.com/TheThingsNetwork/gateway-connector-bridge/backend/mqtt"
	"github.com/TheThingsNetwork/gateway-connector-bridge/backend/ttn"
	"github.com/TheThingsNetwork/gateway-connector-bridge/exchange"
	"github.com/TheThingsNetwork/go-utils/handlers/cli"
	ttnlog "github.com/TheThingsNetwork/go-utils/log"
	"github.com/TheThingsNetwork/go-utils/log/apex"
	"github.com/TheThingsNetwork/ttn/api"
	"github.com/apex/log"
	"github.com/apex/log/handlers/json"
	"github.com/apex/log/handlers/multi"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	redis "gopkg.in/redis.v5"
)

// BridgeCmd is the main command that is executed when running gateway-connector-bridge
var BridgeCmd = &cobra.Command{
	Use:   "gateway-connector-bridge",
	Short: "The Things Network's Gateway Connector bridge",
	Long:  `gateway-connector-bridge bridges between Gateway Connector and gRPC`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		var logHandlers []log.Handler

		logHandlers = append(logHandlers, cli.New(os.Stdout))

		if logFileLocation := config.GetString("log-file"); logFileLocation != "" {
			absLogFileLocation, err := filepath.Abs(logFileLocation)
			if err != nil {
				panic(err)
			}
			logFile, err = os.OpenFile(absLogFileLocation, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
			if err != nil {
				panic(err)
			}
			if err == nil {
				logHandlers = append(logHandlers, json.New(logFile))
			}
		}

		ctx = &log.Logger{
			Level:   log.DebugLevel,
			Handler: multi.New(logHandlers...),
		}
	},
	Run: runBridge,
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if logFile != nil {
			time.Sleep(100 * time.Millisecond)
			logFile.Close()
		}
	},
}

func runBridge(cmd *cobra.Command, args []string) {
	bridge := exchange.New(ctx)

	// Set up Redis
	var connectedGatewayIDs []string
	var authBackend auth.Interface
	if config.GetBool("redis") {
		redis := redis.NewClient(&redis.Options{
			Addr:     config.GetString("redis-address"),
			Password: config.GetString("redis-password"),
			DB:       config.GetInt("redis-db"),
		})
		ctx.Info("Initializing Redis state backend")
		connectedGatewayIDs = bridge.InitRedisState(redis, "")
		ctx.Info("Initializing Redis auth backend")
		authBackend = auth.NewRedis(redis, "")
	} else {
		ctx.Info("Initializing Memory auth backend")
		authBackend = auth.NewMemory()
	}

	if accountServer := config.GetString("account-server"); accountServer != "" && accountServer != "disable" {
		ctx.WithField("AccountServer", accountServer).Info("Initializing access key exchanger")
		authBackend.SetExchanger(auth.NewAccountServer(accountServer, ctx))
	}
	bridge.SetAuth(authBackend)

	// Set up the TTN routers (from comma-separated list of discovery-server/router-id)
	ttnRouters := strings.Split(config.GetString("ttn-router"), ",")
	if len(ttnRouters) > 0 {
		if rootCAFile := config.GetString("root-ca-file"); rootCAFile != "" {
			roots, err := ioutil.ReadFile(rootCAFile)
			if err != nil {
				ctx.WithError(err).Fatal("Could not load Root CA file")
			}
			if !api.RootCAs.AppendCertsFromPEM(roots) {
				ctx.Warn("Could not load all CAs from the Root CA file")
			} else {
				ctx.Infof("Using Root CAs from %s", rootCAFile)
			}
		}
		ttnlog.Set(apex.Wrap(ctx))
	}
	for _, ttnRouter := range ttnRouters {
		if ttnRouter == "disable" {
			continue
		}
		parts := strings.Split(ttnRouter, "/")
		if len(parts) == 2 {
			ctx.WithField("DiscoveryServer", parts[0]).WithField("RouterID", parts[1]).Infof("Initializing TTN Router")
			router, err := ttn.New(ttn.RouterConfig{
				DiscoveryServer: parts[0],
				RouterID:        parts[1],
			}, ctx, func(gatewayID string) string {
				token, err := authBackend.GetToken(gatewayID)
				if err != nil {
					ctx.WithField("GatewayID", gatewayID).WithError(err).Debug("Could not get token for Gateway")
					return ""
				}
				return token
			})
			if err != nil {
				ctx.WithError(err).Warnf("Could not initialize TTN router %s", ttnRouter)
				continue
			}
			bridge.AddNorthbound(router)
		}
	}

	// Set up the MQTT backends (from comma-separated list of user:pass@host:port)
	mqttRegexp := regexp.MustCompile(`^(?:([0-9a-z_-]+)(?::([0-9A-Za-z-!"#$%&'()*+,.:;<=>?@[\]^_{|}~]+))?@)?([0-9a-z.-]+:[0-9]+)$`)
	mqttBrokers := strings.Split(config.GetString("mqtt"), ",")
	for _, mqttBroker := range mqttBrokers {
		if mqttBroker == "disable" {
			continue
		}
		parts := mqttRegexp.FindStringSubmatch(mqttBroker)
		ctx.WithField("Username", parts[1]).WithField("Password", parts[2]).WithField("Address", parts[3]).Infof("Initializing MQTT")
		mqtt, err := mqtt.New(mqtt.Config{
			Brokers:  []string{"tcp://" + parts[3]},
			Username: parts[1],
			Password: parts[2],
		}, ctx)
		if err != nil {
			ctx.WithError(err).Warnf("Could not initialize MQTT broker %s", mqttBroker)
		}
		bridge.AddSouthbound(mqtt)
	}

	// Set up the AMQP backends (from comma-separated list of user:pass@host:port)
	amqpRegexp := regexp.MustCompile(`^(?:([0-9a-z_-]+)(?::([0-9A-Za-z-!"#$%&'()*+,.:;<=>?@[\]^_{|}~]+))?@)?([0-9a-z.-]+:[0-9]+)$`) // user:pass@host:port
	amqpBrokers := strings.Split(config.GetString("amqp"), ",")
	for _, amqpBroker := range amqpBrokers {
		if amqpBroker == "disable" {
			continue
		}
		parts := amqpRegexp.FindStringSubmatch(amqpBroker)
		ctx.WithField("Username", parts[1]).WithField("Password", parts[2]).WithField("Address", parts[3]).Infof("Initializing AMQP")
		amqp, err := amqp.New(amqp.Config{
			Address:  parts[3],
			Username: parts[1],
			Password: parts[2],
		}, ctx)
		if err != nil {
			ctx.WithError(err).Warnf("Could not initialize AMQP broker %s", amqpBroker)
		}
		bridge.AddSouthbound(amqp)
	}

	if bridge.Start(30 * time.Second) {
		ctx.Info("All backends started")
	} else {
		ctx.Fatal("Not all backends started in time")
	}

	defer func() {
		bridge.Stop()
		time.Sleep(100 * time.Millisecond)
	}()

	if len(connectedGatewayIDs) > 0 {
		ctx.Infof("Reconnecting %d gateways", len(connectedGatewayIDs))
		bridge.ConnectGateway(connectedGatewayIDs...)
	}

	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	ctx.WithField("signal", <-sigChan).Info("signal received")
}

func init() {
	BridgeCmd.Flags().String("log-file", "", "Location of the log file")

	BridgeCmd.Flags().Bool("redis", true, "Use Redis auth backend")
	BridgeCmd.Flags().String("redis-address", "localhost:6379", "Redis host and port")
	BridgeCmd.Flags().String("redis-password", "", "Redis password")
	BridgeCmd.Flags().Int("redis-db", 0, "Redis database")

	BridgeCmd.Flags().String("root-ca-file", "", "Location of the file containing Root CA certificates")

	BridgeCmd.Flags().String("account-server", "https://preview.account.thethingsnetwork.org", "Use an account server for exchanging access keys")

	BridgeCmd.Flags().StringSlice("ttn-router", []string{"discovery.thethingsnetwork.org:1900/ttn-router-eu"}, "TTN Router to connect to")
	BridgeCmd.Flags().StringSlice("mqtt", []string{"guest:guest@localhost:1883"}, "MQTT Broker to connect to (disable with \"disable\")")
	BridgeCmd.Flags().StringSlice("amqp", []string{"guest:guest@localhost:5672"}, "AMQP Broker to connect to (disable with \"disable\")")

	viper.BindPFlags(BridgeCmd.Flags())
}
