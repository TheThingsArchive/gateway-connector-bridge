// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/TheThingsNetwork/gateway-connector-bridge/auth"
	"github.com/TheThingsNetwork/gateway-connector-bridge/backend/amqp"
	"github.com/TheThingsNetwork/gateway-connector-bridge/backend/dummy"
	"github.com/TheThingsNetwork/gateway-connector-bridge/backend/mqtt"
	"github.com/TheThingsNetwork/gateway-connector-bridge/backend/pktfwd"
	"github.com/TheThingsNetwork/gateway-connector-bridge/backend/ttn"
	"github.com/TheThingsNetwork/gateway-connector-bridge/exchange"
	"github.com/TheThingsNetwork/gateway-connector-bridge/middleware"
	"github.com/TheThingsNetwork/gateway-connector-bridge/middleware/blacklist"
	"github.com/TheThingsNetwork/gateway-connector-bridge/middleware/debug"
	"github.com/TheThingsNetwork/gateway-connector-bridge/middleware/deduplicate"
	"github.com/TheThingsNetwork/gateway-connector-bridge/middleware/gatewayinfo"
	"github.com/TheThingsNetwork/gateway-connector-bridge/middleware/inject"
	"github.com/TheThingsNetwork/gateway-connector-bridge/middleware/lorafilter"
	"github.com/TheThingsNetwork/gateway-connector-bridge/middleware/ratelimit"
	"github.com/TheThingsNetwork/go-utils/handlers/cli"
	ttnlog "github.com/TheThingsNetwork/go-utils/log"
	"github.com/TheThingsNetwork/go-utils/log/apex"
	"github.com/TheThingsNetwork/ttn/api/pool"
	"github.com/apex/log"
	"github.com/apex/log/handlers/json"
	"github.com/apex/log/handlers/multi"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

		logLevel := log.InfoLevel
		if config.GetBool("debug") {
			logLevel = log.DebugLevel
		}
		ctx = &log.Logger{
			Level:   logLevel,
			Handler: multi.New(logHandlers...),
		}

		ttnlog.Set(apex.Wrap(ctx))
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
	var err error

	bridge := exchange.New(ctx)

	var middleware middleware.Chain

	if viper.GetBool("lorafilter") {
		ctx.Info("Adding lorafilter middleware")
		middleware = append(middleware, lorafilter.NewFilter())
	}

	if list := viper.GetStringSlice("blacklist"); len(list) > 0 {
		ctx.Info("Adding blacklist middleware")
		blacklist, _ := blacklist.NewBlacklist(list...)
		go func() {
			for range time.Tick(viper.GetDuration("blacklist-refresh")) {
				blacklist.FetchRemotes()
			}
		}()
		middleware = append(middleware, blacklist)
	}

	if viper.GetBool("deduplicate") {
		ctx.Info("Adding deduplicate middleware")
		middleware = append(middleware, deduplicate.NewDeduplicate())
	}

	middleware = append(middleware, debug.New())

	id := fmt.Sprintf(
		"%s %s-%s (%s)",
		config.GetString("id"),
		config.GetString("version"),
		config.GetString("gitCommit"),
		config.GetString("buildDate"),
	)
	bridge.SetID(config.GetString("id"))

	// Set up Redis
	var redisClient *redis.Client
	if config.GetBool("redis") {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     config.GetString("redis-address"),
			Password: config.GetString("redis-password"),
			DB:       config.GetInt("redis-db"),
		})

		for {
			err = redisClient.Ping().Err()
			if err == nil {
				ctx.Info("Connected to Redis")
				break
			}
			time.Sleep(time.Second)
			ctx.WithError(err).Warn("Could not connect to Redis. Retrying...")
		}
	}

	// Redis state
	var connectedGatewayIDs []string
	if viper.GetBool("reconnect-gateways") && redisClient != nil {
		ctx.Info("Initializing Redis state backend")
		connectedGatewayIDs = bridge.InitRedisState(redisClient, "")
	}

	// Auth
	var authBackend auth.Interface
	if redisClient != nil {
		ctx.Info("Initializing Redis auth backend")
		authBackend = auth.NewRedis(redisClient, "")
	} else {
		ctx.Info("Initializing Memory auth backend")
		authBackend = auth.NewMemory()
	}

	// Ratelimit
	if viper.GetBool("ratelimit") {
		limits := ratelimit.Limits{
			Uplink:   config.GetInt("ratelimit-uplink"),
			Downlink: config.GetInt("ratelimit-downlink"),
			Status:   config.GetInt("ratelimit-status"),
		}

		if redisClient != nil {
			ctx.Info("Initializing Redis rate limiting")
			middleware = append(middleware, ratelimit.NewRedisRateLimit(redisClient, limits))
		} else {
			ctx.Info("Initializing rate limiting")
			middleware = append(middleware, ratelimit.NewRateLimit(limits))
		}
	}

	if accountServer := config.GetString("account-server"); accountServer != "" && accountServer != "disable" {
		ctx := ctx.WithField("AccountServer", accountServer)

		expire := viper.GetDuration("info-expire")
		gatewayInfo := gatewayinfo.NewPublic(accountServer).WithExpire(expire)
		if redisClient != nil {
			ctx.WithField("Expire", expire).Info("Initializing Redis gatewayinfo")
			gatewayInfo, err = gatewayInfo.WithRedis(redisClient, "gatewayinfo")
		} else {
			ctx.WithField("Expire", expire).Info("Initializing gatewayinfo")
		}
		middleware = append(middleware, gatewayInfo)

		ctx.WithField("AccountServer", accountServer).Info("Initializing access key exchanger")
		authBackend.SetExchanger(auth.NewAccountServer(accountServer, ctx))
	}
	bridge.SetAuth(authBackend)

	middleware = append(middleware, inject.NewInject(inject.Fields{
		Bridge:        id,
		FrequencyPlan: viper.GetString("inject-frequency-plan"),
	}))

	// Set up the TTN routers (from comma-separated list of discovery-server/router-id)
	ttnRouters := config.GetStringSlice("ttn-router")
	if len(ttnRouters) > 0 {
		if rootCAFile := config.GetString("root-ca-file"); rootCAFile != "" {
			roots, err := ioutil.ReadFile(rootCAFile)
			if err != nil {
				ctx.WithError(err).Fatal("Could not load Root CA file")
			}
			if !pool.RootCAs.AppendCertsFromPEM(roots) {
				ctx.Warn("Could not load all CAs from the Root CA file")
			} else {
				ctx.Infof("Using Root CAs from %s", rootCAFile)
			}
		}
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
				if err != nil && err != auth.ErrGatewayNotFound {
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

	if udp := config.GetString("udp"); udp != "" {
		pktfwd := pktfwd.New(pktfwd.Config{
			Bind:     udp,
			Session:  config.GetDuration("udp-session"),
			LockIP:   config.GetBool("udp-lock-ip") || config.GetBool("udp-lock-port"),
			LockPort: config.GetBool("udp-lock-port"),
		}, ttnlog.Get())
		bridge.AddSouthbound(pktfwd)
	} else {
		ctx.Warn("Parameter 'udp' is empty. No UDP listener for gateways opened")
	}

	// Set up the MQTT backends (from comma-separated list of user:pass@host:port)
	mqttRegexp := regexp.MustCompile(`^(?:([0-9a-z_-]+)(?::([0-9A-Za-z-!"#$%&'()*+,.:;<=>?@[\]^_{|}~]+))?@)?([0-9a-z.-]+:[0-9]+)$`)
	mqttBrokers := config.GetStringSlice("mqtt")
	for _, mqttBroker := range mqttBrokers {
		if mqttBroker == "disable" || mqttBroker == "" {
			continue
		}
		parts := mqttRegexp.FindStringSubmatch(mqttBroker)
		if len(parts) < 4 {
			ctx.WithField("Broker", mqttBroker).Info("Skipping MQTT Broker")
			continue
		}
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
	amqpBrokers := config.GetStringSlice("amqp")
	for _, amqpBroker := range amqpBrokers {
		if amqpBroker == "disable" || amqpBroker == "" {
			continue
		}
		parts := amqpRegexp.FindStringSubmatch(amqpBroker)
		if len(parts) < 4 {
			ctx.WithField("Broker", amqpBroker).Info("Skipping AMQP Broker")
			continue
		}
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

	if debugAddr := config.GetString("http-debug-addr"); debugAddr != "" {
		ctx.WithField("Address", debugAddr).Infof("Initializing HTTP Debug")
		httpDummy := dummy.New(ctx).WithHTTPServer(debugAddr)
		bridge.AddNorthbound(httpDummy)
		bridge.AddSouthbound(httpDummy)
	}

	bridge.SetMiddleware(middleware)

	ctx.WithField("NumWorkers", config.GetInt("workers")).Info("Starting Bridge...")
	if bridge.Start(config.GetInt("workers"), 30*time.Second) {
		ctx.Info("All backends started")
	} else {
		ctx.Fatal("Not all backends started in time")
	}

	defer func() {
		bridge.Stop()
		time.Sleep(100 * time.Millisecond)
	}()

	if viper.GetBool("reconnect-gateways") && len(connectedGatewayIDs) > 0 {
		ctx.Infof("Reconnecting %d gateways", len(connectedGatewayIDs))
		bridge.ConnectGateway(connectedGatewayIDs...)
	}

	if viper.GetBool("route-unknown-gateways") {
		bridge.ConnectGateway("")
	}

	if addr := config.GetString("http-status-addr"); addr != "" {
		ctx.WithField("Address", addr).Infof("Initializing HTTP Status")
		http.Handle("/metrics", promhttp.Handler())
		go http.ListenAndServe(addr, nil)
	}

	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	ctx.WithField("signal", <-sigChan).Info("signal received")
}

func init() {
	BridgeCmd.Flags().Bool("debug", false, "Print debug logs")
	BridgeCmd.Flags().String("log-file", "", "Location of the log file")

	BridgeCmd.Flags().Bool("redis", true, "Use Redis auth backend")
	BridgeCmd.Flags().String("redis-address", "localhost:6379", "Redis host and port")
	BridgeCmd.Flags().String("redis-password", "", "Redis password")
	BridgeCmd.Flags().Int("redis-db", 0, "Redis database")

	BridgeCmd.Flags().String("root-ca-file", "", "Location of the file containing Root CA certificates")

	BridgeCmd.Flags().String("account-server", "https://account.thethingsnetwork.org", "Use an account server for exchanging access keys and fetching gateway information")
	BridgeCmd.Flags().Duration("info-expire", time.Hour, "Gateway Information expiration time")
	BridgeCmd.Flags().Bool("reconnect-gateways", true, "Reconnect previously connected gateways")
	BridgeCmd.Flags().Bool("route-unknown-gateways", false, "Route traffic for unknown gateways")

	BridgeCmd.Flags().String("inject-frequency-plan", "", "Inject a frequency plan field into status message that don't have one")

	BridgeCmd.Flags().Bool("lorafilter", true, "Block non-LoRaWAN messages")
	BridgeCmd.Flags().Bool("deduplicate", true, "Block duplicate messages")
	BridgeCmd.Flags().StringSlice("blacklist", nil, "Blacklists to use")
	BridgeCmd.Flags().Duration("blacklist-refresh", time.Hour, "Refresh rate for remote blacklists")

	BridgeCmd.Flags().Bool("ratelimit", false, "Rate-limit messages")
	BridgeCmd.Flags().Uint("ratelimit-uplink", 600, "Uplink rate limit (per gateway per minute)")
	BridgeCmd.Flags().Uint("ratelimit-downlink", 0, "Downlink rate limit (per gateway per minute)")
	BridgeCmd.Flags().Uint("ratelimit-status", 20, "Status rate limit (per gateway per minute)")

	BridgeCmd.Flags().StringSlice("ttn-router", []string{"discover.thethingsnetwork.org:1900/ttn-router-eu"}, "TTN Router to connect to")
	BridgeCmd.Flags().String("udp", "", "UDP address to listen on for Semtech Packet Forwarder gateways")
	BridgeCmd.Flags().Duration("udp-session", time.Minute, "Duration of gateway sessions")
	BridgeCmd.Flags().Bool("udp-lock-ip", true, "Lock gateways to IP addresses for the session duration")
	BridgeCmd.Flags().Bool("udp-lock-port", false, "Additional to udp-lock-ip, also lock gateways to ports for the session duration")
	BridgeCmd.Flags().StringSlice("mqtt", []string{"guest:guest@localhost:1883"}, "MQTT Broker to connect to (user:pass@host:port; disable with \"disable\")")
	BridgeCmd.Flags().StringSlice("amqp", []string{}, "AMQP Broker to connect to (user:pass@host:port; disable with \"disable\")")

	BridgeCmd.Flags().String("http-status-addr", ":10700", "Address of the HTTP status server to start")
	BridgeCmd.Flags().String("http-debug-addr", "", "The address of the HTTP debug server to start")

	BridgeCmd.Flags().String("id", "", "ID of this bridge")
	BridgeCmd.Flags().Int("workers", 1, "Number of parallel workers")

	viper.BindPFlags(BridgeCmd.Flags())
}
