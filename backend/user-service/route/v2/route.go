// Package v2 implements the codegen.ServerInterface for the V2
// user-service API. Each handler method (defined across files in
// this package) maps 1:1 to an operation in
// `api/user-service/openapi.yaml`.
package v2

import codegen "github.com/neochaotic/powerlab/backend/user-service/codegen/user_service"

// UserService is the V2 server-interface implementation. Empty
// struct; per-operation handlers are methods on this type.
type UserService struct{}

// NewUserService constructs a fresh V2 server-interface
// implementation. Wired into route.InitV2Router.
func NewUserService() codegen.ServerInterface {
	return &UserService{}
}
