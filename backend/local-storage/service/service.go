package service

import (
	"github.com/neochaotic/powerlab/backend/common/external"
	"github.com/neochaotic/powerlab/backend/local-storage/codegen/message_bus"
	"github.com/neochaotic/powerlab/backend/local-storage/pkg/config"
	v2 "github.com/neochaotic/powerlab/backend/local-storage/service/v2"
	"github.com/neochaotic/powerlab/backend/local-storage/service/v2/wrapper"
	"github.com/patrickmn/go-cache"
	"gorm.io/gorm"
)

// Cache is the package-level in-memory cache shared by the disk +
// USB services for ephemeral state (smartctl readings, mount-table
// snapshots) that's expensive to recompute but doesn't need to
// survive a restart.
var Cache *cache.Cache

// MyService is the package-level Services container set up by
// NewService at process start. Route handlers reach into it
// directly — there's no per-request injection layer.
var MyService Services

// Services is the dependency container exposed to the route layer.
// Each method returns a long-lived collaborator constructed once at
// startup. Test code can satisfy the interface with stubs.
type Services interface {
	Disk() DiskService
	USB() USBService
	LocalStorage() *v2.LocalStorageService
	Gateway() external.ManagementService
	Notify() NotifyServer
	NotifySystem() external.NotifyService
	MessageBus() *message_bus.ClientWithResponses
}

// NewService wires the Services container. Panics on gateway-
// management bring-up failure (without the gateway, this service
// can't register routes — fail fast). Other dependencies are lazy-
// initialised so a temporary upstream outage doesn't kill startup.
func NewService(db *gorm.DB) Services {
	gatewayManagement, err := external.NewManagementService(config.CommonInfo.RuntimePath)
	if err != nil {
		panic(err)
	}

	notifySystem := external.NewNotifyService(config.CommonInfo.RuntimePath)

	return &store{
		usb:          NewUSBService(),
		disk:         NewDiskService(db),
		localStorage: v2.NewLocalStorageService(db, wrapper.NewMountInfo()),
		gateway:      gatewayManagement,
		notify:       NewNotifyService(),
		notifySystem: notifySystem,
	}
}

// store is the unexported Services implementation — fields are the
// concrete collaborators wired by NewService.
type store struct {
	usb          USBService
	disk         DiskService
	localStorage *v2.LocalStorageService
	gateway      external.ManagementService
	notify       NotifyServer
	notifySystem external.NotifyService
}

func (c *store) NotifySystem() external.NotifyService {
	return c.notifySystem
}

func (c *store) Gateway() external.ManagementService {
	return c.gateway
}

func (c *store) USB() USBService {
	return c.usb
}

func (c *store) Disk() DiskService {
	return c.disk
}

func (c *store) LocalStorage() *v2.LocalStorageService {
	return c.localStorage
}

func (c *store) Notify() NotifyServer {
	return c.notify
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
