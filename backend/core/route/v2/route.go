// Package v2 implements the PowerLab core /v2 HTTP server. The
// Server type binds the codegen ServerInterface to the handler
// methods scattered across this directory (file.go, etc.). The
// type was previously named `CasaOS` (pre-rebrand, #251); renamed
// to Server in Sprint 9.
package v2

import (
	"github.com/neochaotic/powerlab/backend/core/codegen"
	"github.com/neochaotic/powerlab/backend/core/service"
)

// Server is the v2 HTTP handler set. Implements
// codegen.ServerInterface — every operationId in the OpenAPI spec
// has a method here.
type Server struct {
	fileUploadService *service.FileUploadService
}

// NewServer constructs the v2 server with its collaborator
// services. Returns the codegen.ServerInterface so callers stay
// decoupled from the concrete type.
func NewServer() codegen.ServerInterface {
	return &Server{
		fileUploadService: service.NewFileUploadService(),
	}
}
