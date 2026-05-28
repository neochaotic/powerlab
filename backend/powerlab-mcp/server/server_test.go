package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/neochaotic/powerlab/backend/powerlab-mcp/config"
)

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	s, err := New(config.Default(), BuildInfo{Version: "0.7.4", Commit: "abc123", Date: "2026-05-27"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s.Handler()
}

// /healthz is the liveness contract the systemd unit and the install
// smoke test poll. It must answer 200 with no auth — a health probe that
// needs a token is not a health probe.
func TestHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestHandler(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("/healthz status = %d; want 200", rec.Code)
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "ok") {
		t.Fatalf("/healthz body = %q; want it to signal ok", rec.Body.String())
	}
}

// /version surfaces the ldflags-injected build info. The updater and any
// "what's running" check read this, so the bytes we inject at link time
// must actually come back out — not a hardcoded or empty string.
func TestVersionReflectsInjectedBuildInfo(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestHandler(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/version", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("/version status = %d; want 200", rec.Code)
	}
	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("/version body is not JSON object: %v (body=%q)", err, rec.Body.String())
	}
	if got["version"] != "0.7.4" {
		t.Fatalf("version = %q; want the injected %q", got["version"], "0.7.4")
	}
	if got["commit"] != "abc123" {
		t.Fatalf("commit = %q; want the injected %q", got["commit"], "abc123")
	}
}

// The whole point of this binary is to speak MCP. The MCP transport must
// be actually mounted, not merely declared — a POST to the endpoint must
// reach the MCP handler (any non-404 proves the mount; a 404 means the
// route was never wired). Guards against shipping a server that builds
// but exposes no MCP surface at all.
func TestMCPEndpointIsMounted(t *testing.T) {
	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	req := httptest.NewRequest(http.MethodPost, MCPEndpointPath, body)
	req.Header.Set("Content-Type", "application/json")
	// From loopback so the read-tier gate skips auth and the request
	// actually reaches the MCP handler — otherwise we'd only be proving
	// the gate 401s, not that the transport is mounted behind it.
	req.RemoteAddr = "127.0.0.1:54321"
	newTestHandler(t).ServeHTTP(rec, req)

	if rec.Code == http.StatusNotFound {
		t.Fatalf("POST %s returned 404 — the MCP transport is not mounted", MCPEndpointPath)
	}
	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("POST %s from loopback returned 401 — the read-tier gate must skip loopback", MCPEndpointPath)
	}
}
