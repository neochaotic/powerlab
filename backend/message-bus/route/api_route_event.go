package route

import (
	"context"
	"fmt"
	"io/ioutil"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/neochaotic/powerlab/backend/common/utils"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/neochaotic/powerlab/backend/message-bus/codegen"
	"github.com/neochaotic/powerlab/backend/message-bus/common"
	"github.com/neochaotic/powerlab/backend/message-bus/model"
	"github.com/neochaotic/powerlab/backend/message-bus/route/adapter/in"
	"github.com/neochaotic/powerlab/backend/message-bus/route/adapter/out"
)

// GetEventTypes returns every registered event type.
//
// Route: GET /v2/message_bus/event_type
func (r *APIRoute) GetEventTypes(ctx echo.Context) error {
	eventTypes, err := r.services.EventTypeService.GetEventTypes()
	if err != nil {
		message := err.Error()
		return ctx.JSON(http.StatusInternalServerError, codegen.ResponseInternalServerError{Message: &message})
	}

	results := make([]codegen.EventType, 0)

	for _, eventType := range eventTypes {
		results = append(results, out.EventTypeAdapter(eventType))
	}

	return ctx.JSON(http.StatusOK, results)
}

// RegisterEventTypes upserts each event type in the request body.
// Idempotent — publishers call on every startup.
//
// Route: POST /v2/message_bus/event_type
func (r *APIRoute) RegisterEventTypes(ctx echo.Context) error {
	var eventTypes []codegen.EventType
	if err := ctx.Bind(&eventTypes); err != nil {
		message := err.Error()
		return ctx.JSON(http.StatusBadRequest, codegen.ResponseBadRequest{Message: &message})
	}

	for _, eventType := range eventTypes {
		_, err := r.services.EventTypeService.RegisterEventType(in.EventTypeAdapter(eventType))
		if err != nil {
			message := err.Error()
			return ctx.JSON(http.StatusBadRequest, codegen.ResponseBadRequest{Message: &message})
		}
	}

	return ctx.JSON(http.StatusOK, codegen.ResponseOK{})
}

// GetEventTypesBySourceID returns every event type registered by
// the given publisher.
//
// Route: GET /v2/message_bus/event_type/{source_id}
func (r *APIRoute) GetEventTypesBySourceID(ctx echo.Context, sourceID codegen.SourceID) error {
	results, err := r.services.EventTypeService.GetEventTypesBySourceID(sourceID)
	if err != nil {
		message := err.Error()
		return ctx.JSON(http.StatusBadRequest, codegen.ResponseBadRequest{Message: &message})
	}

	return ctx.JSON(http.StatusOK, results)
}

// GetEventType returns the event type identified by (sourceID, name)
// or 404 if absent.
//
// Route: GET /v2/message_bus/event_type/{source_id}/{name}
func (r *APIRoute) GetEventType(ctx echo.Context, sourceID codegen.SourceID, name codegen.EventName) error {
	result, err := r.services.EventTypeService.GetEventType(sourceID, name)
	if err != nil {
		message := err.Error()
		return ctx.JSON(http.StatusNotFound, codegen.ResponseNotFound{Message: &message})
	}

	if result == nil {
		return ctx.JSON(http.StatusNotFound, codegen.ResponseNotFound{Message: utils.Ptr("not found")})
	}

	return ctx.JSON(http.StatusOK, result)
}

// PublishEvent fans an event out to socketio clients + WS
// subscribers. Generates a UUID for de-dup. Body is the
// Properties map.
//
// Route: POST /v2/message_bus/event/{source_id}/{name}
func (r *APIRoute) PublishEvent(ctx echo.Context, sourceID codegen.SourceID, name codegen.EventName) error {
	var properties map[string]string
	body, err := ioutil.ReadAll(ctx.Request().Body)
	if err != nil {

		message := err.Error()
		return ctx.JSON(http.StatusBadRequest, codegen.ResponseBadRequest{Message: &message})
	}

	if err = json.Unmarshal(body, &properties); err != nil {
		message := err.Error()
		return ctx.JSON(http.StatusBadRequest, codegen.ResponseBadRequest{Message: &message})
	}

	uuidStr := uuid.New().String()
	event := in.EventAdapter(codegen.Event{
		SourceID:   sourceID,
		Name:       name,
		Properties: properties,
		Timestamp:  utils.Ptr(time.Now()),
		Uuid:       &uuidStr,
	})

	go r.services.SocketIOService.Publish(event)
	go r.services.EventServiceWS.Publish(event)

	return ctx.JSON(http.StatusOK, out.EventAdapter(event))
}

