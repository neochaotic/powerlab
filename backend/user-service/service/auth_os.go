package service

import (
	"errors"
	"os/exec"
	"os/user"
	"runtime"
	"strings"
)

// OSService validates a username/password pair against the host operating
// system's account database. It is the *primary* authentication path —
// users sign in with their machine credentials, not a separate "panel
// account".
//
// Per-platform support:
//
//	macOS  → `dscl . -authonly` against the local Directory Service.
//	         Working today.
//	Linux  → not implemented in this build. PAM via CGO is the planned
//	         path for v0.2 (see SUPPORT.md). Until then this returns
//	         an error so the login handler upstream can route the user
//	         to the bcrypt fallback (SetupWizard).
//	other  → not supported.
//
// Why no shell-out today: the obvious helpers all have a fatal flaw.
// `unix_chkpwd` (the PAM helper) silently returns exit 0 even on wrong
// passwords when called from outside `pam_unix` — a security feature for
// `pam_unix`'s own use, but a footgun for direct callers (verified
// empirically on Ubuntu 22.04). `mkpasswd` does not support yescrypt
// (the default on Ubuntu 22.04+ and Debian 12+). `su -c true` cannot
// take a password on stdin without an external `expect`-style wrapper.
// The clean, robust answer is PAM via CGO; we will add that in a
// follow-up release rather than ship a half-secure shortcut here.
//
// IMPORTANT: this service MUST run as root for any future Linux path
// that reads /etc/shadow. The packaged systemd units run user-service
// as root by design.
type OSService struct{}

// Authenticate returns:
//
//	(true,  nil) — credentials accepted by the OS
//	(false, nil) — credentials rejected (clear "wrong password" UX)
//	(false, err) — auth path itself could not run; caller MUST treat
//	               this as "OS auth unavailable" and decide whether to
//	               fall back to the bcrypt path. NEVER treat err == nil
//	               + result == false the same as err != nil — only the
//	               nil-error rejection means "user typed the wrong
//	               password", which is the only case to surface as 401.
func (s *OSService) Authenticate(username, password string) (bool, error) {
	if username == "" || strings.ContainsAny(username, "\x00\n:") {
		return false, errors.New("invalid username")
	}
	if password == "" || strings.Contains(password, "\x00") {
		// Empty / NUL-bearing passwords are never valid; refuse early
		// to avoid passing them to a subprocess.
		return false, errors.New("invalid password")
	}

	if _, err := user.Lookup(username); err != nil {
		return false, errors.New("user does not exist on this system")
	}

	switch runtime.GOOS {
	case "darwin":
		return s.authenticateMacOS(username, password)
	case "linux":
		// Returning a typed error (not `false, nil`) is critical: the
		// login handler treats `err != nil` as "OS auth unavailable,
		// fall back to bcrypt". Returning `false, nil` would be read
		// as "user typed the wrong password", silently shadowing every
		// successful bcrypt fallback for OS users.
		return false, errors.New(
			"native OS auth on Linux is not yet implemented in this build; " +
				"sign in with the password you set during the SetupWizard, " +
				"or follow the upgrade path in SUPPORT.md to enable PAM",
		)
	default:
		return false, errors.New("unsupported operating system for OS auth")
	}
}

// authenticateMacOS validates a credential against the local Directory
// Service via `dscl . -authonly <user> <password>`. dscl exposes no
// stdin form, so the password is passed positionally. The user-service
// runs as root in production, restricting `ps` visibility to root —
// equivalent to how `sudo` already operates.
func (s *OSService) authenticateMacOS(username, password string) (bool, error) {
	cmd := exec.Command("dscl", ".", "-authonly", username, password)
	if err := cmd.Run(); err != nil {
		// Any non-zero dscl exit on a healthy macOS means rejection.
		// dscl has no separate "system unavailable" path for us to
		// distinguish, so we fold all failures into a clean rejection
		// rather than a typed error. Pre-flight checks above already
		// caught "user does not exist".
		return false, nil
	}
	return true, nil
}

// GetOSUser returns the OS user record (uid, home, shell, …). Used by
// the login handler to populate the DB record on first OS sign-in.
func (s *OSService) GetOSUser(username string) (*user.User, error) {
	return user.Lookup(username)
}
