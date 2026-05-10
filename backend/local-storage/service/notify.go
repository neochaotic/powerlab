package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/neochaotic/powerlab/backend/local-storage/common"
)

// NotifyServer fans local-storage events out to the message-bus.
// Used by disk hot-plug + format completion paths to broadcast
// "powerlab:local-storage:disk_added" and friends to subscribers.
type NotifyServer interface {
	// SendNotify publishes name as a message-bus event with the
	// given payload. Errors from the bus are logged; the bool
	// return is the underlying HTTP error (caller may ignore).
	SendNotify(name string, message map[string]interface{}) error
}

type notifyServer struct {
}

func (i *notifyServer) SendNotify(name string, message map[string]interface{}) error {
	msg := make(map[string]string)
	for k, v := range message {
		bt, _ := json.Marshal(v)
		msg[k] = string(bt)
	}
	response, err := MyService.MessageBus().PublishEventWithResponse(context.Background(), common.ServiceName, name, msg)
	if err != nil {
		_log.Error(context.Background(), "failed to publish event to message bus", err, slog.Any("event", msg))
		return err
	}
	if response.StatusCode() != http.StatusOK {
		_log.Error(context.Background(), "failed to publish event to message bus", nil, slog.String("status", response.Status()), slog.Any("response", response))
	}
	// SocketServer.BroadcastToRoom("/", "public", path, message)
	return nil
}

// NewNotifyService constructs a NotifyServer. Stateless — safe to
// share across goroutines; the underlying message-bus client is
// resolved per-call via MyService.MessageBus().
func NewNotifyService() NotifyServer {
	return &notifyServer{}
}
