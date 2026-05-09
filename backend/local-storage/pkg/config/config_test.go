package config

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/IceWhaleTech/CasaOS-Common/utils/constants"
)

// TestLocalStorageConfigFilePath_DerivedFromConstants — Sprint 3 Phase 3
// rebrand. See message-bus equivalent for full rationale; same regression
// risk applies here (legacy /etc/casaos/local-storage.conf path post the
// install.sh /etc/powerlab/ rewrite).
func TestLocalStorageConfigFilePath_DerivedFromConstants(t *testing.T) {
	want := filepath.Join(constants.DefaultConfigPath, "local-storage.conf")
	if LocalStorageConfigFilePath != want {
		t.Errorf("LocalStorageConfigFilePath = %q, want %q (derived from constants.DefaultConfigPath)",
			LocalStorageConfigFilePath, want)
	}
	if strings.Contains(LocalStorageConfigFilePath, "/casaos/") {
		t.Errorf("LocalStorageConfigFilePath = %q must not reference legacy /casaos/ path",
			LocalStorageConfigFilePath)
	}
}
