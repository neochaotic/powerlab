package coreproxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ResolveCore reads the .url file core publishes — if it's missing
// the resolver MUST return a core_unavailable error (not a transport
// error) so the resource layer can serve the canonical shape to the
// agent, with a fallback hint pointing at journal + audit.
func TestResolveCore_MissingURLFileSurfacesAsCoreUnavailable(t *testing.T) {
	c := NewClient(t.TempDir(), nil)
	_, err := c.ResolveCore()
	pe, ok := err.(*Error)
	if !ok {
		t.Fatalf("missing url file returned %T; want *Error", err)
	}
	if pe.Code != "core_unavailable" {
		t.Fatalf("Code=%q; want core_unavailable", pe.Code)
	}
	if !strings.Contains(pe.Detail, CoreURLFile) {
		t.Fatalf("Detail=%q; want it to mention %q so the operator can locate the missing file", pe.Detail, CoreURLFile)
	}
}

// The .url file contents are sometimes a bare host:port (the legacy
// CasaOS writer), sometimes an http:// URL. Both must work — the
// resolver normalises to a full URL with scheme.
func TestResolveCore_NormalisesBareHostPort(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, CoreURLFile), []byte("127.0.0.1:9876"), 0o600); err != nil {
		t.Fatalf("write url file: %v", err)
	}
	c := NewClient(dir, nil)
	got, err := c.ResolveCore()
	if err != nil {
		t.Fatalf("ResolveCore: %v", err)
	}
	if got != "http://127.0.0.1:9876" {
		t.Fatalf("ResolveCore = %q; want %q", got, "http://127.0.0.1:9876")
	}
}

// Repeated resolves within the TTL window must hit the cache, not
// re-read the file. This both keeps the hot path cheap AND means we
// can stat-prove the cache by writing the file once and then
// removing it — the second call still succeeds.
func TestResolveCore_CachesWithinTTL(t *testing.T) {
	dir := t.TempDir()
	urlFile := filepath.Join(dir, CoreURLFile)
	if err := os.WriteFile(urlFile, []byte("http://127.0.0.1:1234"), 0o600); err != nil {
		t.Fatalf("write url file: %v", err)
	}
	c := NewClient(dir, nil)

	first, err := c.ResolveCore()
	if err != nil {
		t.Fatalf("first ResolveCore: %v", err)
	}
	// Drop the file — only the cache can satisfy the next call.
	if err := os.Remove(urlFile); err != nil {
		t.Fatalf("remove url file: %v", err)
	}
	second, err := c.ResolveCore()
	if err != nil {
		t.Fatalf("cached ResolveCore returned %v; want cache hit", err)
	}
	if first != second {
		t.Fatalf("cache returned a different URL: first=%q second=%q", first, second)
	}
}

// A successful Get round-trips the body verbatim from core. We stand
// up an httptest server playing core's role, point the resolver at a
// .url file containing the test server URL, and confirm the bytes
// come back unchanged.
func TestGet_RoundTripsBodyFromCore(t *testing.T) {
	want := `{"cpu":{"percent":42.5},"mem":{"used":1024}}`
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/sys/utilization" {
			t.Errorf("core received unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(want))
	}))
	defer core.Close()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, CoreURLFile), []byte(core.URL), 0o600); err != nil {
		t.Fatalf("write url file: %v", err)
	}

	got, err := NewClient(dir, core.Client()).Get(context.Background(), "/v1/sys/utilization", "")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != want {
		t.Fatalf("got body %q; want %q", got, want)
	}
}

// A Bearer token on the MCP request must propagate to core verbatim
// — the LAN path forwards the agent's JWT so core's own auth runs
// against the same identity. No service account, no shared secret.
func TestGet_ForwardsBearerTokenWhenPresent(t *testing.T) {
	var gotAuth string
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer core.Close()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, CoreURLFile), []byte(core.URL), 0o600); err != nil {
		t.Fatalf("write url file: %v", err)
	}

	const fakeToken = "eyJtest.payload.sig"
	if _, err := NewClient(dir, core.Client()).Get(context.Background(), "/anything", fakeToken); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if gotAuth != "Bearer "+fakeToken {
		t.Fatalf("core received Authorization=%q; want %q", gotAuth, "Bearer "+fakeToken)
	}
}

