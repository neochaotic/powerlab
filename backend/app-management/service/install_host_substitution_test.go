package service

import (
	"strings"
	"testing"
)

// TestSubstituteHostPlaceholders_URLEmbedded locks the Sprint 21 PR 9
// fix for the "Invalid URI" crash class. Apps inherited from Umbrel /
// CasaOS embed ${DEVICE_DOMAIN_NAME} inside URLs (APP_URL, BASE_URL,
// ALLOWED_ORIGINS, NEXTAUTH_URL, ...). Sprint 21 PR 3 sync-catalog
// transform deliberately skipped this case because substituting with
// `*` produces invalid URLs (`http://*:8770` rejected by SvelteKit).
// Install-time substitution with the operator's actual host fixes it
// without breaking the URL syntax.
func TestSubstituteHostPlaceholders_URLEmbedded(t *testing.T) {
	in := []byte(`services:
  app:
    environment:
      APP_URL: http://${DEVICE_DOMAIN_NAME}:8770
      BASE_URL: http://${DEVICE_HOSTNAME}:9000
      NEXTAUTH_URL: http://${APP_DOMAIN}:8233/api/v1/auth
`)
	out := SubstituteHostPlaceholders(in, "192.168.18.86:8765")
	s := string(out)
	if strings.Contains(s, "${DEVICE_DOMAIN_NAME}") {
		t.Errorf("expected ${DEVICE_DOMAIN_NAME} substituted, got:\n%s", s)
	}
	if strings.Contains(s, "${DEVICE_HOSTNAME}") {
		t.Errorf("expected ${DEVICE_HOSTNAME} substituted, got:\n%s", s)
	}
	if strings.Contains(s, "${APP_DOMAIN}") {
		t.Errorf("expected ${APP_DOMAIN} substituted, got:\n%s", s)
	}
	// Host hint includes the gateway port :8765. We strip the port so
	// app-specific ports (:8770, :9000) stay intact in the URL.
	if !strings.Contains(s, "http://192.168.18.86:8770") {
		t.Errorf("expected APP_URL = http://192.168.18.86:8770, got:\n%s", s)
	}
	if !strings.Contains(s, "http://192.168.18.86:9000") {
		t.Errorf("expected BASE_URL with substituted host, got:\n%s", s)
	}
}

func TestSubstituteHostPlaceholders_ListForm(t *testing.T) {
	// `ALLOWED_ORIGINS=http://${DEVICE_DOMAIN_NAME}:port,...` — comma
	// separated origins, each must get the host substituted.
	in := []byte(`services:
  app:
    environment:
      - ALLOWED_ORIGINS=http://${DEVICE_DOMAIN_NAME}:8770,http://${DEVICE_HOSTNAME}:8770
`)
	out := SubstituteHostPlaceholders(in, "powerlab.local")
	s := string(out)
	if strings.Contains(s, "${DEVICE_DOMAIN_NAME}") || strings.Contains(s, "${DEVICE_HOSTNAME}") {
		t.Errorf("expected both placeholders substituted, got:\n%s", s)
	}
	if !strings.Contains(s, "http://powerlab.local:8770,http://powerlab.local:8770") {
		t.Errorf("expected both origins substituted, got:\n%s", s)
	}
}

func TestSubstituteHostPlaceholders_LocalIPs(t *testing.T) {
	// `${APP_FOO_LOCAL_IPS}` is the Umbrel placeholder for "list of
	// LAN IPs this app should accept". For install-time we resolve to
	// a single value (the host hint) — same DEVICE_DOMAIN_NAME path.
	in := []byte(`services:
  app:
    environment:
      - TRUSTED_PROXIES=${APP_NEXTCLOUD_LOCAL_IPS}
`)
	out := SubstituteHostPlaceholders(in, "192.168.18.86")
	s := string(out)
	if strings.Contains(s, "${APP_") {
		t.Errorf("expected APP_*_LOCAL_IPS substituted, got:\n%s", s)
	}
	if !strings.Contains(s, "TRUSTED_PROXIES=192.168.18.86") {
		t.Errorf("expected TRUSTED_PROXIES=192.168.18.86, got:\n%s", s)
	}
}

