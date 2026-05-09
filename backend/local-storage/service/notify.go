package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/IceWhaleTech/CasaOS-LocalStorage/common"
)

type NotifyServer interface {
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

func NewNotifyService() NotifyServer {
	return &notifyServer{}
}
