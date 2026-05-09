package common

import (
	"strings"
	"testing"
)

// TestSERVICENAME_NoLegacyCasaOSValue — Sprint 3 Phase 3 rebrand (#106).
//
// SERVICENAME is sent to message-bus as the SourceID on every published
// event + every event-type registration. It surfaces in logs and trace
// output of every PowerLab service that consumes those events. The
// legacy "CasaOS-UserService" value advertised CasaOS branding from a
// PowerLab process — fixed by this rebrand.
func TestSERVICENAME_NoLegacyCasaOSValue(t *testing.T) {
	if SERVICENAME != "PowerLab-UserService" {
		t.Errorf("SERVICENAME = %q, want %q", SERVICENAME, "PowerLab-UserService")
	}
	if strings.Contains(SERVICENAME, "CasaOS") {
		t.Errorf("SERVICENAME = %q must not advertise legacy CasaOS branding", SERVICENAME)
	}
	if strings.Contains(strings.ToLower(SERVICENAME), "zima") {
		t.Errorf("SERVICENAME = %q must not advertise legacy ZimaOS branding", SERVICENAME)
	}
}