// Loopback callers (no JWT) must NOT receive a forged token — MCP
// makes the call without an Authorization header at all, and core's
// own loopback skip handles the auth.
func TestGet_DoesNotForwardEmptyToken(t *testing.T) {
	var gotAuth string
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer core.Close()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, CoreURLFile), []byte(core.URL), 0o600); err != nil {
		t.Fatalf("write url file: %v", err)
	}

	if _, err := NewClient(dir, core.Client()).Get(context.Background(), "/anything", ""); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if gotAuth != "" {
		t.Fatalf("core received Authorization=%q; want '' (no token to forward)", gotAuth)
	}
}

// core returning a non-2xx must surface as a core_status_<N> error
// with the upstream body included — so an agent can pattern-match
// on "core said no" vs "core unreachable", and the operator sees
// what core actually replied with.
func TestGet_NonOKStatusSurfacesAsTypedError(t *testing.T) {
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"msg":"db migration in progress"}`))
	}))
	defer core.Close()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, CoreURLFile), []byte(core.URL), 0o600); err != nil {
		t.Fatalf("write url file: %v", err)
	}

	_, err := NewClient(dir, core.Client()).Get(context.Background(), "/anything", "")
	pe, ok := err.(*Error)
	if !ok {
		t.Fatalf("non-2xx returned %T; want *Error", err)
	}
	if pe.Code != "core_status_503" {
		t.Fatalf("Code=%q; want core_status_503", pe.Code)
	}
	if !strings.Contains(pe.Body, "db migration in progress") {
		t.Fatalf("Body=%q; want it to include core's response payload so the operator can read what core said", pe.Body)
	}
}

// A transport-level failure (core down, refused connection) must
// (a) surface as core_unavailable AND (b) invalidate the URL cache
// so the next call re-reads the file — handles the case where core
// restarted on a new port.
func TestGet_TransportFailureInvalidatesCache(t *testing.T) {
	// Point at a port nothing's listening on.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, CoreURLFile), []byte("http://127.0.0.1:1"), 0o600); err != nil {
		t.Fatalf("write url file: %v", err)
	}
	c := NewClient(dir, &http.Client{Timeout: 500 * time.Millisecond})
	// Prime the cache.
	if _, err := c.ResolveCore(); err != nil {
		t.Fatalf("ResolveCore: %v", err)
	}

	_, err := c.Get(context.Background(), "/anything", "")
	pe, ok := err.(*Error)
	if !ok {
		t.Fatalf("transport error returned %T; want *Error", err)
	}
	if pe.Code != "core_unavailable" {
		t.Fatalf("Code=%q; want core_unavailable", pe.Code)
	}

	// Verify cache was invalidated: rewrite the .url with a DIFFERENT
	// port, call again, observe we picked it up rather than retrying
	// the dead one.
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer core.Close()
	if err := os.WriteFile(filepath.Join(dir, CoreURLFile), []byte(core.URL), 0o600); err != nil {
		t.Fatalf("rewrite url file: %v", err)
	}
	c.httpClient = core.Client()
	if _, err := c.Get(context.Background(), "/anything", ""); err != nil {
		t.Fatalf("after cache invalidation Get should succeed against the new URL; got %v", err)
	}
}

// The error-as-payload helper produces the canonical shape the agent
// reads: a JSON object with `error`, `detail`, optional `body`, and
// a `fallback` hint pointing at the resources that don't need core.
func TestAsErrorPayload_ShapeContract(t *testing.T) {
	err := &Error{Code: "core_unavailable", Detail: "GET /v1/sys/utilization: dial: connection refused"}
	payload := AsErrorPayload(err)

	var got struct {
		Error    string `json:"error"`
		Detail   string `json:"detail"`
		Body     string `json:"body"`
		Fallback string `json:"fallback"`
	}
	if uerr := json.Unmarshal(payload, &got); uerr != nil {
		t.Fatalf("payload not valid JSON: %v", uerr)
	}
	if got.Error != "core_unavailable" {
		t.Fatalf("Error=%q; want core_unavailable", got.Error)
	}
	if got.Detail == "" {
		t.Fatalf("Detail empty; want the GET-line cause to be preserved")
	}
	if !strings.Contains(got.Fallback, "audit") || !strings.Contains(got.Fallback, "journal") {
		t.Fatalf("Fallback=%q; want it to mention audit + journal so the agent can pivot", got.Fallback)
	}
}
