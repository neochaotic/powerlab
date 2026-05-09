package v2

import codegen "github.com/neochaotic/powerlab/backend/user-service/codegen/user_service"

type UserService struct{}

func NewUserService() codegen.ServerInterface {
	return &UserService{}
}
