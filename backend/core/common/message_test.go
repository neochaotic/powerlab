package common

import (
	"strings"
	"testing"
)

// TestEventTypes_UsePowerLabPrefix — Sprint 3 Phase 3 rebrand.
//
// PowerLab event topics use the `powerlab:*` prefix to make routing
// self-describing in logs + traces. The legacy `casaos:*` prefix
// existed for parity with the upstream CasaOS UI but no PowerLab
// component subscribes to it (verified by grep across UI + all 6
// services), so renaming is safe.
//
// Exception: `casaos:file:recover` is intentionally left for now
// because core's parallel cloud-drive infrastructure still references
// it. That topic dies together with core's cloud-drive removal
// (planned follow-up PR mirroring #139's local-storage cleanup).
func TestEventTypes_UsePowerLabPrefix(t *testing.T) {
	for _, et := range EventTypes {
		if et.Name == "casaos:file:recover" {
			// Intentional CasaOS-prefix holdover, see godoc above.
			continue
		}
		if !strings.HasPrefix(et.Name, "powerlab:") {
			t.Errorf("event type %q must start with `powerlab:` prefix", et.Name)
		}
		if strings.HasPrefix(et.Name, "casaos:") {
			t.Errorf("event type %q still uses legacy `casaos:` prefix", et.Name)
		}
	}
}
