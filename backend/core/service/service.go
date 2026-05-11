package service

import (
	"fmt"

	"github.com/gorilla/websocket"
	"github.com/neochaotic/powerlab/backend/common/external"
	"github.com/neochaotic/powerlab/backend/core/codegen/message_bus"
	"github.com/neochaotic/powerlab/backend/core/pkg/config"
	"github.com/patrickmn/go-cache"
	"gorm.io/gorm"
)

// Cache is the package-level in-memory cache shared by service code
// for ephemeral state (system metrics snapshots, file-list caches)
// that's expensive to recompute but doesn't need to survive a
// restart.
var Cache *cache.Cache

// MyService is the package-level Repository container set up by
// NewService at process start. Route handlers reach into it
// directly — there's no per-request injection layer.
var (
	MyService Repository
)

// WebSocket bookkeeping for the legacy /v1/sys/socket endpoint —
// list of active connections + a flag set when the broadcaster
// goroutine is alive.
var (
	WebSocketConns []*websocket.Conn
	SocketRun      bool
)

// Repository is the dependency container exposed to the route
// layer. Each method returns a long-lived collaborator constructed
// once at startup. Test code can satisfy the interface with stubs.
type Repository interface {
	// Gateway returns the gateway management SDK client (route
	// registration on boot).
	Gateway() external.ManagementService
	// Health returns the health-check service used by the
	// readiness probe + admin diagnostic page.
	Health() HealthService
	// Notify returns the in-process notification fan-out used by
	// system events to reach the message-bus + the legacy
	// websocket broadcaster.
	Notify() NotifyServer
	// Rely returns the dependency-tracking service used by the
	// install/upgrade gates ("can app X be installed?").
	Rely() RelyService
	// System returns the system-info + utilization service that
	// powers the homepage stats widgets.
	System() SystemService
	// MessageBus returns the codegen message-bus REST client.
	MessageBus() *message_bus.ClientWithResponses
	// Peer returns the local-network peer-discovery service used
	// by the multi-device pairing flow.
	Peer() PeerService
	// Other holds odds-and-ends helpers that don't deserve their
	// own service — kept narrow to discourage growth.
	Other() OtherService
}

// NewService wires the Repository container. Tolerates gateway-
// management bring-up failure for local development (the gateway
// isn't always running in a dev shell); in prod the panic comes
// later when route registration tries to fire.
func NewService(db *gorm.DB, RuntimePath string) Repository {
	gatewayManagement, err := external.NewManagementService(RuntimePath)
	if err != nil && len(RuntimePath) > 0 {
		// Ignore panic for local development where Gateway is not running
		fmt.Println("Warning: Failed to connect to Gateway. Ignoring for local testing. Error:", err)
	}

	return &store{
		gateway: gatewayManagement,
		notify:  NewNotifyService(db),
		rely:    NewRelyService(db),
		system:  NewSystemService(),
		health:  NewHealthService(),
		other:   NewOtherService(),

		peer: NewPeerService(db),
	}
}

// store is the unexported Repository implementation — fields are
// the concrete collaborators wired by NewService.
type store struct {
	peer    PeerService
	db      *gorm.DB
	notify  NotifyServer
	rely    RelyService
	system  SystemService
	gateway external.ManagementService
	health  HealthService
	other   OtherService
}

func (c *store) Peer() PeerService {
	return c.peer
}

func (c *store) Other() OtherService {
	return c.other
}

func (c *store) Gateway() external.ManagementService {
	return c.gateway
}

func (c *store) Rely() RelyService {
	return c.rely
}

func (c *store) System() SystemService {
	return c.system
}

func (c *store) Notify() NotifyServer {
	return c.notify
}

func (c *store) Health() HealthService {
	return c.health
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
