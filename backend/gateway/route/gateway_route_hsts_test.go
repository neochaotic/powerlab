package route

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/neochaotic/powerlab/backend/common/pkg/security"
)

// TestWrapHSTS_HTTPSArmedSetsHeader — request comes in over TLS,
// gate is armed → next runs AND the HSTS header is set.
func TestWrapHSTS_HTTPSArmedSetsHeader(t *testing.T) {
	cm := newTestCertManager(t)
	if err := cm.ArmHSTS(); err != nil {
		t.Fatalf("ArmHSTS: %v", err)
	}
	g := &GatewayRoute{cm: cm}

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	req.TLS = &tls.ConnectionState{}

	g.WrapHSTS(next, "8443").ServeHTTP(rec, req)

	if !called {
		t.Fatal("next handler was not invoked on HTTPS request")
	}
	hsts := rec.Header().Get("Strict-Transport-Security")
	if hsts == "" {
		t.Fatal("HSTS header missing on HTTPS+armed request")
	}
	if !strings.Contains(hsts, "max-age=") {
		t.Errorf("HSTS header missing max-age: %q", hsts)
	}
}

// TestWrapHSTS_HTTPSNotArmedNoHeader — request comes in over TLS but
// the gate has not been armed yet → next runs, NO HSTS header.
// Without this guarantee, an HTTPS visit before trust-confirmed
// would lock the user out (browser caches HSTS forever).
func TestWrapHSTS_HTTPSNotArmedNoHeader(t *testing.T) {
	cm := newTestCertManager(t)
	// Do NOT arm.
	g := &GatewayRoute{cm: cm}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	req.TLS = &tls.ConnectionState{}

	g.WrapHSTS(next, "8443").ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected next to run with 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("HSTS header set pre-arm — would lock user out: %q", got)
	}
}

// TestWrapHSTS_HTTPNotArmedPassthrough — plain HTTP, gate not armed,
// passthrough to next. The HTTP listener has to keep serving so the
// user can complete the trust dance.
func TestWrapHSTS_HTTPNotArmedPassthrough(t *testing.T) {
	cm := newTestCertManager(t)
	g := &GatewayRoute{cm: cm}

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	// req.TLS == nil — plain HTTP

	g.WrapHSTS(next, "8443").ServeHTTP(rec, req)

	if !called {
		t.Fatal("HTTP+not-armed should pass through to next; next was not called")
	}
	if rec.Code == http.StatusMovedPermanently {
		t.Fatalf("HTTP+not-armed redirected — would block trust dance")
	}
}

// TestWrapHSTS_HTTPArmedRedirects — plain HTTP, gate IS armed →
// permanent redirect to https://<host>:<httpsPort> with the original
// path/query preserved.
func TestWrapHSTS_HTTPArmedRedirects(t *testing.T) {
	cm := newTestCertManager(t)
	if err := cm.ArmHSTS(); err != nil {
		t.Fatalf("ArmHSTS: %v", err)
	}
	g := &GatewayRoute{cm: cm}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next should NOT be invoked on HTTP+armed")
	})

	cases := []struct {
		name      string
		incoming  string
		hostHdr   string
		httpsPort string
		want      string
	}{
		{"path with query, custom port", "/v1/foo?bar=baz", "powerlab.local:8765", "8443", "https://powerlab.local:8443/v1/foo?bar=baz"},
		{"strips incoming port", "/", "192.168.1.42:8765", "8443", "https://192.168.1.42:8443/"},
		{"port 443 → omitted", "/", "powerlab.local:8765", "443", "https://powerlab.local/"},
		{"empty port → omitted", "/", "powerlab.local", "", "https://powerlab.local/"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, c.incoming, nil)
			req.Host = c.hostHdr

			g.WrapHSTS(next, c.httpsPort).ServeHTTP(rec, req)

			if rec.Code != http.StatusMovedPermanently {
				t.Fatalf("status: got %d, want 301", rec.Code)
			}
			if loc := rec.Header().Get("Location"); loc != c.want {
				t.Errorf("redirect: got %q, want %q", loc, c.want)
			}
		})
	}
}

