package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	pkglogging "github.com/neochaotic/powerlab/backend/pkg/logging"
)

// TestMain initializes the package-level _log so checkURLWithRetry's
// _log.Info() calls don't nil-deref. init() is skipped in test
// binaries (see main.go) — that's intentional, but it leaves _log
// nil. We restore the default here in lieu of the heavy production
// init.
func TestMain(m *testing.M) {
	if _log == nil {
		l, _ := pkglogging.New(pkglogging.Config{Level: "error", Format: "json"})
		_log = l
	}
	os.Exit(m.Run())
}

// These tests lock in the four bugs found during the v0.5.0 → v0.5.1
// upgrade incident (issue #130, summary in issue #129):
//
//   1. checkURL nil-deref on err path (bug-#64 ressuscitado)
//   2. checkURL StatusOK comparison was inverted
//   3. reloadGateway built self-ping URL from listener.Addr() which
//      returns the bind address ([::]:PORT) — invalid as TCP client
//      destination on IPv6-strict configs (clientLoopback fix)
//   4. checkURLWithRetry used uint count with `count >= 0` and
//      `count--` from 0 wraps to MAX_UINT64 → infinite retry
//
// The original bugs hid each other: (1) panicked before (2) could be
// observed, (3) was masked because (4) looped forever, etc. Tests
// here exercise each independently so future regressions can't
// re-introduce one and call it "fixed because the chain still works."

// TestCheckURL_DoesNotPanicOnConnectionRefused — bug 1.
// Original code did `defer response.Body.Close()` unconditionally
// after `if err == nil { return err }`. When err != nil (transport
// failure), response is nil → SIGSEGV.
func TestCheckURL_DoesNotPanicOnConnectionRefused(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("checkURL panicked on a non-listening port: %v", r)
		}
	}()
	// 127.0.0.1:1 is reserved; nothing should be listening.
	err := checkURL("http://127.0.0.1:1/ping")
	if err == nil {
		t.Errorf("expected an error from a non-listening port, got nil")
	}
}

// TestCheckURL_AcceptsAnyHTTPResponse — bug 2 (and the safety net
// for our "broken upgrade" scenario where 8765 returns 301 redirect
// to HTTPS during boot).
//
// The original (broken) code returned nil when err == nil, never
// reaching the StatusCode check. Then someone "fixed" the StatusCode
// check to require 200 — which broke the boot self-ping (sees 301).
// Final semantic: ANY response means the listener is up.
func TestCheckURL_AcceptsAnyHTTPResponse(t *testing.T) {
	cases := []struct {
		name string
		code int
	}{
		{"200 OK", http.StatusOK},
		{"301 Moved", http.StatusMovedPermanently},
		{"302 Found", http.StatusFound},
		{"401 Unauthorized", http.StatusUnauthorized},
		{"500 Internal", http.StatusInternalServerError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(c.code)
			}))
			defer srv.Close()
			if err := checkURL(srv.URL + "/ping"); err != nil {
				t.Errorf("checkURL on %d response: want nil, got %v", c.code, err)
			}
		})
	}
}

// TestClientLoopback_RewritesBindAddressToLoopback — bug 3.
// listener.Addr() returns the BIND address (where the server bound
// to). Using it directly as a CLIENT destination fails on
// IPv6-strict configs because [::] and 0.0.0.0 are wildcard "any
// interface" addresses — valid for binding, invalid for connecting.
func TestClientLoopback_RewritesBindAddressToLoopback(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"[::]:8765", "127.0.0.1:8765"},
		{"0.0.0.0:8765", "127.0.0.1:8765"},
		{"[::]:80", "127.0.0.1:80"},
		// Already-loopback or named-host addresses pass through.
		{"127.0.0.1:8765", "127.0.0.1:8765"},
		{"localhost:8765", "localhost:8765"},
		{"[::1]:8765", "[::1]:8765"},
		{"192.168.1.10:8765", "192.168.1.10:8765"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := clientLoopback(c.in)
			if got != c.want {
				t.Errorf("clientLoopback(%q): want %q, got %q", c.in, c.want, got)
			}
		})
	}
}

