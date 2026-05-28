package server

import (
	"crypto/ecdsa"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/neochaotic/powerlab/backend/common/utils/jwt"
)

// The MCP endpoint is the read tier (ADR-0034): free from loopback (the
// trusted local agent), JWT-gated from the LAN. These tests drive the
// real jwt.HTTPJWT gate with real signed tokens — no mock — by setting
// RemoteAddr directly, which exercises the LAN path that httptest's own
// loopback-only server cannot reach.

const lanAddr = "192.0.2.10:5000" // TEST-NET-1, never loopback

func mcpInitReq(remoteAddr, bearer string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, MCPEndpointPath,
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remoteAddr
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	return req
}

func handlerWithKey(pub *ecdsa.PublicKey) http.Handler {
	return newServer(BuildInfo{Version: "test"}, func() (*ecdsa.PublicKey, error) {
		return pub, nil
	}, resourcesConfig{}).Handler()
}

// A LAN caller with no token must be rejected — the whole reason the
// surface can bind to all interfaces is that the gate, not the bind,
// protects it.
func TestGate_LANWithoutTokenIs401(t *testing.T) {
	_, pub, err := jwt.GenerateKeyPair()
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	rec := httptest.NewRecorder()
	handlerWithKey(pub).ServeHTTP(rec, mcpInitReq(lanAddr, ""))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("LAN /mcp without token = %d; want 401", rec.Code)
	}
}

// A LAN caller with a validly-signed PowerLab token must get through the
// gate and reach the MCP handler (proven by a non-401, non-404).
func TestGate_LANWithValidTokenPasses(t *testing.T) {
	priv, pub, err := jwt.GenerateKeyPair()
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	tok, err := jwt.GenerateToken("alice", priv, 1, "powerlab", time.Hour)
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	rec := httptest.NewRecorder()
	handlerWithKey(pub).ServeHTTP(rec, mcpInitReq(lanAddr, tok))
	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("LAN /mcp with a valid token was rejected (401) — the gate must accept PowerLab-issued tokens")
	}
	if rec.Code == http.StatusNotFound {
		t.Fatalf("LAN /mcp with a valid token = 404 — it passed the gate but never reached the mounted MCP handler")
	}
}

// A garbage/forged token must be rejected, not silently treated as
// anonymous-but-allowed.
func TestGate_LANWithBadTokenIs401(t *testing.T) {
	_, pub, err := jwt.GenerateKeyPair()
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	rec := httptest.NewRecorder()
	handlerWithKey(pub).ServeHTTP(rec, mcpInitReq(lanAddr, "not.a.real.token"))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("LAN /mcp with a bad token = %d; want 401", rec.Code)
	}
}

// Loopback is the trusted local-agent path: it must bypass auth entirely
// and must NOT even attempt to resolve the JWT public key (which would
// fail on a box where the user-service isn't reachable — exactly when an
// operator needs local observability most).
func TestGate_LoopbackBypassesAuthAndKeyLookup(t *testing.T) {
	keyResolved := false
	s := newServer(BuildInfo{Version: "test"}, func() (*ecdsa.PublicKey, error) {
		keyResolved = true
		return nil, errors.New("key lookup must not happen for loopback")
	}, resourcesConfig{})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, mcpInitReq("127.0.0.1:54321", ""))
	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("loopback /mcp was gated (401) — the read tier must skip loopback")
	}
	if keyResolved {
		t.Fatalf("loopback path resolved the JWT key — local observability must not depend on the user-service being up")
	}
}

// The control endpoints must answer from the LAN with no token — they
// are liveness/identity, not protected data.
func TestGate_ControlEndpointsOpenFromLAN(t *testing.T) {
	_, pub, err := jwt.GenerateKeyPair()
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	h := handlerWithKey(pub)
	for _, path := range []string{"/healthz", "/version"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = lanAddr
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s from LAN = %d; want 200 (control endpoints must not be gated)", path, rec.Code)
		}
	}
}