// TestWrapHSTS_GateFileToggleObserved — flipping the gate on disk
// changes behavior on the very next request, no caching. This pins
// the file-based gate semantics: the daemon never caches IsHSTSArmed
// in memory across requests.
func TestWrapHSTS_GateFileToggleObserved(t *testing.T) {
	cm := newTestCertManager(t)
	g := &GatewayRoute{cm: cm}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	mkReq := func() *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.TLS = &tls.ConnectionState{}
		return req
	}

	// Pre-arm: no header.
	rec := httptest.NewRecorder()
	g.WrapHSTS(next, "8443").ServeHTTP(rec, mkReq())
	if rec.Header().Get("Strict-Transport-Security") != "" {
		t.Fatal("HSTS set before arm")
	}

	// Arm the gate.
	if err := cm.ArmHSTS(); err != nil {
		t.Fatalf("ArmHSTS: %v", err)
	}

	// Same handler, same wrapper — header now appears.
	rec = httptest.NewRecorder()
	g.WrapHSTS(next, "8443").ServeHTTP(rec, mkReq())
	if rec.Header().Get("Strict-Transport-Security") == "" {
		t.Fatal("HSTS not set after ArmHSTS — gate file toggle was not observed")
	}
}

// _ unused-variable guards for security pkg (we don't import it
// directly but the helper file does)
var _ = security.HSTSGateFile

// TestWrapHSTS_DisarmingEmitsMaxAgeZero — after DisarmHSTS, HTTPS
// requests get `max-age=0` so the browser's cached HSTS pin is
// evicted (RFC 6797 §6.1.1). This is the only mechanism that
// reliably recovers a user whose browser already pinned the host.
func TestWrapHSTS_DisarmingEmitsMaxAgeZero(t *testing.T) {
	cm := newTestCertManager(t)
	if err := cm.ArmHSTS(); err != nil {
		t.Fatalf("ArmHSTS: %v", err)
	}
	// Reset trust → moves to disarming state.
	if err := cm.DisarmHSTS(); err != nil {
		t.Fatalf("DisarmHSTS: %v", err)
	}
	g := &GatewayRoute{cm: cm}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	req.TLS = &tls.ConnectionState{}

	g.WrapHSTS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), "8443").
		ServeHTTP(rec, req)

	hsts := rec.Header().Get("Strict-Transport-Security")
	if hsts != "max-age=0" {
		t.Fatalf("HSTS header during disarming window: got %q, want max-age=0", hsts)
	}
}

// TestWrapHSTS_DisarmingHTTPNoRedirect — during the disarming
// window, HTTP requests pass through (no 301 to HTTPS). Otherwise
// a browser that just had its pin cleared would still be bounced
// to HTTPS where the cert may now be unrecognized.
func TestWrapHSTS_DisarmingHTTPNoRedirect(t *testing.T) {
	cm := newTestCertManager(t)
	_ = cm.ArmHSTS()
	_ = cm.DisarmHSTS()
	g := &GatewayRoute{cm: cm}

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	// req.TLS == nil → plain HTTP

	g.WrapHSTS(next, "8443").ServeHTTP(rec, req)

	if rec.Code == http.StatusMovedPermanently {
		t.Fatal("HTTP+disarming should NOT redirect; would defeat recovery")
	}
	if !called {
		t.Fatal("next handler should run on HTTP+disarming")
	}
}

// TestWrapHSTS_ArmAfterDisarmClearsDisarmingMarker — re-arming via
// a fresh successful trust dance must remove the disarming marker,
// otherwise we'd keep emitting max-age=0 forever after the user
// recovered.
func TestWrapHSTS_ArmAfterDisarmClearsDisarmingMarker(t *testing.T) {
	cm := newTestCertManager(t)
	_ = cm.ArmHSTS()
	_ = cm.DisarmHSTS()
	if !cm.IsHSTSDisarming() {
		t.Fatal("setup: expected disarming state")
	}
	if err := cm.ArmHSTS(); err != nil {
		t.Fatalf("re-arm: %v", err)
	}
	if cm.IsHSTSDisarming() {
		t.Fatal("disarming marker survived ArmHSTS — would emit max-age=0 forever")
	}
	if !cm.IsHSTSArmed() {
		t.Fatal("re-arm did not flip armed state")
	}
}
