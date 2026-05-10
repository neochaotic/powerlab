// Package route holds the message-bus HTTP + WebSocket handlers.
// Handlers implement the oapi-codegen ServerInterface (REST verbs
// over /v2/*) and a hand-written socketio adapter (/v1/socketio/*)
// for legacy CasaOS-shaped clients.
package route

import (
	jsoniter "github.com/json-iterator/go"
	"github.com/neochaotic/powerlab/backend/message-bus/codegen"
	"github.com/neochaotic/powerlab/backend/message-bus/service"
)

// APIRoute is the codegen ServerInterface implementation. It owns
// no state of its own — all logic lives in the underlying Services
// container so the route layer stays a thin HTTP→service shim.
type APIRoute struct {
	services *service.Services
}

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// NewAPIRoute wires the codegen ServerInterface to the message-bus
// Services container.
func NewAPIRoute(services *service.Services) codegen.ServerInterface {
	return &APIRoute{
		services: services,
	}
}
