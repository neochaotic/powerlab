package config

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/IceWhaleTech/CasaOS-Common/utils/constants"
)

// TestMessageBusConfigFilePath_DerivedFromConstants — Sprint 3 Phase 3
// rebrand. Production install writes config to /etc/powerlab/message-bus.conf
// (Linux) or /opt/powerlab/etc/message-bus.conf (darwin). The Go-side
// default that the binary reads when started without `-c` MUST match
// what install.sh shipped, otherwise the binary creates an empty file
// at the wrong path and silently runs with all-default in-memory config.
//
// This test caught the legacy CasaOS-era const which hardcoded
// /etc/casaos/message-bus.conf — a path that no longer exists after
// the install.sh rewrite to /etc/powerlab/.
func TestMessageBusConfigFilePath_DerivedFromConstants(t *testing.T) {
	want := filepath.Join(constants.DefaultConfigPath, "message-bus.conf")
	if MessageBusConfigFilePath != want {
		t.Errorf("MessageBusConfigFilePath = %q, want %q (derived from constants.DefaultConfigPath)",
			MessageBusConfigFilePath, want)
	}
	if strings.Contains(MessageBusConfigFilePath, "/casaos/") {
		t.Errorf("MessageBusConfigFilePath = %q must not reference legacy /casaos/ path",
			MessageBusConfigFilePath)
	}
}
