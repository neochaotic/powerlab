package v1

import (
	"strings"

	"github.com/neochaotic/powerlab/backend/common/utils/common_err"
)

// MinPasswordLen is the floor enforced server-side. Kept as an
// exported constant so:
//   1. The SetupWizard's helper text can be generated from the same
//      number via a server-config endpoint (#306 follow-up).
//   2. A regression test pins the value — issue #306's headline bug
//      was a mismatch between this floor (6) and the UI guard (5);
//      every change to this value MUST be coordinated with
//      `ui/src/lib/components/auth/SetupWizard.svelte` and the
//      `error.passTooShort` i18n key.
const MinPasswordLen = 8

// ValidatePassword returns 0 when pwd is acceptable, or a
// common_err code identifying the specific failure:
//
//   common_err.INVALID_PARAMS     — empty / whitespace-only
//   common_err.PWD_IS_TOO_SIMPLE  — shorter than MinPasswordLen
//
// Length is measured in **bytes**, matching what bcrypt sees and
// what the original implementation enforced. Users typing a 7-char
// password with a multi-byte character (e.g. an emoji) may see
// inconsistent UX vs. visual character count — that's a Unicode
// edge case the UI helper text already calls out implicitly by
// stating "8 characters" (UTF-8 conservative).
func ValidatePassword(pwd string) int {
	if strings.TrimSpace(pwd) == "" {
		return common_err.INVALID_PARAMS
	}
	if len(pwd) < MinPasswordLen {
		return common_err.PWD_IS_TOO_SIMPLE
	}
	return 0
}
