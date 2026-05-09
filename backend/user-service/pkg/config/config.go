package config

import (
	"path/filepath"

	"github.com/neochaotic/powerlab/backend/common/utils/constants"
)

func GetUserServiceConfigFilePath() string {
	return filepath.Join(constants.DefaultConfigPath, "user-service.conf")
}