func TestSubstituteHostPlaceholders_StripPort(t *testing.T) {
	// The Host header arrives as "host:port" (the gateway port). We
	// substitute with just the host portion so the env-var-embedded
	// app-specific port survives.
	in := []byte(`services:
  app:
    environment:
      APP_URL: http://${DEVICE_DOMAIN_NAME}:8770
`)
	for _, hint := range []string{"192.168.18.86:8765", "192.168.18.86:443", "powerlab.local:80", "192.168.18.86"} {
		out := SubstituteHostPlaceholders(in, hint)
		s := string(out)
		if strings.Contains(s, ":8765") || strings.Contains(s, ":443") || strings.Contains(s, ":80\n") {
			t.Errorf("hint=%q: gateway port leaked into substitution: %s", hint, s)
		}
	}
}

func TestSubstituteHostPlaceholders_FallsBackWhenHintEmpty(t *testing.T) {
	// No host hint (e.g., install triggered from a non-HTTP path, or
	// the request had no Host header). Fallback: any non-loopback
	// IPv4 from the host. We can't assert the exact IP in a unit test,
	// but we CAN assert "no placeholder remains" + "no empty host".
	in := []byte(`services:
  app:
    environment:
      APP_URL: http://${DEVICE_DOMAIN_NAME}:8770
`)
	out := SubstituteHostPlaceholders(in, "")
	s := string(out)
	if strings.Contains(s, "${DEVICE_DOMAIN_NAME}") {
		t.Errorf("expected placeholder substituted even with empty hint, got:\n%s", s)
	}
	if strings.Contains(s, "http://:8770") {
		t.Errorf("expected non-empty host even with empty hint, got:\n%s", s)
	}
}

func TestSubstituteHostPlaceholders_NoopWhenNoPlaceholders(t *testing.T) {
	in := []byte(`services:
  app:
    image: nginx:latest
    environment:
      FOO: bar
`)
	out := SubstituteHostPlaceholders(in, "192.168.18.86:8765")
	if string(out) != string(in) {
		t.Errorf("expected pass-through when no placeholders, got change:\nin:  %s\nout: %s", in, out)
	}
}

func TestSubstituteHostPlaceholders_PreservesOtherPlaceholders(t *testing.T) {
	// We MUST NOT touch other Umbrel placeholders that have their own
	// resolution path: ${APP_DATA_DIR} (volumes — sync-time), ${APP_SEED}
	// (secrets — install-time but a different func), ${APP_PASSWORD},
	// ${APP_FOO_PORT}.
	in := []byte(`services:
  app:
    environment:
      APP_URL: http://${DEVICE_DOMAIN_NAME}:8770
      APP_DATA_DIR: ${APP_DATA_DIR}/sub
      SEED: ${APP_SEED}
      PORT: ${APP_PORT}
`)
	out := SubstituteHostPlaceholders(in, "powerlab.local")
	s := string(out)
	if !strings.Contains(s, "${APP_DATA_DIR}") {
		t.Errorf("expected APP_DATA_DIR preserved (volume-substitution territory)")
	}
	if !strings.Contains(s, "${APP_SEED}") {
		t.Errorf("expected APP_SEED preserved (secret-substitution territory)")
	}
	if !strings.Contains(s, "${APP_PORT}") {
		t.Errorf("expected APP_PORT preserved (port-substitution territory)")
	}
}

// Adversarial coverage for stripPort — IPv6 / malformed / port edges.
// Caught a latent bug: bare (unbracketed) IPv6 "::1" was mangled to ":"
// because the trailing "1" looked like a port. Host headers bracket
// IPv6 so it wasn't hit in prod, but the defensive function should not
// corrupt it.
func TestStripPort_AdversarialEdges(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"192.168.1.10:8765", "192.168.1.10"},
		{"powerlab.local:80", "powerlab.local"},
		{"powerlab.local", "powerlab.local"},
		{"[::1]:8765", "::1"},
		{"[fe80::1]:443", "fe80::1"},
		{"[::1", "[::1"},                   // unterminated bracket → unchanged
		{"host:notaport", "host:notaport"}, // non-numeric suffix → unchanged
		{"::1", "::1"},                     // bare IPv6 → must NOT be mangled to ":"
		{"fe80::1234", "fe80::1234"},       // bare IPv6 → unchanged
	}
	for _, c := range cases {
		if got := stripPort(c.in); got != c.want {
			t.Errorf("stripPort(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
