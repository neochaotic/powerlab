package config

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/IceWhaleTech/CasaOS-Common/utils/constants"
)

// TestAppManagementConfigFilePath_DerivedFromConstants — Sprint 3 Phase 3
// rebrand. The init() in this package already rewrites the var to
// `filepath.Join(constants.DefaultConfigPath, "app-management.conf")`,
// but the literal initializer still references /etc/casaos which is
// confusing and risks divergence if the init logic is ever simplified.
// This test pins the expected post-init value.
func TestAppManagementConfigFilePath_DerivedFromConstants(t *testing.T) {
	want := filepath.Join(constants.DefaultConfigPath, "app-management.conf")
	if AppManagementConfigFilePath != want {
		t.Errorf("AppManagementConfigFilePath = %q, want %q (derived from constants.DefaultConfigPath)",
			AppManagementConfigFilePath, want)
	}
	if strings.Contains(AppManagementConfigFilePath, "/casaos/") {
		t.Errorf("AppManagementConfigFilePath = %q must not reference legacy /casaos/ path",
			AppManagementConfigFilePath)
	}
}

func TestAppManagementGlobalEnvFilePath_DerivedFromConstants(t *testing.T) {
	want := filepath.Join(constants.DefaultConfigPath, "env")
	if AppManagementGlobalEnvFilePath != want {
		t.Errorf("AppManagementGlobalEnvFilePath = %q, want %q",
			AppManagementGlobalEnvFilePath, want)
	}
	if strings.Contains(AppManagementGlobalEnvFilePath, "/casaos/") {
		t.Errorf("AppManagementGlobalEnvFilePath = %q must not reference legacy /casaos/ path",
			AppManagementGlobalEnvFilePath)
	}
}
