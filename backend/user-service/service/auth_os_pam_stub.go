//go:build !linux || (linux && !cgo)

package service

import "errors"

// authenticateLinuxPAM stub for platforms that do not have CGO + libpam
// available at compile time. This covers:
//   - macOS development cross-builds (no PAM headers in the toolchain)
//   - Linux builds with CGO_ENABLED=0 (some minimal CI matrices use
//     this for fast static binaries)
//   - Other UNIX-likes the project does not target
//
// The stub returns a typed error so the login handler treats it the
// same as "PAM unavailable on this host" and routes the user to the
// bcrypt SetupWizard fallback. This means a no-CGO build still works
// for sign-in, just without the OS-credentials path — the same UX
// PowerLab v0.1.x shipped with.
func authenticateLinuxPAM(username, password string) (bool, error) {
	return false, errors.New(
		"PAM was not built into this binary (CGO disabled at compile time). " +
			"Use the SetupWizard bcrypt password, or recompile with CGO_ENABLED=1 + libpam0g-dev installed",
	)
}
