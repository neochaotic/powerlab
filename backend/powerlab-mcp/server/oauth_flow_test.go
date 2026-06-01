package server

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// Auto-approval contract: a loopback caller arrives via Mac→limactl
// forward as 127.0.0.1. PowerLab's existing jwt.HTTPJWT middleware
// already grants loopback callers full access without a Bearer. The
// authorize endpoint must extend that trust: any loopback caller
// gets an immediate code redirect, no consent UI, no JWT required.
// LAN callers (non-loopback) MUST be rejected here — they don't
// have an OAuth-2.1-compliant login flow to fall through to yet.
func TestOAuthAuthorize_LoopbackAutoApprovesWithCode(t *testing.T) {
	s := &Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/authorize", s.handleOAuthAuthorize)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	verifier := "abcdefghijklmnopqrstuvwxyz0123456789abcdefghij"
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", "mcp-loopback")
	q.Set("redirect_uri", "http://localhost:9999/callback")
	q.Set("state", "xyz")
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get(srv.URL + "/oauth/authorize?" + q.Encode())
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 300 || resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d; want 3xx redirect. body=%s", resp.StatusCode, string(body))
	}
	loc := resp.Header.Get("Location")
	if loc == "" {
		t.Fatalf("Location header missing")
	}
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	if u.Scheme != "http" || u.Host != "localhost:9999" || u.Path != "/callback" {
		t.Fatalf("redirected to %q; want callback URL", loc)
	}
	rq := u.Query()
	if rq.Get("code") == "" {
		t.Fatalf("redirect missing ?code=...: %s", loc)
	}
	if rq.Get("state") != "xyz" {
		t.Fatalf("state=%q; want xyz (RFC 6749 §4.1.2 — must echo client state)", rq.Get("state"))
	}
}

// PKCE round-trip: client posts the verifier whose SHA-256 matches
// the challenge it sent at /authorize. Server returns an access
// token. The token's exact value is unspecified by the spec — what
// matters is that PowerLab honors it on subsequent /mcp calls.
// Loopback bypass at request time means the actual auth check is
// the TCP peer; the token is symbolic. But the response MUST be
// well-formed per RFC 6749 §5.1 or mcp-remote rejects it.
func TestOAuthToken_PKCEExchangeReturnsAccessToken(t *testing.T) {
	s := &Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/authorize", s.handleOAuthAuthorize)
	mux.HandleFunc("/oauth/token", s.handleOAuthToken)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	verifier := "abcdefghijklmnopqrstuvwxyz0123456789abcdefghij"
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", "mcp-loopback")
	q.Set("redirect_uri", "http://localhost:9999/cb")
	q.Set("state", "xyz")
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	noRedirect := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := noRedirect.Get(srv.URL + "/oauth/authorize?" + q.Encode())
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	loc, _ := url.Parse(resp.Header.Get("Location"))
	code := loc.Query().Get("code")
	_ = resp.Body.Close()

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", "http://localhost:9999/cb")
	form.Set("client_id", "mcp-loopback")
	form.Set("code_verifier", verifier)

	tresp, err := http.PostForm(srv.URL+"/oauth/token", form)
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	defer func() { _ = tresp.Body.Close() }()
	if tresp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(tresp.Body)
		t.Fatalf("token status=%d; body=%s", tresp.StatusCode, string(body))
	}
	if ct := tresp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type=%q; want application/json (RFC 6749 §5.1)", ct)
	}
	var out struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(tresp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.AccessToken == "" {
		t.Fatalf("access_token empty (RFC 6749 §5.1 — REQUIRED)")
	}
	if !strings.EqualFold(out.TokenType, "Bearer") {
		t.Fatalf("token_type=%q; want Bearer (case-insensitive)", out.TokenType)
	}
}

