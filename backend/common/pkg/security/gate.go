package security

import "os"

// HTTPSGateEnvVar is the environment variable that gates the entire
// HTTPS feature. When set to "true" (case-sensitive), HTTPS is
// active: cert manager initializes, HTTPS listener binds at 8443,
// HSTS header is emitted, HTTP→HTTPS redirect is active, cert
// download endpoints serve cert files. When unset or set to anything
// else, HTTPS is GATED and the gateway runs HTTP-only.
//
// Why an env var rather than gateway.ini: this is a per-release
// safety switch, not a user-facing config. v0.5.2 ships with HTTPS
// gated by default after the upgrade incident (#129, #130) revealed
// 4 boot-time gateway bugs and a security concern about silent CA
// trust. v0.6 ships with the env var implicitly true in install.sh,
// after #118 (trust-dance redo) and integration tests land.
const HTTPSGateEnvVar = "POWERLAB_HTTPS_ENABLED"

// HTTPSEnabled returns true ONLY if HTTPSGateEnvVar is exactly the
// string "true". Any other value (including "1", "yes", "TRUE", or
// unset) means HTTPS is gated. This deliberately strict comparison
// makes accidental enables impossible — explicit opt-in only.
//
// See issue #130 for the v0.5.2 strategic decision and the v0.6
// re-enable plan.
func HTTPSEnabled() bool {
	return os.Getenv(HTTPSGateEnvVar) == "true"
}
