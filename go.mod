module github.com/TheThingsNetwork/gateway-connector-bridge

go 1.11

replace github.com/brocaar/lorawan => github.com/ThethingsIndustries/legacy-lorawan-lib v0.0.0-20190212122748-b905ab327304

require (
	github.com/TheThingsNetwork/api v0.0.0-20190516145942-631860935c2b
	github.com/TheThingsNetwork/go-account-lib v0.0.0-20190516094738-77d15a3f8875
	github.com/TheThingsNetwork/go-utils v0.0.0-20190516083235-bdd4967fab4e
	github.com/TheThingsNetwork/ttn/api v0.0.0-20190516113615-648de2b33240
	github.com/TheThingsNetwork/ttn/core/types v0.0.0-20190516113615-648de2b33240 // indirect
	github.com/TheThingsNetwork/ttn/utils/errors v0.0.0-20190516113615-648de2b33240 // indirect
	github.com/TheThingsNetwork/ttn/utils/random v0.0.0-20190516113615-648de2b33240
	github.com/TheThingsNetwork/ttn/utils/security v0.0.0-20190516113615-648de2b33240 // indirect
	github.com/apex/log v1.1.0
	github.com/brocaar/lorawan v0.0.0-20190402092148-5bca41b178e9
	github.com/deckarep/golang-set v1.7.1
	github.com/eclipse/paho.mqtt.golang v1.2.0
	github.com/fsnotify/fsnotify v1.4.7
	github.com/gogo/protobuf v1.2.1
	github.com/golang/protobuf v1.3.1
	github.com/googollee/go-engine.io v0.0.0-20170224222511-80ae0e43aca1 // indirect
	github.com/googollee/go-socket.io v0.0.0-20170525141029-5447e71f36d3
	github.com/gorilla/websocket v1.4.0 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/prometheus/client_golang v0.9.3
	github.com/prometheus/procfs v0.0.0-20190516194456-169873baca24 // indirect
	github.com/smartystreets/goconvey v0.0.0-20190330032615-68dc04aab96a
	github.com/spf13/cobra v0.0.3
	github.com/spf13/viper v1.3.2
	github.com/streadway/amqp v0.0.0-20190404075320-75d898a42a94
	golang.org/x/oauth2 v0.0.0-20190402181905-9f3314589c9a // indirect
	google.golang.org/appengine v1.6.0 // indirect
	google.golang.org/genproto v0.0.0-20190516172635-bb713bdc0e52 // indirect
	google.golang.org/grpc v1.20.1
	gopkg.in/redis.v5 v5.2.9
	gopkg.in/yaml.v2 v2.2.2
)
