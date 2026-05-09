package v2

import (
	"github.com/neochaotic/powerlab/backend/local-storage/service/v2/wrapper"
	"gorm.io/gorm"
)

type LocalStorageService struct {
	_mountinfo wrapper.MountInfoWrapper
	_db        *gorm.DB
}

func NewLocalStorageService(db *gorm.DB, mountinfo wrapper.MountInfoWrapper) *LocalStorageService {
	return &LocalStorageService{
		_mountinfo: mountinfo,
		_db:        db,
	}
}