// TestCheckURLWithRetry_DoesNotInfiniteLoop — bug 4.
// Original signature `func(url string, retry uint) error` with
// `for count >= 0; count--` is an infinite loop because uint
// underflows from 0 to MAX_UINT64. With the URL bug (3) fixed but
// count bug (4) still present, gateway boot CPU-spins forever.
//
// This test enforces a wall-clock cap: 10 retries against an
// unreachable URL should complete in well under 30 seconds.
func TestCheckURLWithRetry_DoesNotInfiniteLoop(t *testing.T) {
	done := make(chan error, 1)
	start := time.Now()
	go func() {
		// Use 1ms timeout via a small retry count + unreachable port.
		// Each attempt: ~5s for http.Get timeout — but 127.0.0.1:1
		// returns connection-refused immediately, so each iter is
		// ~1s sleep + immediate err. 3 retries total ≤ 5s.
		done <- checkURLWithRetry("http://127.0.0.1:1/ping", 3)
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Errorf("expected error from unreachable URL, got nil")
		}
		if elapsed := time.Since(start); elapsed > 30*time.Second {
			t.Errorf("checkURLWithRetry took %v — much longer than expected for retry=3", elapsed)
		}
	case <-time.After(30 * time.Second):
		t.Fatalf("checkURLWithRetry did not return after 30s — likely the uint-wraparound infinite-loop regression is back")
	}
}

// TestCheckURLWithRetry_StopsAfterMaxAttempts — bug 4 corollary.
// With retry=N, the function should attempt at most N+1 times
// (initial + N retries) and return the last err. Counter must
// terminate; this test fails if the loop runs more attempts than
// expected (which would happen if uint wraparound came back).
func TestCheckURLWithRetry_StopsAfterMaxAttempts(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		// Hijack to simulate connection-reset rather than a normal
		// HTTP response — keeps checkURL in the err != nil path.
		hj, _ := w.(http.Hijacker)
		conn, _, _ := hj.Hijack()
		_ = conn.Close()
	}))
	defer srv.Close()

	parsedURL, _ := url.Parse(srv.URL)
	// Use the test server's actual port so the connection is to a
	// real (but always-failing) endpoint.
	const retries = 4
	err := checkURLWithRetry("http://"+parsedURL.Host+"/ping", retries)
	if err == nil {
		t.Errorf("expected error after all retries failed, got nil")
	}
	if attempts > retries+1 {
		t.Errorf("attempts: want at most %d (initial + %d retries), got %d (uint wraparound regression?)", retries+1, retries, attempts)
	}
	if attempts < 1 {
		t.Errorf("attempts: want at least 1, got %d (loop didn't run?)", attempts)
	}
}

// TestCheckURLWithRetry_SucceedsOnFirstReachable — happy path,
// confirms the success branch breaks the loop instead of running
// to retry exhaustion.
func TestCheckURLWithRetry_SucceedsOnFirstReachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := checkURLWithRetry(srv.URL+"/ping", 5); err != nil {
		t.Errorf("checkURLWithRetry on a reachable URL: want nil, got %v", err)
	}
}

// TestCheckURL_TransportErrorIsReturned — sanity: not all errors
// are SIGSEGV; transport errors (DNS, dial, timeout) should
// propagate to the caller as-is (not be swallowed as nil).
func TestCheckURL_TransportErrorIsReturned(t *testing.T) {
	err := checkURL("http://127.0.0.1:1/ping")
	if err == nil {
		t.Fatal("expected transport error, got nil")
	}
	// Sanity: error mentions the network path so debug logs are
	// useful (don't enforce exact text, just non-empty).
	if !strings.Contains(strings.ToLower(err.Error()), "connect") &&
		!strings.Contains(strings.ToLower(err.Error()), "refused") &&
		!errors.Is(err, context.DeadlineExceeded) {
		t.Logf("note: error message %q didn't mention connect/refused; check if Go std lib changed", err.Error())
	}
}
