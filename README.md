# The Things Network Gateway Connector Bridge

[![Build Status](https://travis-ci.org/TheThingsNetwork/gateway-connector-bridge.svg?branch=master)](https://travis-ci.org/TheThingsNetwork/gateway-connector-bridge) [![Coverage Status](https://coveralls.io/repos/github/TheThingsNetwork/gateway-connector-bridge/badge.svg?branch=master)](https://coveralls.io/github/TheThingsNetwork/gateway-connector-bridge?branch=master)

## Getting Started

- Make sure you have [Go](https://golang.org) installed (version 1.7 or later).
- Set up your [Go environment](https://golang.org/doc/code.html#GOPATH)
- Make sure you have [RabbitMQ](https://www.rabbitmq.com/download.html) and its [MQTT plugin](https://www.rabbitmq.com/mqtt.html) **installed** and **running**.  
- Fork this repository on Github
- `git clone git@github.com:YOURUSERNAME/gateway-connector-bridge.git $GOPATH/src/github.com/TheThingsNetwork/gateway-connector-bridge.git`
- `cd $GOPATH/src/github.com/TheThingsNetwork/gateway-connector-bridge`
- `make dev-deps`
- `make test`
- `make build`

## License

Source code for The Things Network is released under the MIT License, which can be found in the [LICENSE](LICENSE) file. A list of authors can be found in the [AUTHORS](AUTHORS) file.
