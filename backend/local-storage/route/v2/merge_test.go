package v2

import (
	"strings"
	"testing"
)

// User-facing error messages must reflect the PowerLab brand. CasaOS
// branding leaking into responses is a regression of Sprint 2 Kill #3.
func TestMessageVolumeNotPowerLabStorage_branded(t *testing.T) {
	if !strings.Contains(MessageVolumeNotPowerLabStorage, "PowerLab") {
		t.Errorf("expected message to mention PowerLab, got: %q", MessageVolumeNotPowerLabStorage)
	}
	if strings.Contains(strings.ToLower(MessageVolumeNotPowerLabStorage), "casaos") {
		t.Errorf("expected message to NOT mention CasaOS, got: %q", MessageVolumeNotPowerLabStorage)
	}
}
