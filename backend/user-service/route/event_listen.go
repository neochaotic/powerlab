package route

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/IceWhaleTech/CasaOS-Common/external"
	message_bus "github.com/IceWhaleTech/CasaOS-UserService/codegen/message_bus"
	"github.com/IceWhaleTech/CasaOS-UserService/model"
	"github.com/IceWhaleTech/CasaOS-UserService/pkg/config"
	"github.com/IceWhaleTech/CasaOS-UserService/service"
	"golang.org/x/net/websocket"
)

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
