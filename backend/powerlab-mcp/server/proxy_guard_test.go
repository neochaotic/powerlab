package server

import (
	"crypto/ecdsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neochaotic/powerlab/backend/common/utils/audit"
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
	}, resourcesConfig{})
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

// The guard rewrites RemoteAddr to the 192.0.2.1 sentinel to force the
// JWT check, but that sentinel must not leak into the audit trail: the
// record has to carry the real TCP peer (#595). X-Real-Ip is used here
// because the audit middleware prefers X-Forwarded-For when present,
// which would mask the bug.
func TestProxyGuard_AuditRecordsRealPeerNotSentinel(t *testing.T) {
	priv, pub, err := jwt.GenerateKeyPair()
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	tok, err := jwt.GenerateToken("alice", priv, 1, "powerlab", time.Hour)
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	auditPath := filepath.Join(t.TempDir(), "audit.jsonl")
	svc, err := audit.NewService(audit.ServiceOptions{Path: auditPath})
	if err != nil {
		t.Fatalf("audit.NewService: %v", err)
	}
	s := newServer(BuildInfo{Version: "test"}, func() (*ecdsa.PublicKey, error) { return pub, nil }, resourcesConfig{})
	s.audit = svc

	req := mcpInitReq("127.0.0.1:5000", tok)
	req.Header.Set("X-Real-Ip", "203.0.113.7")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("authenticated proxied request = 401")
	}
	_ = svc.Close() // flush the async writer

	body, err := os.ReadFile(auditPath) // #nosec G304 -- t.TempDir
	if err != nil {
		t.Fatalf("read audit.jsonl: %v", err)
	}
	var r audit.Record
	if err := json.Unmarshal([]byte(strings.SplitN(strings.TrimSpace(string(body)), "\n", 2)[0]), &r); err != nil {
		t.Fatalf("unmarshal audit line: %v (body=%q)", err, body)
	}
	if r.RemoteIP == "192.0.2.1" {
		t.Fatalf("audit remote_ip = sentinel 192.0.2.1; want the real peer")
	}
	if r.RemoteIP != audit.LoopbackSentinel {
		t.Fatalf("audit remote_ip = %q; want %q (the real loopback peer)", r.RemoteIP, audit.LoopbackSentinel)
	}
}
