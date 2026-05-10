package service

import (
	"context"
	"log/slog"
	"net/http"

	socketio "github.com/CorrectRoadH/go-socket.io"
	"github.com/CorrectRoadH/go-socket.io/engineio"
	"github.com/CorrectRoadH/go-socket.io/engineio/transport"
	"github.com/CorrectRoadH/go-socket.io/engineio/transport/polling"
	"github.com/CorrectRoadH/go-socket.io/engineio/transport/websocket"
	"github.com/neochaotic/powerlab/backend/message-bus/model"
)

type SocketIOService struct {
	server *socketio.Server
}

func (s *SocketIOService) Publish(message interface{}) {
	if event, ok := message.(model.Event); ok {
		s.server.BroadcastToRoom("/", "event", event.Name, event)
		return
	}

	if action, ok := message.(model.Action); ok {
		s.server.BroadcastToRoom("/", "action", action.Name, action)
		return
	}

	_log.Error(context.Background(), "unknown message type", nil, slog.Any("message", message))
}

func (s *SocketIOService) Start(ctx *context.Context) {
	if err := s.server.Serve(); err != nil {
		_log.Error(context.Background(), "error when serving socketio for events", err)
	}
}

func (s *SocketIOService) Server() *socketio.Server {
	return s.server
}

func NewSocketIOService() *SocketIOService {
	return &SocketIOService{
		server: buildServer(),
	}
}

func buildServer() *socketio.Server {
	// SECURITY: WebSocket + polling CheckOrigin currently accept ANY
	// Origin. Mitigated by JWT auth on the gateway path (caller must
	// present a valid token to reach the message-bus), but the
	// bypass IS real. Tracked in #219 — do not extend the
	// message-bus's exposed surface until the allowlist lands.
	websocketTransport := websocket.Default
	websocketTransport.CheckOrigin = func(r *http.Request) bool {
		return true // accepts any origin; see #219
	}

	pollingTransport := polling.Default
	pollingTransport.CheckOrigin = func(r *http.Request) bool {
		return true // accepts any origin; see #219
	}

	server := socketio.NewServer(&engineio.Options{
		Transports: []transport.Transport{
			websocketTransport,
			pollingTransport,
		},
	})

	server.OnConnect("/", func(s socketio.Conn) error {
		// TODO add connector info. we need to know who is connecting
		s.SetContext("")
		_log.Info(context.Background(), "a socketio connection has started", slog.Any("remote_addr", s.RemoteAddr()))

		s.Join("event")
		s.Join("action")

		return nil
	})

	server.OnError("/", func(s socketio.Conn, e error) {
		// TODO add connector info. we need to know who is disconnecting
		_log.Error(context.Background(), "error in socketio connnection", e)
	})

	server.OnDisconnect("/", func(s socketio.Conn, reason string) {
		server.Remove(s.ID())
		// TODO add connector info. we need to know who is disconnecting
		_log.Info(context.Background(), "a socketio connection is disconnected", slog.Any("reason", reason))
	})

	return server
}
