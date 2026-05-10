package route

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/neochaotic/powerlab/backend/common/external"
	message_bus "github.com/neochaotic/powerlab/backend/user-service/codegen/message_bus"
	"github.com/neochaotic/powerlab/backend/user-service/model"
	"github.com/neochaotic/powerlab/backend/user-service/pkg/config"
	"github.com/neochaotic/powerlab/backend/user-service/service"
	"golang.org/x/net/websocket"
)

// EventListen subscribes to the message-bus websocket for the
// "local-storage" topic and persists every event into the user-service
// EventModel store via service.MyService.Event().CreateEvemt.
//
// It runs as a background goroutine spawned from main() under
// pkglifecycle.SafeGo so a panic anywhere in the websocket loop is
// recovered and logged rather than killing the process. The function
// retries connection up to 1000 times with a 1s backoff between
// attempts; once a connection is established it loops forever reading
// messages until the websocket closes.
//
// Filtering: events named "local-storage:raid_status" are dropped to
// avoid persisting noisy periodic status updates that have no user
// value.
func EventListen() {
	ctx := context.Background()

	for i := 0; i < 1000; i++ {

		messageBusUrl, err := external.GetMessageBusAddress(config.CommonInfo.RuntimePath)
		if err != nil {
			_log.Error(ctx, "get message bus url error", err)
			return
		}

		wsURL := fmt.Sprintf("ws://%s/event/%s", strings.ReplaceAll(messageBusUrl, "http://", ""), "local-storage")
		ws, err := websocket.Dial(wsURL, "", "http://localhost")
		if err != nil {
			_log.Error(ctx, "connect websocket err"+strconv.Itoa(i), err)
			time.Sleep(time.Second * 1)
			continue
		}
		defer ws.Close()

		_log.Info(ctx, "subscribed to", slog.String("url", wsURL))
		for {
			msg := make([]byte, 1024)
			n, err := ws.Read(msg)
			if err != nil {
				// Read failure means the websocket is dead (peer closed,
				// connection reset, EOF). Bail out of the inner loop so
				// the outer reconnect loop fires. The previous code logged
				// + fell through, which then crashed in parseEventPayload
				// when the (zero-length) message had no Uuid. See #160.
				_log.Error(ctx, "websocket read error — reconnecting", err)
				break
			}

			m, err := parseEventPayload(msg[:n])
			if err != nil {
				_log.Error(ctx, "skipping malformed event", err)
				continue
			}
			if m.Name == "local-storage:raid_status" {
				continue
			}
			service.MyService.Event().CreateEvemt(*m)
		}
	}
	_log.Error(ctx, "error when try to connect to message bus", nil)
}

// parseEventPayload converts a raw message-bus payload to an EventModel.
// Returns an error rather than panicking on malformed input — the
// caller's loop must continue regardless of any single bad message.
//
// Sprint 4 fix (#160): extracted from the inline EventListen loop body
// because the original code had three nil-deref paths that combined to
// crash the goroutine on every message-bus disconnect:
//
//   1. ws.Read err → no continue, fell through to unmarshal of zero
//      bytes
//   2. json.Unmarshal err → no continue, fell through to *event.Uuid
//   3. event.Uuid == nil even when unmarshal succeeded (no uuid field
//      in payload) → nil-pointer deref panic at the assignment
//
// SafeGo recovered the panic, so the process kept running, but the
// goroutine died on every restart cycle and a real bug stayed buried.
// All three paths are now error-returning here, with regression test
// coverage in event_listen_test.go.
func parseEventPayload(raw []byte) (*model.EventModel, error) {
	var event message_bus.Event
	if err := json.Unmarshal(raw, &event); err != nil {
		return nil, fmt.Errorf("unmarshal event payload: %w", err)
	}
	if event.Uuid == nil {
		return nil, fmt.Errorf("event payload missing required uuid field")
	}
	propertiesStr, err := json.Marshal(event.Properties)
	if err != nil {
		return nil, fmt.Errorf("marshal event properties: %w", err)
	}
	return &model.EventModel{
		SourceID:   event.SourceID,
		Name:       event.Name,
		Properties: string(propertiesStr),
		UUID:       *event.Uuid,
	}, nil
}
