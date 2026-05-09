package v2

import (
	"github.com/neochaotic/powerlab/backend/local-storage/codegen"
)

type LocalStorage struct{}

func NewLocalStorage() codegen.ServerInterface {
	return &LocalStorage{}
}
