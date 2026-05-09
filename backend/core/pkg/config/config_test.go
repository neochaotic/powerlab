package config

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/neochaotic/powerlab/backend/common/utils/constants"
)

// TestCoreConfigFilePath_DerivedFromConstants — Sprint 3 Phase 3 rebrand.
//
// Production bug this test guards: install.sh used to ship
// `casaos.conf.sample` into /etc/powerlab/casaos.conf, but systemd starts
// the core service with `-c /etc/powerlab/core.conf`. The two paths
// disagreed → systemd's `-c` won, the binary opened a non-existent file,
// and config.go silently created an empty core.conf, dropping every
// shipped default. The rebrand renames the var to CoreConfigFilePath
// (file basename "core.conf") so the in-binary default and the systemd
// `-c` flag agree.
func TestCoreConfigFilePath_DerivedFromConstants(t *testing.T) {
	want := filepath.Join(constants.DefaultConfigPath, "core.conf")
	if CoreConfigFilePath != want {
		t.Errorf("CoreConfigFilePath = %q, want %q (must match systemd -c flag /etc/powerlab/core.conf)",
			CoreConfigFilePath, want)
	}
	if strings.Contains(CoreConfigFilePath, "casaos") {
		t.Errorf("CoreConfigFilePath = %q must not reference legacy CasaOS path",
			CoreConfigFilePath)
	}
}
