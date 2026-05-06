package config

import (
	"path/filepath"

	"github.com/IceWhaleTech/CasaOS-Common/utils/constants"
)

func GetUserServiceConfigFilePath() string {
	return filepath.Join(constants.DefaultConfigPath, "user-service.conf")
}
