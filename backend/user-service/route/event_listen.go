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
				_log.Error(ctx, "websocket read error", err)
			}

			var event message_bus.Event

			if err := json.Unmarshal(msg[:n], &event); err != nil {
				_log.Error(ctx, "websocket payload unmarshal error", err)
			}
			propertiesStr, err := json.Marshal(event.Properties)
			if err != nil {
				_log.Error(ctx, "marshal error", err, slog.Any("event", event))
				continue
			}
			model := model.EventModel{
				SourceID:   event.SourceID,
				Name:       event.Name,
				Properties: string(propertiesStr),
				UUID:       *event.Uuid,
			}
			if event.Name == "local-storage:raid_status" {
				continue
			}
			service.MyService.Event().CreateEvemt(model)
		}
	}
	_log.Error(ctx, "error when try to connect to message bus", nil)
}
