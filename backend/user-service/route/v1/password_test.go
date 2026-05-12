package v1

import (
	"strings"
	"testing"

	"github.com/neochaotic/powerlab/backend/common/utils/common_err"
)

// TestMinPasswordLen_RegressionLock pins the floor at 8.
//
// Issue #306: production user hit "backend error" on onboarding
// because the UI guard rejected `<5` and the backend rejected
// `<6` — typing 5 chars passed the UI, failed the backend, and
// the generic `error.setupFailed` message replaced what should
// have been "password too short".
//
// Both surfaces (this Go file + the SvelteKit SetupWizard) now
// agree on 8. Tightening was the intentional v0.7 bump; if you
// loosen this without updating the UI guard + the
// `error.passTooShort` i18n key in en/pt-BR/es, you reintroduce
// the same class of bug.
func TestMinPasswordLen_RegressionLock(t *testing.T) {
	if MinPasswordLen != 8 {
		t.Errorf("MinPasswordLen = %d, want 8 — issue #306 regression. If you lower the floor, also update SetupWizard.svelte (password.length check) AND ui/src/lib/i18n/locales/{en,pt-BR,es}.json error.passTooShort string.",
			MinPasswordLen)
	}
}

// TestValidatePassword_RejectsEmpty covers the explicit-error
// case for empty / whitespace input. Original code used
// INVALID_PARAMS (a different code from PWD_IS_TOO_SIMPLE), which
// matters because the UI maps the codes to different messages.
func TestValidatePassword_RejectsEmpty(t *testing.T) {
	cases := []string{"", " ", "\t", "\n", "    "}
	for _, pwd := range cases {
		t.Run("empty="+pwd, func(t *testing.T) {
			got := ValidatePassword(pwd)
			if got != common_err.INVALID_PARAMS {
				t.Errorf("ValidatePassword(%q) = %d, want INVALID_PARAMS (%d)", pwd, got, common_err.INVALID_PARAMS)
			}
		})
	}
}

// TestValidatePassword_RejectsShort — the original #306 trigger:
// 5-, 6-, 7-char passwords pass the OLD UI guard but fail the
// backend. After this fix, they fail BOTH (regression-locked).
func TestValidatePassword_RejectsShort(t *testing.T) {
	cases := []string{
		strings.Repeat("a", 1),
		strings.Repeat("a", 5), // the original UI's accepted-but-backend-rejected case
		strings.Repeat("a", 6), // the original backend floor — must now also fail
		strings.Repeat("a", 7), // the new boundary — one shy of MinPasswordLen
	}
	for _, pwd := range cases {
		t.Run(pwd, func(t *testing.T) {
			got := ValidatePassword(pwd)
			if got != common_err.PWD_IS_TOO_SIMPLE {
				t.Errorf("ValidatePassword(%q, len=%d) = %d, want PWD_IS_TOO_SIMPLE (%d)",
					pwd, len(pwd), got, common_err.PWD_IS_TOO_SIMPLE)
			}
		})
	}
}

// TestValidatePassword_Accepts the floor + above.
func TestValidatePassword_Accepts(t *testing.T) {
	cases := []string{
		strings.Repeat("a", 8),     // exact floor
		strings.Repeat("a", 12),    // typical
		"correct horse battery",   // very common passphrase
		"P@ssw0rd!",                // mixed
	}
	for _, pwd := range cases {
		t.Run(pwd, func(t *testing.T) {
			got := ValidatePassword(pwd)
			if got != 0 {
				t.Errorf("ValidatePassword(%q) = %d, want 0 (accepted)", pwd, got)
			}
		})
	}
}
