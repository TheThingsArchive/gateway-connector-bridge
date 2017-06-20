// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package statusserver

import (
	"github.com/TheThingsNetwork/gateway-connector-bridge/status"
	"github.com/TheThingsNetwork/go-utils/grpc/ttnctx"
	"github.com/TheThingsNetwork/ttn/api"
	"github.com/TheThingsNetwork/ttn/api/stats"
	"github.com/rcrowley/go-metrics"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

var global = &statusServer{
	uplink:            metrics.NewMeter(),
	downlink:          metrics.NewMeter(),
	gatewayStatus:     metrics.NewMeter(),
	connectedGateways: metrics.NewCounter(),
}

type statusServer struct {
	accessKeys []string

	uplink            metrics.Meter
	downlink          metrics.Meter
	gatewayStatus     metrics.Meter
	connectedGateways metrics.Counter
}

func (s *statusServer) AddAccessKey(key string) {
	s.accessKeys = append(s.accessKeys, key)
}

// AddAccessKey adds an access key for a client
func AddAccessKey(key string) {
	global.AddAccessKey(key)
}

func (s *statusServer) Uplink() {
	s.uplink.Mark(1)
}

// Uplink registers an uplink message in the default status server
func Uplink() {
	global.Uplink()
}

func (s *statusServer) Downlink() {
	s.downlink.Mark(1)
}

// Downlink registers a downlink message in the default status server
func Downlink() {
	global.Downlink()
}

func (s *statusServer) GatewayStatus() {
	s.gatewayStatus.Mark(1)
}

// GatewayStatus registers a gateway status in the default status server
func GatewayStatus() {
	global.GatewayStatus()
}

func (s *statusServer) ConnectGateway() {
	s.connectedGateways.Inc(1)
}

// ConnectGateway registers a gateway connection in the default status server
func ConnectGateway() {
	global.ConnectGateway()
}

func (s *statusServer) DisconnectGateway() {
	s.connectedGateways.Dec(1)
}

// DisconnectGateway registers a gateway disconnection in the default status server
func DisconnectGateway() {
	global.DisconnectGateway()
}

func (s *statusServer) getStatus() *status.StatusResponse {
	status := new(status.StatusResponse)
	status.System = stats.GetSystem()
	status.Component = stats.GetComponent()
	uplink := s.uplink.Snapshot()
	status.Uplink = &api.Rates{
		Rate1:  float32(uplink.Rate1()),
		Rate5:  float32(uplink.Rate5()),
		Rate15: float32(uplink.Rate15()),
	}
	downlink := s.downlink.Snapshot()
	status.Downlink = &api.Rates{
		Rate1:  float32(downlink.Rate1()),
		Rate5:  float32(downlink.Rate5()),
		Rate15: float32(downlink.Rate15()),
	}
	gatewayStatus := s.gatewayStatus.Snapshot()
	status.GatewayStatus = &api.Rates{
		Rate1:  float32(gatewayStatus.Rate1()),
		Rate5:  float32(gatewayStatus.Rate5()),
		Rate15: float32(gatewayStatus.Rate15()),
	}
	status.ConnectedGateways = uint32(s.connectedGateways.Snapshot().Count())
	return status
}

func (s *statusServer) GetStatus(ctx context.Context, _ *status.StatusRequest) (*status.StatusResponse, error) {
	if len(s.accessKeys) != 0 {
		key, err := ttnctx.KeyFromIncomingContext(ctx)
		if err != nil {
			return nil, err
		}
		for _, allowed := range s.accessKeys {
			if key == allowed {
				return s.getStatus(), nil
			}
		}
		return nil, grpc.Errorf(codes.Unauthenticated, "Not authenticated")
	}
	return s.getStatus(), nil
}

func (s *statusServer) Register(srv *grpc.Server) {
	status.RegisterStatusServer(srv, s)
}

// Register the default status server
func Register(srv *grpc.Server) {
	global.Register(srv)
}