// PKCE NEGATIVE: a wrong verifier MUST be rejected. Without this
// check the code-challenge dance provides zero security — anyone
// who steals the code could trivially exchange it without proving
// they're the original client.
func TestOAuthToken_WrongPKCEVerifierRejected(t *testing.T) {
	s := &Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/authorize", s.handleOAuthAuthorize)
	mux.HandleFunc("/oauth/token", s.handleOAuthToken)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	verifier := "abcdefghijklmnopqrstuvwxyz0123456789abcdefghij"
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", "mcp-loopback")
	q.Set("redirect_uri", "http://localhost:9999/cb")
	q.Set("state", "xyz")
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	noRedirect := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := noRedirect.Get(srv.URL + "/oauth/authorize?" + q.Encode())
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	loc, _ := url.Parse(resp.Header.Get("Location"))
	code := loc.Query().Get("code")
	_ = resp.Body.Close()

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", "http://localhost:9999/cb")
	form.Set("client_id", "mcp-loopback")
	form.Set("code_verifier", "wrong-verifier-that-does-not-hash-to-the-challenge")

	tresp, err := http.PostForm(srv.URL+"/oauth/token", form)
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	defer func() { _ = tresp.Body.Close() }()
	if tresp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d; want 400 (invalid_grant)", tresp.StatusCode)
	}
}

// Code single-use: exchanging the same code twice MUST fail. RFC 6749
// §4.1.2 mandates this — a stolen code could otherwise be replayed.
func TestOAuthToken_CodeIsSingleUse(t *testing.T) {
	s := &Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/authorize", s.handleOAuthAuthorize)
	mux.HandleFunc("/oauth/token", s.handleOAuthToken)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	verifier := "abcdefghijklmnopqrstuvwxyz0123456789abcdefghij"
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", "mcp-loopback")
	q.Set("redirect_uri", "http://localhost:9999/cb")
	q.Set("state", "xyz")
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	noRedirect := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := noRedirect.Get(srv.URL + "/oauth/authorize?" + q.Encode())
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	loc, _ := url.Parse(resp.Header.Get("Location"))
	code := loc.Query().Get("code")
	_ = resp.Body.Close()

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", "http://localhost:9999/cb")
	form.Set("client_id", "mcp-loopback")
	form.Set("code_verifier", verifier)

	first, err := http.PostForm(srv.URL+"/oauth/token", form)
	if err != nil {
		t.Fatalf("first exchange: %v", err)
	}
	_ = first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first exchange status=%d; want 200", first.StatusCode)
	}

	second, err := http.PostForm(srv.URL+"/oauth/token", form)
	if err != nil {
		t.Fatalf("second exchange: %v", err)
	}
	defer func() { _ = second.Body.Close() }()
	if second.StatusCode != http.StatusBadRequest {
		t.Fatalf("second exchange status=%d; want 400 (code consumed)", second.StatusCode)
	}
}

// AS metadata advertises the flows we actually implement. mcp-remote
// uses this to decide whether to attempt authorization_code; if we
// don't advertise it, mcp-remote aborts with "Incompatible auth
// server: does not support response type code".
func TestOAuthAuthorizationServer_AdvertisesAuthCodeFlow(t *testing.T) {
	s := &Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/oauth-authorization-server", s.handleOAuthAuthorizationServer)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/.well-known/oauth-authorization-server")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var meta authorizationServerMetadata
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		t.Fatalf("decode: %v", err)
	}

	hasCode := false
	for _, rt := range meta.ResponseTypesSupported {
		if rt == "code" {
			hasCode = true
		}
	}
	if !hasCode {
		t.Fatalf("response_types_supported=%v; must include \"code\"", meta.ResponseTypesSupported)
	}

	hasAuthCode := false
	for _, gt := range meta.GrantTypesSupported {
		if gt == "authorization_code" {
			hasAuthCode = true
		}
	}
	if !hasAuthCode {
		t.Fatalf("grant_types_supported=%v; must include \"authorization_code\"", meta.GrantTypesSupported)
	}

	hasS256 := false
	for _, m := range meta.CodeChallengeMethodsSupported {
		if m == "S256" {
			hasS256 = true
		}
	}
	if !hasS256 {
		t.Fatalf("code_challenge_methods_supported=%v; must include \"S256\" (RFC 7636 mandatory for confidential clients)", meta.CodeChallengeMethodsSupported)
	}
}
