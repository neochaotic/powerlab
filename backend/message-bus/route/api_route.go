package route

import (
	jsoniter "github.com/json-iterator/go"
	"github.com/neochaotic/powerlab/backend/message-bus/codegen"
	"github.com/neochaotic/powerlab/backend/message-bus/service"
)

type APIRoute struct {
	services *service.Services
}

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func NewAPIRoute(services *service.Services) codegen.ServerInterface {
	return &APIRoute{
		services: services,
	}
}
