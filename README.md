# The Things Network Gateway Connector Bridge

[![Build Status](https://travis-ci.org/TheThingsNetwork/gateway-connector-bridge.svg?branch=master)](https://travis-ci.org/TheThingsNetwork/gateway-connector-bridge) [![Coverage Status](https://coveralls.io/repos/github/TheThingsNetwork/gateway-connector-bridge/badge.svg?branch=master)](https://coveralls.io/github/TheThingsNetwork/gateway-connector-bridge?branch=master) [![GoDoc](https://godoc.org/github.com/TheThingsNetwork/gateway-connector-bridge?status.svg)](https://godoc.org/github.com/TheThingsNetwork/gateway-connector-bridge)

![The Things Network](https://thethings.blob.core.windows.net/ttn/logo.svg)

## Installation

Download precompiled binaries for [64 bit Linux][download-linux-amd64], [32 bit Linux][download-linux-386], [ARM Linux][download-linux-arm], [macOS][download-darwin-amd64], [64 bit Windows][download-windows-amd64], [32 bit Windows][download-windows-386].

[download-linux-amd64]: https://ttnreleases.blob.core.windows.net/gateway-connector-bridge/master/gateway-connector-bridge-linux-amd64.zip
[download-linux-386]: https://ttnreleases.blob.core.windows.net/gateway-connector-bridge/master/gateway-connector-bridge-linux-386.zip
[download-linux-arm]: https://ttnreleases.blob.core.windows.net/gateway-connector-bridge/master/gateway-connector-bridge-linux-arm.zip
[download-darwin-amd64]: https://ttnreleases.blob.core.windows.net/gateway-connector-bridge/master/gateway-connector-bridge-darwin-amd64.zip
[download-windows-amd64]: https://ttnreleases.blob.core.windows.net/gateway-connector-bridge/master/gateway-connector-bridge-windows-amd64.exe.zip
[download-windows-386]: https://ttnreleases.blob.core.windows.net/gateway-connector-bridge/master/gateway-connector-bridge-windows-386.exe.zip

Other requirements are:

- [Redis](http://redis.io/download)
- An MQTT Broker (see also the [Security](#security) section)

## Usage

```
Usage:
  gateway-connector-bridge [flags]

Flags:
      --account-server string          Use an account server for exchanging access keys and fetching gateway information (default "https://account.thethingsnetwork.org")
      --amqp stringSlice               AMQP Broker to connect to (user:pass@host:port; disable with "disable")
      --debug                          Print debug logs
      --http-debug-addr string         The address of the HTTP debug server to start
      --id string                      ID of this bridge
      --info-expire duration           Gateway Information expiration time (default 1h0m0s)
      --inject-frequency-plan string   Inject a frequency plan field into status message that don't have one
      --log-file string                Location of the log file
      --mqtt stringSlice               MQTT Broker to connect to (user:pass@host:port; disable with "disable") (default [guest:guest@localhost:1883])
      --ratelimit                      Rate-limit messages
      --ratelimit-downlink uint        Downlink rate limit (per gateway per minute)
      --ratelimit-status uint          Status rate limit (per gateway per minute) (default 20)
      --ratelimit-uplink uint          Uplink rate limit (per gateway per minute) (default 600)
      --redis                          Use Redis auth backend (default true)
      --redis-address string           Redis host and port (default "localhost:6379")
      --redis-db int                   Redis database
      --redis-password string          Redis password
      --root-ca-file string            Location of the file containing Root CA certificates
      --route-unknown-gateways         Route traffic for unknown gateways
      --status-addr string             Address of the gRPC status server to start
      --status-key stringSlice         Access key for the gRPC status server
      --ttn-router stringSlice         TTN Router to connect to (default [discover.thethingsnetwork.org:1900/ttn-router-eu])
      --udp string                     UDP address to listen on for Semtech Packet Forwarder gateways
      --udp-lock-ip                    Lock gateways to IP addresses for the session duration (default true)
      --udp-lock-port                  Additional to udp-lock-ip, also lock gateways to ports for the session duration
      --udp-session duration           Duration of gateway sessions (default 1m0s)
      --workers int                    Number of parallel workers (default 1)
```

For running in Docker, please refer to [`docker-compose.yml`](docker-compose.yml).

## Protocol

The Things Network's `gateway-connector` protocol sends protocol buffers over MQTT.

- Connect to MQTT with your gateway's ID as username and Access Key as password.
  - On MQTT brokers that don't support authentication, you can connect without authentication.
- After connect: send [`types.ConnectMessage`](types/types.proto) on topic `connect`.
  - Supply the gateway's ID and Access Key to authenticate with the backend
- On disconnect: send [`types.DisconnectMessage`](types/types.proto) on topic `disconnect`.
  - Supply the same ID and Access Key as in the `ConnectMessage`.
  - Use the "will" feature of MQTT to send the `DisconnectMessage` when the gateway unexpectedly disconnects.
- On uplink: send [`router.UplinkMessage`](https://github.com/TheThingsNetwork/api/blob/master/router/router.proto) on topic `<gateway-id>/up`.
- For downlink: subscribe to topic `<gateway-id>/down` and receive [`router.DownlinkMessage`](https://github.com/TheThingsNetwork/api/blob/master/router/router.proto).
- On status: send [`gateway.Status`](https://github.com/TheThingsNetwork/api/blob/master/gateway/gateway.proto) on topic `<gateway-id>/status`.

## Security

⚠️ MQTT brokers should support authentication and access control:

- The `connect`, `disconnect`, `<gateway-id>/up`, `<gateway-id>/status` topics **must only allow**
  - **publish** for authenticated gateways with `<gateway-id>`.
  - **subscribe** for the bridge.
- The `<gateway-id>/down` topics **must only allow**
  - **publish** for the bridge.
  - **subscribe** for authenticated gateways with `<gateway-id>`.

## Development

- Make sure you have [Go](https://golang.org) installed (version 1.7 or later).
- Set up your [Go environment](https://golang.org/doc/code.html#GOPATH).
- Make sure you have [Redis](http://redis.io/download) **installed** and **running**.
- Make sure you have [RabbitMQ](https://www.rabbitmq.com/download.html) and its [MQTT plugin](https://www.rabbitmq.com/mqtt.html) **installed** and **running**.
- Fork this repository on Github
- `git clone git@github.com:YOURUSERNAME/gateway-connector-bridge.git $GOPATH/src/github.com/TheThingsNetwork/gateway-connector-bridge.git`
- `cd $GOPATH/src/github.com/TheThingsNetwork/gateway-connector-bridge`
- `make dev-deps`
- `make test`
- `make build`

## License

Source code for The Things Network is released under the MIT License, which can be found in the [LICENSE](LICENSE) file. A list of authors can be found in the [AUTHORS](AUTHORS) file.
