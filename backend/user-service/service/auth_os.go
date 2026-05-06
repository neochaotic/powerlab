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
//	Linux  → PAM via CGO + libpam (see auth_os_pam_linux.go). Linkage
//	         happens at build time; CGO_ENABLED=0 builds get the stub
//	         in auth_os_pam_stub.go and will fall back to the bcrypt
//	         SetupWizard at runtime.
//	other  → not supported.
//
// Why PAM via CGO and not a shell-out: every shell-out alternative we
// evaluated had a fatal flaw. `unix_chkpwd` silently returns exit 0
// even on wrong passwords when called from outside `pam_unix` — a
// security feature for pam_unix's own use, but a password-bypass
// footgun for direct callers (verified empirically on Ubuntu 22.04).
// `mkpasswd` does not support yescrypt (the default on Ubuntu 22.04+
// and Debian 12+). `su -c true` cannot take a password on stdin
// without an external `expect`-style wrapper. CGO + libpam is the
// only path that delegates the entire crypto choice (yescrypt, SHA-512,
// bcrypt, …) to the host's libxcrypt at runtime, which is what every
// serious Linux panel (Cockpit, Webmin, Wazuh) does.
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
		// Delegate to authenticateLinuxPAM (CGO + libpam in production
		// builds, no-op stub returning a typed error for builds compiled
		// without CGO). The contract is identical to dscl on macOS:
		// (true, nil) on accept, (false, nil) on a clean rejection,
		// (false, err) when PAM cannot run — the login handler upstream
		// routes the err case to the bcrypt SetupWizard fallback.
		return authenticateLinuxPAM(username, password)
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
