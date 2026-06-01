package route

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// REGRESSION (2026-06-01): IPv6 bracket-parsing bug class — Mac→VM
// (limactl, Docker Desktop) connections arrive with `RemoteAddr =
// "[::1]:PORT"`. The pre-fix LastIndex(":")-based slice left brackets
// in the result, so the `remoteIP == "::1"` check missed every IPv6
// loopback request and the X-Forwarded-For rewrite path was skipped.
// Same bug fixed in jwt + audit middleware; this locks the gateway
// path.
func TestRewriteRequestSourceIP_IPv6BracketedLoopback(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "[::1]:54321"
	r.Header.Set("X-Forwarded-For", "203.0.113.5")

	rewriteRequestSourceIP(r)

	// Pre-fix: rewriteRequestSourceIP saw `remoteIP="[::1]"` (bracketed),
	// failed the `== "::1"` check, and stripped X-Forwarded-For without
	// re-adding the genuine client IP. Post-fix: brackets stripped,
	// IPv6 loopback recognised, X-Forwarded-For carries the real client.
	got := r.Header.Get("X-Forwarded-For")
	if got != "203.0.113.5" {
		t.Fatalf("X-Forwarded-For=%q after rewrite; want %q (IPv6 loopback should be recognised so the upstream IP survives)",
			got, "203.0.113.5")
	}
}

// Sanity: IPv4 loopback path still works the same way it always did.
// Without this, a refactor that breaks the IPv6 case might break IPv4
// too without anyone noticing.
func TestRewriteRequestSourceIP_IPv4LoopbackStillRecognised(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "127.0.0.1:54321"
	r.Header.Set("X-Forwarded-For", "203.0.113.5")

	rewriteRequestSourceIP(r)

	got := r.Header.Get("X-Forwarded-For")
	if got != "203.0.113.5" {
		t.Fatalf("X-Forwarded-For=%q after rewrite; want %q for IPv4 loopback", got, "203.0.113.5")
	}
}
