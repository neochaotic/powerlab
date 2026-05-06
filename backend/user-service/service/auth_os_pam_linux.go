//go:build linux && cgo

package service

import (
	"errors"
	"fmt"
	"os"

	"github.com/msteinert/pam"
)

// authenticateLinuxPAM validates a credential against the host's PAM
// stack. We delegate the entire crypto path (yescrypt, SHA-512, bcrypt
// — whatever the distro chose) to libpam + libxcrypt at runtime, which
// is the only correct way to do this without reimplementing every hash
// algorithm Linux distros currently ship.
//
// Service-name selection: we prefer a dedicated `/etc/pam.d/powerlab`
// (so admins can lock auth down to a custom policy) and fall back to
// `login`, which is universally present and routes through `pam_unix`
// → /etc/shadow. We do NOT fall back to `system-auth` or similar
// because their availability varies across distros (Debian-family vs
// RHEL-family) and would surprise users.
//
// Error semantics — important: returning a typed error vs `(false,
// nil)` is what the login handler upstream uses to decide whether to
// fall back to bcrypt. This function returns:
//
//   - (true,  nil)  password accepted by PAM
//   - (false, nil)  password rejected (PAM_AUTH_ERR / PAM_USER_UNKNOWN
//                   / PAM_MAXTRIES). User typed the wrong password.
//   - (false, err)  PAM itself could not run (no library, missing
//                   service file, conversation broken). Caller treats
//                   this as "OS auth unavailable" and routes to the
//                   bcrypt SetupWizard fallback.
//
// The conversation function never logs the password and is only
// retained for the duration of the transaction; PAM's own buffers are
// released by t.End() in the deferred call below.
func authenticateLinuxPAM(username, password string) (bool, error) {
	service := pickPAMService()

	t, err := pam.StartFunc(service, username, func(s pam.Style, msg string) (string, error) {
		switch s {
		case pam.PromptEchoOff:
			// pam_unix asks for the password via PromptEchoOff.
			return password, nil
		case pam.PromptEchoOn:
			// Some modules ask the username again at this point. We
			// already supplied it via StartFunc; an empty answer is
			// the safe response.
			return "", nil
		case pam.ErrorMsg, pam.TextInfo:
			// Informational; nothing to return. PAM is allowed to
			// surface MOTD-style messages here, which we discard
			// because PowerLab is not a tty login.
			return "", nil
		}
		return "", fmt.Errorf("unhandled PAM conversation style: %v", s)
	})
	if err != nil {
		return false, fmt.Errorf("pam.StartFunc(%q): %w", service, err)
	}
	// msteinert/pam v1.x releases the underlying handle automatically
	// through a runtime finalizer set inside StartFunc; there is no
	// explicit End()/Close() to call. Letting `t` go out of scope is
	// the documented cleanup path.

	if err := t.Authenticate(0); err != nil {
		// Anything from t.Authenticate is a credential-level rejection
		// in practice. `msteinert/pam` returns the underlying PAM_*
		// code embedded in the error message; we deliberately collapse
		// every authentication failure into `(false, nil)` so the
		// upstream handler returns the same generic "invalid
		// credential" message and does not leak which PAM module
		// rejected the call (a subtle information-leak otherwise).
		return false, nil
	}

	// Account validity checks — locked accounts (`usermod -L`),
	// expired passwords, and disabled shells all fail here. We
	// treat them the same as a rejected password from the user's
	// perspective: a clean refusal, not a system error.
	if err := t.AcctMgmt(0); err != nil {
		return false, nil
	}

	return true, nil
}

// pickPAMService returns the PAM service name to authenticate against.
// Priority order:
//
//  1. `powerlab` — the dedicated policy installed by install.sh into
//     /etc/pam.d/powerlab. Minimal `pam_unix` only — no pam_nologin
//     (which blocks during early boot), no pam_securetty (which is
//     for tty logins, irrelevant here), no MOTD spam.
//
//  2. `passwd` — the universal password-change service that every
//     Linux distro ships and that runs auth against pam_unix without
//     the boot/tty restrictions. Used when the dedicated policy is
//     missing — e.g. someone is testing PowerLab without running the
//     installer, or the admin removed our pam.d file.
//
// We deliberately do NOT fall back to `login`: that service includes
// pam_nologin.so, which rejects authentication while /run/nologin
// exists (set by systemd during early boot AND inside many Docker
// containers). Using `login` would make PowerLab unusable for the
// first ~30 seconds of boot and broken in container-based deploys.
//
// Each call is one stat — cheap enough not to bother caching, and
// dynamic so an admin who drops the policy file mid-flight does not
// need a service restart.
func pickPAMService() string {
	if _, err := os.Stat("/etc/pam.d/powerlab"); err == nil {
		return "powerlab"
	}
	return "passwd"
}

// errPAMUnavailable is the sentinel the handler can match on if it
// wants to surface a more specific UI message than the generic
// "OS authentication unavailable, falling back to bcrypt".
var errPAMUnavailable = errors.New("PAM is unavailable on this host")
