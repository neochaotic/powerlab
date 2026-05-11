package service

import (
	"context"
	"log/slog"
	"strings"

	socketio "github.com/CorrectRoadH/go-socket.io"
	"github.com/CorrectRoadH/go-socket.io/engineio"
	"github.com/CorrectRoadH/go-socket.io/engineio/transport"
	"github.com/CorrectRoadH/go-socket.io/engineio/transport/polling"
	"github.com/CorrectRoadH/go-socket.io/engineio/transport/websocket"
	"github.com/neochaotic/powerlab/backend/message-bus/config"
	"github.com/neochaotic/powerlab/backend/message-bus/model"
)

// SocketIOService bridges the in-process event/action bus to legacy
// CasaOS-shaped socketio clients (CasaOS UI + zimaos UI gen 1).
// Keeps a single socketio.Server with two rooms ("event", "action")
// and broadcasts model.Event / model.Action by Name.
type SocketIOService struct {
	server *socketio.Server
}

// Publish dispatches message to the matching socketio room. Accepts
// model.Event (-> "event" room) and model.Action (-> "action" room);
// any other type is logged at error level and dropped.
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

// Start runs the underlying socketio server. Blocks until the
// server errors out — invoke as a goroutine.
func (s *SocketIOService) Start(ctx *context.Context) {
	if err := s.server.Serve(); err != nil {
		_log.Error(context.Background(), "error when serving socketio for events", err)
	}
}

// Server returns the wrapped socketio server so the route layer
// can mount it onto an http.Handler tree.
func (s *SocketIOService) Server() *socketio.Server {
	return s.server
}

// NewSocketIOService constructs a SocketIOService with the default
// websocket + polling transports. CheckOrigin enforces the #219
// allowlist: same-origin always, plus any operator-configured
// origins from `[security] AllowedOrigins` in message-bus.conf.
func NewSocketIOService() *SocketIOService {
	return &SocketIOService{
		server: buildServer(),
	}
}

// parseAllowedOrigins splits the comma-separated `[security]
// AllowedOrigins` config value into a normalized slice. Whitespace
// around each entry is trimmed; blank entries are dropped.
func parseAllowedOrigins(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func buildServer() *socketio.Server {
	// #219: WebSocket + polling now share an allowlist-based
	// CheckOrigin. Same-origin requests pass without explicit
	// configuration; cross-origin callers must be listed in
	// `[security] AllowedOrigins` of message-bus.conf.
	checkOrigin := newOriginChecker(parseAllowedOrigins(config.SecurityInfo.AllowedOrigins))

	websocketTransport := websocket.Default
	websocketTransport.CheckOrigin = checkOrigin

	pollingTransport := polling.Default
	pollingTransport.CheckOrigin = checkOrigin

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
