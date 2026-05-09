package v2

import (
	"github.com/neochaotic/powerlab/backend/core/codegen"
	"github.com/neochaotic/powerlab/backend/core/service"
)

type CasaOS struct {
	fileUploadService *service.FileUploadService
}

func NewCasaOS() codegen.ServerInterface {
	return &CasaOS{
		fileUploadService: service.NewFileUploadService(),
	}
}
