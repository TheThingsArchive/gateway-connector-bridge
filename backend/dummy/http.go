// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package dummy

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/TheThingsNetwork/gateway-connector-bridge/types"
	"github.com/apex/log"
	"github.com/googollee/go-socket.io"
)

const (
	room          = "evts"
	connectEvt    = "gtw-connect"
	disconnectEvt = "gtw-disconnect"
	uplinkEvt     = "uplink"
	downlinkEvt   = "downlink"
	statusEvt     = "status"
)

// Server is a http server that exposes some events over websockets
type Server struct {
	ctx        log.Interface
	addr       string
	server     *socketio.Server
	connect    chan string
	disconnect chan string
	uplink     chan *types.UplinkMessage
	downlink   chan *types.DownlinkMessage
	status     chan *types.StatusMessage

	mu                sync.RWMutex // Protects connectedGateways
	connectedGateways []string
}

// NewServer creates a new server
func NewServer(ctx log.Interface, addr string) (*Server, error) {
	server, err := socketio.NewServer(nil)
	if err != nil {
		return nil, err
	}

	return &Server{
		ctx:               ctx.WithField("Connector", "Dummy-HTTP"),
		server:            server,
		addr:              addr,
		connect:           make(chan string, BufferSize),
		disconnect:        make(chan string, BufferSize),
		uplink:            make(chan *types.UplinkMessage, BufferSize),
		downlink:          make(chan *types.DownlinkMessage, BufferSize),
		status:            make(chan *types.StatusMessage, BufferSize),
		connectedGateways: make([]string, 0),
	}, nil
}

// Listen opens the server and starts listening for http requests
func (s *Server) Listen() {
	s.server.On("connection", func(so socketio.Socket) {
		s.handleConnect(so)
	})

	go s.handleEvents()

	mux := http.NewServeMux()
	mux.Handle("/socket.io/", s.server)
	mux.Handle("/", http.FileServer(http.Dir("./assets")))
	mux.HandleFunc("/gateways", func(res http.ResponseWriter, _ *http.Request) {
		res.Header().Add("content-type", "application/json; charset=utf-8")
		enc := json.NewEncoder(res)
		enc.Encode(s.ConnectedGateways())
	})
	s.ctx.Infof("HTTP server listening on %s", s.addr)
	err := http.ListenAndServe(s.addr, mux)
	if err != nil {
		s.ctx.WithError(err).Fatal("Could not serve HTTP")
	}
}

func (s *Server) handleConnect(so socketio.Socket) {
	ctx := s.ctx.WithField("ID", so.Id)
	ctx.Debug("Socket connected")
	so.Join(room)
	so.On("disconnection", func() {
		ctx.Debug("Socket disconnected")
	})
}

func (s *Server) handleEvents() {
	for {
		select {
		case gtwID := <-s.connect:
			s.emit(connectEvt, gtwID)
		case gtwID := <-s.disconnect:
			s.emit(disconnectEvt, gtwID)
		case msg := <-s.uplink:
			s.emit(uplinkEvt, msg)
		case msg := <-s.downlink:
			s.emit(downlinkEvt, msg)
		case msg := <-s.status:
			s.emit(statusEvt, msg)
		}
	}
}

func (s *Server) emit(name string, v interface{}) {
	marshalled, err := json.Marshal(v)
	if err != nil {
		s.ctx.WithError(err).Error("Could not marshal event")
		return
	}
	s.server.BroadcastTo(room, name, string(marshalled))
}

// Uplink emits an uplink message on the server page
func (s *Server) Uplink(msg *types.UplinkMessage) {
	select {
	case s.uplink <- msg:
		return
	default:
		s.ctx.Warn("Dropping uplink message on websocket")
	}
}

// Downlink emits a downlink message on the server page
func (s *Server) Downlink(msg *types.DownlinkMessage) {
	select {
	case s.downlink <- msg:
		return
	default:
		s.ctx.Warn("Dropping downlink message on websocket")
	}
}

// Status emits a status message on the server page
func (s *Server) Status(msg *types.StatusMessage) {
	select {
	case s.status <- msg:
		return
	default:
		s.ctx.Warn("Dropping status message on websocket")
	}
}

// Connect emits a message when a gateway connects
func (s *Server) Connect(gatewayID string) {
	select {
	case s.connect <- gatewayID:
	default:
		s.ctx.Warn("Dropping connect message on websocket")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.connectedGateways {
		if existing == gatewayID {
			return
		}
	}
	s.connectedGateways = append(s.connectedGateways, gatewayID)
}

// Disconnect emits a message when a gateway disconnects
func (s *Server) Disconnect(gatewayID string) {
	select {
	case s.disconnect <- gatewayID:
	default:
		s.ctx.Warn("Dropping disconnect message on websocket")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	previouslyConnectedGateways := s.connectedGateways
	s.connectedGateways = make([]string, 0, len(previouslyConnectedGateways))
	for _, existing := range previouslyConnectedGateways {
		if existing != gatewayID {
			s.connectedGateways = append(s.connectedGateways, existing)
		}
	}
}

// ConnectedGateways returns the list of connected gateway IDs
func (s *Server) ConnectedGateways() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connectedGateways
}
