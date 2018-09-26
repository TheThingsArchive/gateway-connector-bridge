FROM alpine:3.8
RUN apk --update --no-cache add ca-certificates
ADD ./release/gateway-connector-bridge-linux-amd64 /usr/local/bin/gateway-connector-bridge
ADD ./assets ./assets
RUN chmod 755 /usr/local/bin/gateway-connector-bridge
ENTRYPOINT ["/usr/local/bin/gateway-connector-bridge"]
