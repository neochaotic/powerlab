package config

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestAppManagementConfigFilePath_NoLegacyCasaOSPath — Sprint 3 Phase 3
// rebrand. Primary regression check: the post-init value must not
// reference the legacy `/casaos/` path. We do NOT assert the full
// path matches `filepath.Join(constants.DefaultConfigPath, ...)`
// because this package's init() has a fallback that rewrites the
// var to bare basename `app-management.conf` when DefaultConfigPath
// does not exist on disk. That fallback fires on CI (where the dev
// sandbox `<repo>/backend/conf` directory is constructed by
// constants.maybeApplyDevSandbox but never created) and is
// intentional behavior — keeps the binary usable when run from a
// directory containing a hand-placed `app-management.conf`.
func TestAppManagementConfigFilePath_NoLegacyCasaOSPath(t *testing.T) {
	if strings.Contains(AppManagementConfigFilePath, "/casaos/") {
		t.Errorf("AppManagementConfigFilePath = %q must not reference legacy /casaos/ path",
			AppManagementConfigFilePath)
	}
	if filepath.Base(AppManagementConfigFilePath) != "app-management.conf" {
		t.Errorf("AppManagementConfigFilePath = %q basename must be app-management.conf",
			AppManagementConfigFilePath)
	}
}

func TestAppManagementGlobalEnvFilePath_NoLegacyCasaOSPath(t *testing.T) {
	if strings.Contains(AppManagementGlobalEnvFilePath, "/casaos/") {
		t.Errorf("AppManagementGlobalEnvFilePath = %q must not reference legacy /casaos/ path",
			AppManagementGlobalEnvFilePath)
	}
	if filepath.Base(AppManagementGlobalEnvFilePath) != "env" {
		t.Errorf("AppManagementGlobalEnvFilePath = %q basename must be env",
			AppManagementGlobalEnvFilePath)
	}
}
