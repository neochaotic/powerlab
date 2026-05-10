package service

import (
	"github.com/neochaotic/powerlab/backend/common/external"
	"github.com/neochaotic/powerlab/backend/user-service/codegen/message_bus"
	"github.com/neochaotic/powerlab/backend/user-service/pkg/config"
	"gorm.io/gorm"
)

var MyService Repository

// Repository bundles the user-service's live services + the thin
// clients it needs to talk to other services. Stored as the
// package-level singleton MyService so route handlers can reach
// any subsystem via service.MyService.<sub>().
type Repository interface {
	Gateway() external.ManagementService
	User() UserService
	OS() *OSService
	MessageBus() *message_bus.ClientWithResponses
	Event() EventService
}

// NewService constructs the user-service Repository wired to the
// supplied gorm.DB (user.db, owned by this service) + the runtime
// socket dir (where gateway/message-bus URL files live). Panics if
// the gateway socket file isn't readable — user-service can't
// register routes without it, so failing fast is correct.
func NewService(db *gorm.DB, RuntimePath string) Repository {

	gatewayManagement, err := external.NewManagementService(RuntimePath)
	if err != nil {
		panic(err)
	}

	return &store{
		gateway: gatewayManagement,
		user:    NewUserService(db),
		event:   NewEventService(db),
	}
}

type store struct {
	gateway external.ManagementService
	user    UserService
	event   EventService
}

func (c *store) Event() EventService {
	return c.event
}
func (c *store) Gateway() external.ManagementService {
	return c.gateway
}

func (c *store) User() UserService {
	return c.user
}
func (c *store) OS() *OSService {
	return &OSService{}
}
func (c *store) MessageBus() *message_bus.ClientWithResponses {
	client, _ := message_bus.NewClientWithResponses("", func(c *message_bus.Client) error {
		// error will never be returned, as we always want to return a client, even with wrong address,
		// in order to avoid panic.
		//
		// If we don't avoid panic, message bus becomes a hard dependency, which is not what we want.

		messageBusAddress, err := external.GetMessageBusAddress(config.CommonInfo.RuntimePath)
		if err != nil {
			c.Server = "message bus address not found"
			return nil
		}

		c.Server = messageBusAddress
		return nil
	})

	return client
}
