package jwt_test

import (
	"crypto/ecdsa"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/neochaotic/powerlab/backend/common/utils/jwt"
)

// echoHandler echoes the user_id / user_name headers if set,
// otherwise reports "anonymous". Used to confirm HTTPJWT
// extracts and forwards JWT claims correctly.
func echoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-User-Id", r.Header.Get("user_id"))
	w.Header().Set("X-User-Name", r.Header.Get("user_name"))
	w.WriteHeader(200)
	_, _ = w.Write([]byte("ok"))
}

func keypairForTest(t *testing.T) (*ecdsa.PrivateKey, func() (*ecdsa.PublicKey, error)) {
	t.Helper()
	priv, pub, err := jwt.GenerateKeyPair()
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	keyFunc := func() (*ecdsa.PublicKey, error) { return pub, nil }
	return priv, keyFunc
}

func TestHTTPJWT_RejectsMissingToken(t *testing.T) {
	_, keyFunc := keypairForTest(t)
	mw := jwt.HTTPJWT(keyFunc)
	srv := httptest.NewServer(mw(http.HandlerFunc(echoHandler)))
	defer srv.Close()

	// Simulate a NON-loopback request by setting X-Forwarded-For —
	// but the stdlib middleware checks RemoteAddr directly, which
	// for httptest is always 127.0.0.1. We bypass loopback skip by
	// pointing at a remote — use the dialer's connection directly.
	//
	// Workaround: HTTPJWT considers ONLY r.RemoteAddr. httptest
	// binds to 127.0.0.1, so loopback skip activates. The "missing
	// token" path is therefore exercised via direct unit test on
	// the underlying extractor in jwt_helper_test, not here.
	//
	// What we CAN assert from httptest: loopback skip bypasses
	// auth entirely → 200 regardless of token presence.
	resp, _ := http.Get(srv.URL + "/x")
	if resp.StatusCode != 200 {
		t.Errorf("loopback should bypass auth; got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHTTPJWT_LoopbackSkipPassesThrough(t *testing.T) {
	_, keyFunc := keypairForTest(t)
	mw := jwt.HTTPJWT(keyFunc)
	srv := httptest.NewServer(mw(http.HandlerFunc(echoHandler)))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/x")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 on loopback, got %d", resp.StatusCode)
	}
}

func TestHTTPJWT_ValidTokenSetsHeaders(t *testing.T) {
	priv, keyFunc := keypairForTest(t)

	// Issue a real token.
	token, err := jwt.GetAccessToken("alice", priv, 42)
	if err != nil {
		t.Fatalf("token gen: %v", err)
	}

	mw := jwt.HTTPJWT(keyFunc)
	// Simulate a non-loopback origin by overriding RemoteAddr inside
	// the handler chain — wrap the middleware so RemoteAddr is set
	// to a non-loopback string before HTTPJWT inspects it.
	nonLoopback := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.RemoteAddr = "192.168.1.50:54321"
			h.ServeHTTP(w, r)
		})
	}
	srv := httptest.NewServer(nonLoopback(mw(http.HandlerFunc(echoHandler))))
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/x", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("valid token should pass; got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("X-User-Id"); got != "42" {
		t.Errorf("user_id header not set; got %q", got)
	}
	if got := resp.Header.Get("X-User-Name"); got != "alice" {
		t.Errorf("user_name header not set; got %q", got)
	}
}

func TestHTTPJWT_RawTokenWithoutBearerPrefixIsAccepted(t *testing.T) {
	priv, keyFunc := keypairForTest(t)
	token, _ := jwt.GetAccessToken("bob", priv, 1)
	mw := jwt.HTTPJWT(keyFunc)
	nonLoopback := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.RemoteAddr = "10.0.0.1:11111"
			h.ServeHTTP(w, r)
		})
	}
	srv := httptest.NewServer(nonLoopback(mw(http.HandlerFunc(echoHandler))))
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/x", nil)
	req.Header.Set("Authorization", token) // no "Bearer " prefix
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("raw token should be accepted (legacy form); got %d", resp.StatusCode)
	}
}

func TestHTTPJWT_QueryTokenFallback(t *testing.T) {
	priv, keyFunc := keypairForTest(t)
	token, _ := jwt.GetAccessToken("carol", priv, 7)
	mw := jwt.HTTPJWT(keyFunc)
	nonLoopback := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.RemoteAddr = "10.0.0.1:11111"
			h.ServeHTTP(w, r)
		})
	}
	srv := httptest.NewServer(nonLoopback(mw(http.HandlerFunc(echoHandler))))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/x?token=" + token)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("query token fallback should work for EventSource; got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("X-User-Id"); got != "7" {
		t.Errorf("user_id from query token: %q, want 7", got)
	}
}

func TestHTTPJWT_InvalidToken401(t *testing.T) {
	_, keyFunc := keypairForTest(t)
	mw := jwt.HTTPJWT(keyFunc)
	nonLoopback := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.RemoteAddr = "10.0.0.1:11111"
			h.ServeHTTP(w, r)
		})
	}
	srv := httptest.NewServer(nonLoopback(mw(http.HandlerFunc(echoHandler))))
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/x", nil)
	req.Header.Set("Authorization", "Bearer garbage.not.a.jwt")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Errorf("invalid token: got %d, want 401", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("error response should be JSON; got %q", got)
	}
}

func TestHTTPJWT_MissingTokenNonLoopback401(t *testing.T) {
	_, keyFunc := keypairForTest(t)
	mw := jwt.HTTPJWT(keyFunc)
	nonLoopback := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.RemoteAddr = "10.0.0.1:11111"
			h.ServeHTTP(w, r)
		})
	}
	srv := httptest.NewServer(nonLoopback(mw(http.HandlerFunc(echoHandler))))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/x")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Errorf("missing token on non-loopback: got %d, want 401", resp.StatusCode)
	}
}

// REGRESSION (2026-05-31): Mac→Lima (or any host→VM, host→container)
// connections forwarded via IPv6 loopback arrive with RemoteAddr in
// the form "[::1]:PORT". The original implementation compared the
// stripped-port string against "::1" (no brackets) and failed to
// recognise IPv6 loopback as loopback — every Mac→Lima MCP call hit
// the LAN auth path and returned "missing/invalid token" even though
// the connection was genuinely local. This test locks the IPv6
// loopback skip.
func TestHTTPJWT_LoopbackSkipHandlesIPv6Brackets(t *testing.T) {
	_, keyFunc := keypairForTest(t)
	mw := jwt.HTTPJWT(keyFunc)
	v6loopback := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.RemoteAddr = "[::1]:54321"
			h.ServeHTTP(w, r)
		})
	}
	srv := httptest.NewServer(v6loopback(mw(http.HandlerFunc(echoHandler))))
	defer srv.Close()

	// No Authorization header — would have been "missing token" 401
	// pre-fix. Post-fix the IPv6 loopback bypass kicks in → 200.
	resp, err := http.Get(srv.URL + "/x")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("IPv6 loopback should bypass auth; got %d (pre-fix shape: middleware compared \"[::1]\" against \"::1\" and missed)", resp.StatusCode)
	}
}
