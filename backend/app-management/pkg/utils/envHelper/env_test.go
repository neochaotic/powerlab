package envHelper

import (
	"strings"
	"testing"
)

// Closes #243 — `$DefaultPassword` substituted into every newly
// installed Compose app inherited the literal "casaos" from the
// CasaOS fork. The string is also predictable from the upstream
// project name. Replace with "powerlab".
func TestReplaceDefaultENV_PasswordIsPowerLab(t *testing.T) {
	got := ReplaceDefaultENV("$DefaultPassword", "")
	if got != "powerlab" {
		t.Fatalf("ReplaceDefaultENV($DefaultPassword) = %q, want \"powerlab\"", got)
	}
}

// Belt-and-suspenders: the high-level helper that compose YAMLs
// hit must also produce no occurrence of the legacy literal.
func TestReplaceStringDefaultENV_NoCasaOSLiteral(t *testing.T) {
	out := ReplaceStringDefaultENV("password=$DefaultPassword user=$DefaultUserName")
	if strings.Contains(strings.ToLower(out), "casaos") {
		t.Fatalf("ReplaceStringDefaultENV result still contains \"casaos\": %q", out)
	}
}
