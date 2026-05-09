package common

import (
	"strings"
	"testing"
)

// TestEventTypes_UsePowerLabPrefix — Sprint 3 Phase 3 rebrand.
//
// PowerLab event topics use the `powerlab:*` prefix to make routing
// self-describing in logs + traces. The legacy `casaos:file:recover`
// holdover was retired together with core's cloud-drive removal
// (separate PR after #141), so the test now asserts unconditional
// `powerlab:` prefix on every registered topic.
func TestEventTypes_UsePowerLabPrefix(t *testing.T) {
	for _, et := range EventTypes {
		if !strings.HasPrefix(et.Name, "powerlab:") {
			t.Errorf("event type %q must start with `powerlab:` prefix", et.Name)
		}
		if strings.HasPrefix(et.Name, "casaos:") {
			t.Errorf("event type %q still uses legacy `casaos:` prefix", et.Name)
		}
	}
}
