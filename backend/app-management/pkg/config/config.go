package config

import (
	"os"
	"path/filepath"

	"github.com/IceWhaleTech/CasaOS-Common/utils/constants"
)

// AppManagementConfigFilePath / AppManagementGlobalEnvFilePath are
// platform-resolved at init time via constants.DefaultConfigPath.
// Resolved per-platform:
//
//	Linux  → /etc/powerlab/app-management.conf, /etc/powerlab/env
//	darwin → /opt/powerlab/etc/app-management.conf, /opt/powerlab/etc/env
//	dev    → <repo>/backend/conf/...  (sandbox)
//
// Vars rather than consts because the production-vs-dev flip in init()
// below also adapts to environments where DefaultConfigPath does not
// exist on disk yet (init.go writes the sample on first boot).
var (
	AppManagementConfigFilePath    = filepath.Join(constants.DefaultConfigPath, "app-management.conf")
	AppManagementGlobalEnvFilePath = filepath.Join(constants.DefaultConfigPath, "env")
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
