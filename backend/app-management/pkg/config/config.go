package config

import (
	"os"
	"path/filepath"
	"github.com/IceWhaleTech/CasaOS-Common/utils/constants"
)

var (
	AppManagementConfigFilePath    = "/etc/casaos/app-management.conf"
	AppManagementGlobalEnvFilePath = "/etc/casaos/env"
	RemoveRuntimeIfNoNvidiaGPUFlag = false
)

func init() {
	configPath := constants.DefaultConfigPath
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configPath = "."
	}
	AppManagementConfigFilePath = filepath.Join(configPath, "app-management.conf")
	AppManagementGlobalEnvFilePath = filepath.Join(configPath, "env")
}