// SubscribeEventWS upgrades the request to a WebSocket and streams
// matching events to the caller. Filters on sourceID + optional
// names list (all event types for the publisher when nil). Sends
// WS PING frames in place of the bus heartbeat.
//
// Route: GET /v2/message_bus/event/{source_id} (Upgrade: websocket)
func (r *APIRoute) SubscribeEventWS(c echo.Context, sourceID codegen.SourceID, params codegen.SubscribeEventWSParams) error {
	var eventNames []string
	if params.Names != nil {
		for _, eventName := range *params.Names {
			eventType, err := r.services.EventTypeService.GetEventType(sourceID, eventName)
			if err != nil {
				message := err.Error()
				return c.JSON(http.StatusBadRequest, codegen.ResponseBadRequest{Message: &message})
			}

			if eventType == nil {
				return c.JSON(http.StatusBadRequest, codegen.ResponseBadRequest{Message: utils.Ptr(fmt.Sprintf("event type `%s` of source ID `%s` not found", eventName, sourceID))})
			}

			eventNames = append(eventNames, eventName)
		}
	} else {
		eventTypes, err := r.services.EventTypeService.GetEventTypesBySourceID(sourceID)
		if err != nil || len(eventTypes) == 0 {
			if err != nil {
				message := err.Error()
				return c.JSON(http.StatusBadRequest, codegen.ResponseBadRequest{Message: &message})
			}
			message := "event types not found"
			return c.JSON(http.StatusBadRequest, codegen.ResponseBadRequest{Message: &message})
		}

		for _, eventType := range eventTypes {
			eventNames = append(eventNames, eventType.Name)
		}
	}

	conn, _, _, err := ws.UpgradeHTTP(c.Request(), c.Response())
	if err != nil {
		message := err.Error()
		return c.JSON(http.StatusInternalServerError, codegen.ResponseInternalServerError{Message: &message})
	}

	channel, err := r.services.EventServiceWS.Subscribe(sourceID, eventNames)
	if err != nil {
		conn.Close() // need to close connection here, instead of defer, because of the goroutine
		message := err.Error()
		return c.JSON(http.StatusInternalServerError, codegen.ResponseInternalServerError{Message: &message})
	}

	go func(conn net.Conn, channel chan model.Event, eventNames []string) {
		defer conn.Close()
		defer close(channel)
		defer func(eventNames []string) {
			for _, name := range eventNames {
				if err := r.services.EventServiceWS.Unsubscribe(sourceID, name, channel); err != nil {
					_log.Error(context.Background(), "error when trying to unsubscribe an event type via websocket", err, slog.String("source_id", sourceID), slog.String("name", name))
				}
			}
		}(eventNames)

		_log.Info(context.Background(), "a websocket connection has started for events", slog.String("remote_addr", conn.RemoteAddr().String()))

		for {
			event, ok := <-channel
			if !ok {
				_log.Info(context.Background(), "websocket channel for events is closed")
				return
			}

			if event.SourceID == common.MessageBusSourceID && event.Name == common.MessageBusHeartbeatName {
				if err := wsutil.WriteServerMessage(conn, ws.OpPing, []byte{}); err != nil {
					_log.Error(context.Background(), "error when trying to send ping message via websocket", err)
					return
				}
				continue
			}

			message, err := json.Marshal(out.EventAdapter(event))
			if err != nil {
				_log.Error(context.Background(), "error when trying to marshal event for websocket", err)
				continue
			}

			_log.Info(context.Background(), "sending event via websocket", slog.String("remote_addr", conn.RemoteAddr().String()), slog.String("message", string(message)))

			if err := wsutil.WriteServerText(conn, message); err != nil {
				if _, ok := err.(*net.OpError); ok {
					_log.Info(context.Background(), "websocket connection ended", slog.String("error", err.Error()))
				} else {
					_log.Error(context.Background(), "error when sending event via websocket", err)
				}
				return
			}
		}
	}(conn, channel, eventNames)

	return nil
}
