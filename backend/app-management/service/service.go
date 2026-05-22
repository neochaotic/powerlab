package service

import (
	"context"
	"fmt"
	"net/http"

	jsoniter "github.com/json-iterator/go"
	"github.com/neochaotic/powerlab/backend/app-management/codegen/message_bus"
	"github.com/neochaotic/powerlab/backend/app-management/common"
	"github.com/neochaotic/powerlab/backend/app-management/pkg/config"
	"github.com/neochaotic/powerlab/backend/common/external"
	"github.com/neochaotic/powerlab/backend/common/utils/logger"
	"go.uber.org/zap"
)

var (
	MyService Services

	json = jsoniter.ConfigCompatibleWithStandardLibrary
)

type Services interface {
	AppStoreManagement() *AppStoreManagement

	// Git() *GitService
	Compose() *ComposeService
	Docker() DockerService
	Gateway() external.ManagementService
	Notify() external.NotifyService
	MessageBus() *message_bus.ClientWithResponses
}

func NewService(RuntimePath string) Services {
	gatewayManagement, err := external.NewManagementService(RuntimePath)
	if err != nil && len(RuntimePath) > 0 {
		panic(err)
	}

	return &store{
		gateway: gatewayManagement,
		notify:  external.NewNotifyService(RuntimePath),

		appStoreManagement: NewAppStoreManagement(),

		compose: NewComposeService(),
		docker:  NewDockerService(),
	}
}

type store struct {
	appStoreManagement *AppStoreManagement

	// git     *GitService
	compose *ComposeService
	docker  DockerService
	gateway external.ManagementService
	notify  external.NotifyService
}

func (c *store) Gateway() external.ManagementService {
	return c.gateway
}

func (c *store) Notify() external.NotifyService {
	return c.notify
}

func (c *store) AppStoreManagement() *AppStoreManagement {
	return c.appStoreManagement
}

func (c *store) Compose() *ComposeService {
	return c.compose
}

func (c *store) Docker() DockerService {
	return c.docker
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

func PublishEventWrapper(ctx context.Context, eventType message_bus.EventType, properties map[string]string) {
	if MyService == nil {
		fmt.Println("failed to publish event - messsage bus service not initialized")
		return
	}

	if properties == nil {
		properties = map[string]string{}
	}

	// merge with properties from context
	for k, v := range common.PropertiesFromContext(ctx) {
		properties[k] = v
	}

	resp, err := external.PublishEventInSocket(ctx, eventType.SourceID, eventType.Name, properties)
	if err != nil {
		logger.Error("failed to publish event", zap.Error(err))

		response, err := MyService.MessageBus().PublishEventWithResponse(ctx, common.AppManagementServiceName, eventType.Name, properties)
		if err != nil {
			logger.Error("failed to publish event", zap.Error(err))
			return
		}
		defer response.HTTPResponse.Body.Close()

		if response.StatusCode() != http.StatusOK {
			logger.Error("failed to publish event", zap.String("status code", response.Status()))
		}
	} else {
		if resp.StatusCode != http.StatusOK {
			logger.Error("failed to publish event", zap.String("status code", resp.Status))
		}
	}
}
