package jwt_test

// HTTPDecodeOnly contract (Sprint 18 P0):
//
// The strict HTTPJWT middleware (already shipped) enforces auth —
// returns 401 on missing/invalid token. That's correct for endpoints
// the gateway serves directly (/v1/audit/*).
//
// But the gateway's public catch-all proxies to backend services —
// each backend validates its own JWT, so the gateway-side enforcement
// would double-401 legitimate traffic. The AUDIT middleware needs
// user_id populated BEFORE it runs, but it must never block a request
// that doesn't have a valid token (the backend handles that).
//
// HTTPDecodeOnly fills this niche:
//   - if a valid JWT is present → set user_id/user_name request headers
//   - if no token or invalid token → pass through with no headers
//   - NEVER return 401 — auth enforcement is downstream

import (
	"crypto/ecdsa"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/neochaotic/powerlab/backend/common/utils/jwt"
)

// echoUserHeader writes back the user_id / user_name headers the
// middleware sets so test assertions can read them out.
func echoUserHeader(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Echo-User-Id", r.Header.Get("user_id"))
	w.Header().Set("X-Echo-User-Name", r.Header.Get("user_name"))
	w.WriteHeader(http.StatusOK)
}

func decodeOnlyKeypair(t *testing.T) (*ecdsa.PrivateKey, func() (*ecdsa.PublicKey, error)) {
	t.Helper()
	priv, pub, err := jwt.GenerateKeyPair()
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	return priv, func() (*ecdsa.PublicKey, error) { return pub, nil }
}

func TestHTTPDecodeOnly_ValidTokenSetsHeaders(t *testing.T) {
	priv, keyFunc := decodeOnlyKeypair(t)
	tok, err := jwt.GetAccessToken("alice", priv, 42)
	if err != nil {
		t.Fatalf("token: %v", err)
	}

	mw := jwt.HTTPDecodeOnly(keyFunc)
	srv := httptest.NewServer(mw(http.HandlerFunc(echoUserHeader)))
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("X-Echo-User-Id"); got != "42" {
		t.Errorf("user_id header: got %q, want %q", got, "42")
	}
	if got := resp.Header.Get("X-Echo-User-Name"); got != "alice" {
		t.Errorf("user_name header: got %q, want %q", got, "alice")
	}
}

func TestHTTPDecodeOnly_MissingTokenPassesThroughWithoutHeaders(t *testing.T) {
	_, keyFunc := decodeOnlyKeypair(t)
	mw := jwt.HTTPDecodeOnly(keyFunc)
	srv := httptest.NewServer(mw(http.HandlerFunc(echoUserHeader)))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/x")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("must NOT 401 anonymous; got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("X-Echo-User-Id"); got != "" {
		t.Errorf("user_id should be empty for anonymous; got %q", got)
	}
}

func TestHTTPDecodeOnly_InvalidTokenPassesThroughWithoutHeaders(t *testing.T) {
	_, keyFunc := decodeOnlyKeypair(t)
	mw := jwt.HTTPDecodeOnly(keyFunc)
	srv := httptest.NewServer(mw(http.HandlerFunc(echoUserHeader)))
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/x", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-jwt")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("invalid token must NOT 401 at this layer; got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("X-Echo-User-Id"); got != "" {
		t.Errorf("user_id should be empty when token invalid; got %q", got)
	}
}

func TestHTTPDecodeOnly_RawTokenWithoutBearerPrefix(t *testing.T) {
	priv, keyFunc := decodeOnlyKeypair(t)
	tok, _ := jwt.GetAccessToken("bob", priv, 11)

	mw := jwt.HTTPDecodeOnly(keyFunc)
	srv := httptest.NewServer(mw(http.HandlerFunc(echoUserHeader)))
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/x", nil)
	req.Header.Set("Authorization", tok) // no "Bearer " prefix
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("X-Echo-User-Id"); got != "11" {
		t.Errorf("raw-token form should still decode; got %q", got)
	}
}

func TestHTTPDecodeOnly_QueryTokenFallback(t *testing.T) {
	priv, keyFunc := decodeOnlyKeypair(t)
	tok, _ := jwt.GetAccessToken("carol", priv, 17)

	mw := jwt.HTTPDecodeOnly(keyFunc)
	srv := httptest.NewServer(mw(http.HandlerFunc(echoUserHeader)))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/x?token=" + tok)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("X-Echo-User-Id"); got != "17" {
		t.Errorf("query token should decode; got %q", got)
	}
}
