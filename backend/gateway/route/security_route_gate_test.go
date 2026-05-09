package route

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/IceWhaleTech/CasaOS-Common/pkg/security"
)

// These tests pin the HTTPS-gate behavior introduced in v0.5.2 (#130):
// when POWERLAB_HTTPS_ENABLED is unset / not "true", every cert
// download and trust-state mutation endpoint returns 503 + a
// {"code": "https.gated", ...} JSON envelope. The UI consumes this
// to render the gated banner instead of attempting the trust dance.

// gatedRecord runs handler with the env var unset (HTTPS gated) and
// returns the response.
func gatedRecord(t *testing.T, handler http.HandlerFunc, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	// Override TestMain's enable so this specific test sees the gated default.
	previous := os.Getenv(security.HTTPSGateEnvVar)
	_ = os.Unsetenv(security.HTTPSGateEnvVar)
	t.Cleanup(func() {
		if previous != "" {
			_ = os.Setenv(security.HTTPSGateEnvVar, previous)
		}
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(method, target, nil)
	handler(w, r)
	return w
}

// assertGatedResponse confirms the response shape every gated handler
// must produce: 503, application/json, body containing {code: "https.gated"}.
func assertGatedResponse(t *testing.T, w *httptest.ResponseRecorder, name string) {
	t.Helper()
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("%s: status want 503, got %d", name, w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("%s: content-type want application/json, got %q", name, ct)
	}
	body, _ := io.ReadAll(w.Body)
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("%s: response body not JSON: %v (body=%s)", name, err, body)
	}
	if code, _ := parsed["code"].(string); code != "https.gated" {
		t.Errorf("%s: body.code want \"https.gated\", got %q", name, code)
	}
	if msg, _ := parsed["message"].(string); !strings.Contains(msg, "gated") {
		t.Errorf("%s: body.message must mention 'gated', got %q", name, msg)
	}
}

func TestSecurityRoute_CertDownloadGated(t *testing.T) {
	// Construct a SecurityRoute with a non-nil CertManager — the gate
	// must short-circuit BEFORE we attempt to read cert files. If
	// the gate fails to fire, the test would still 503 but for the
	// "ca certificate not available yet" reason, not the gated one
	// — assertGatedResponse pins this distinction via body.code.
	cm := newTestCertManager(t)
	s := NewSecurityRoute(cm)

	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
		method  string
		path    string
	}{
		{"handleCABase", s.handleCABase, http.MethodGet, "/v1/sys/ca-certificate"},
		{"handleCACrt", s.handleCACrt, http.MethodGet, "/v1/sys/ca-certificate.crt"},
		{"handleCAMobileConfig", s.handleCAMobileConfig, http.MethodGet, "/v1/sys/ca-certificate.mobileconfig"},
		{"handleCACer", s.handleCACer, http.MethodGet, "/v1/sys/ca-certificate.cer"},
		{"handleTrustConfirmed", s.handleTrustConfirmed, http.MethodPost, "/v1/sys/trust-confirmed"},
		{"handleRotateCA", s.handleRotateCA, http.MethodPost, "/v1/sys/rotate-ca"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := gatedRecord(t, tc.handler, tc.method, tc.path)
			assertGatedResponse(t, w, tc.name)
		})
	}
}

// TestSecurityRoute_TrustStateNotGated confirms the read-only state
// query is intentionally NOT gated. The UI needs to know HTTPS is
// off to render the banner; gating /trust-state would hide that.
func TestSecurityRoute_TrustStateNotGated(t *testing.T) {
	cm := newTestCertManager(t)
	s := NewSecurityRoute(cm)
	w := gatedRecord(t, s.handleTrustState, http.MethodGet, "/v1/sys/trust-state")
	if w.Code == http.StatusServiceUnavailable {
		body, _ := io.ReadAll(w.Body)
		var parsed map[string]any
		if json.Unmarshal(body, &parsed) == nil {
			if code, _ := parsed["code"].(string); code == "https.gated" {
				t.Errorf("trust-state must NOT be gated — UI needs to read state to render the banner. Got 503 https.gated.")
			}
		}
	}
}
