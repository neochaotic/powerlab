package security

import (
	"os"
	"testing"
)

// TestHTTPSEnabled_StrictTrueOnly locks in the v0.5.2 → v0.6
// strategic decision: HTTPS is opt-in via POWERLAB_HTTPS_ENABLED
// AND the env var must be EXACTLY "true". Any laxness here would
// risk accidental re-enable on hosts where someone ran
// `POWERLAB_HTTPS_ENABLED=1 ...` or similar.
func TestHTTPSEnabled_StrictTrueOnly(t *testing.T) {
	cases := []struct {
		val  string
		want bool
	}{
		// Only literal "true" enables HTTPS.
		{"true", true},
		// Everything else gates HTTPS.
		{"1", false},
		{"yes", false},
		{"TRUE", false},
		{"True", false},
		{"on", false},
		{"enabled", false},
		{"false", false},
		{"0", false},
		{"", false},
		// Whitespace strict — no trimming.
		{" true", false},
		{"true ", false},
	}
	for _, c := range cases {
		t.Run(c.val, func(t *testing.T) {
			t.Setenv(HTTPSGateEnvVar, c.val)
			got := HTTPSEnabled()
			if got != c.want {
				t.Errorf("HTTPSEnabled with %q: want %v, got %v", c.val, c.want, got)
			}
		})
	}
}

// TestHTTPSEnabled_UnsetIsGated covers the default state — when
// install.sh or systemd unit don't set the env var, HTTPS must be
// off. (t.Setenv guarantees the env is restored after the test;
// here we explicitly Unsetenv too in case other tests left it set.)
func TestHTTPSEnabled_UnsetIsGated(t *testing.T) {
	_ = os.Unsetenv(HTTPSGateEnvVar)
	if HTTPSEnabled() {
		t.Errorf("HTTPSEnabled with env unset: want false (gated default), got true")
	}
}
