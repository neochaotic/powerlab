package service

import (
	"context"
	"errors"

	"github.com/neochaotic/powerlab/backend/message-bus/repository"
)

// Services bundles every long-lived service this process owns.
// Constructed once in main, started concurrently via Start, and
// passed by reference into the route layer.
type Services struct {
	EventTypeService *EventTypeService
	EventServiceWS   *EventServiceWS

	ActionTypeService *ActionTypeService
	ActionServiceWS   *ActionServiceWS

	SocketIOService *SocketIOService

	YSKService *YSKService
}

var (
	ErrInboundChannelNotFound     = errors.New("inbound channel not found")
	ErrSubscriberChannelsNotFound = errors.New("subscriber channels not found")
	ErrAlreadySubscribed          = errors.New("already subscribed")
)

// Start launches the long-running goroutines for the WS event/action
// dispatchers and the socketio server. Returns immediately; ctx
// cancellation propagates through to each service's own shutdown
// path.
func (s *Services) Start(ctx *context.Context) {
	go s.EventServiceWS.Start(ctx)
	go s.ActionServiceWS.Start(ctx)

	go s.SocketIOService.Start(ctx)
}

// NewServices constructs the Services container, wiring each
// service to its dependencies. YSKService is wired last so it can
// subscribe to the EventServiceWS that NewEventServiceWS just
// created.
func NewServices(repository *repository.Repository) Services {
	eventTypeService := NewEventTypeService(repository)
	actionTypeService := NewActionTypeService(repository)

	eventServiceWS := NewEventServiceWS(eventTypeService)

	return Services{
		EventTypeService: eventTypeService,
		SocketIOService:  NewSocketIOService(),
		EventServiceWS:   eventServiceWS,

		ActionTypeService: actionTypeService,
		ActionServiceWS:   NewActionServiceWS(actionTypeService),
		YSKService:        NewYSKService(repository, eventServiceWS, eventTypeService),
	}
}
