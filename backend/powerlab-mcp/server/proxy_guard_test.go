package server

import (
	"crypto/ecdsa"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/neochaotic/powerlab/backend/common/utils/jwt"
)

// jwt.HTTPJWT grants the loopback skip from the TCP peer alone. If
// powerlab-mcp were ever fronted by a same-host reverse proxy, every
// forwarded request would arrive from 127.0.0.1 and inherit that trust —
// an auth bypass. These tests pin the guard that closes it: a "loopback"
// request carrying proxy headers must NOT be trusted, while a genuine
// local agent (no proxy headers) keeps its zero-config access.

// The bypass case: a request from 127.0.0.1 that carries X-Forwarded-For
// is a proxied client, not a local agent — it must be made to present a
// token, not waved through.
func TestProxyGuard_LoopbackWithProxyHeaderRequiresToken(t *testing.T) {
	_, pub, err := jwt.GenerateKeyPair()
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	for _, hdr := range proxyHeaders {
		t.Run(hdr, func(t *testing.T) {
			req := mcpInitReq("127.0.0.1:5000", "")
			req.Header.Set(hdr, "203.0.113.7") // claimed upstream client
			rec := httptest.NewRecorder()
			handlerWithKey(pub).ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("loopback request with %s and no token = %d; want 401 (proxy must not inherit loopback trust)", hdr, rec.Code)
			}
		})
	}
}

// The genuine-local case must keep working with zero config: a real
// loopback agent sends no proxy headers and is trusted without a token.
func TestProxyGuard_GenuineLoopbackStillTrusted(t *testing.T) {
	keyResolved := false
	s := newServer(BuildInfo{Version: "test"}, func() (*ecdsa.PublicKey, error) {
		keyResolved = true
		return nil, nil
	})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, mcpInitReq("127.0.0.1:5000", "")) // no proxy headers
	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("genuine loopback (no proxy headers) was gated (401) — the local agent must keep zero-config trust")
	}
	if keyResolved {
		t.Fatalf("genuine loopback resolved the JWT key — it must stay on the trusted path")
	}
}

// A proxied client that DOES authenticate must get through — the guard
// hardens trust, it doesn't ban proxies outright.
func TestProxyGuard_ProxiedWithValidTokenPasses(t *testing.T) {
	priv, pub, err := jwt.GenerateKeyPair()
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	tok, err := jwt.GenerateToken("alice", priv, 1, "powerlab", time.Hour)
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	req := mcpInitReq("127.0.0.1:5000", tok)
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	rec := httptest.NewRecorder()
	handlerWithKey(pub).ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("proxied request with a valid token = 401 — the guard must allow authenticated proxied callers")
	}
}
