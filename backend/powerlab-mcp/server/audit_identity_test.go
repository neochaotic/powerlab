package server

import (
	"crypto/ecdsa"
	"encoding/json"
	"errors"
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

// Regression suite for issue #644: ADR-0047 audit dogfood shipped, but
// audit records carried `user_id:null, username:null` for EVERY /mcp
// request. Root cause: jwt.HTTPJWT SKIPS validation on loopback (by
// design — trusted local agent), so it never sets user_id / user_name
// request headers. The audit middleware then reads empty headers and
// records null. Layer 1 fix: a thin middleware between jwt.HTTPJWT and
// audit.HTTPMiddleware that, when the headers are empty AND an
// Authorization Bearer token is present, decodes the JWT (without
// re-validation — same trust model as AgentIdentity) and populates
// user_id / user_name from `id` + `username` claims.
//
// Trust model: this is audit-only / informational. The signature
// trust decision already happened upstream (jwt.HTTPJWT for LAN; the
// loopback-bind boundary for loopback). We are not re-validating —
// we're surfacing identity the validator either ALREADY proved (LAN)
// or chose not to assert (loopback). An attacker on loopback who can
// craft a forged JWT could pollute audit lines with a chosen username
// — but they already had loopback access; the audit record is not
// security-bearing for that path. Operators reading audit lines see
// the agent's chosen identity, which is what we want.

// serveAuditTest builds the real /mcp Handler with an audit service
// attached, exercises one request through it, then drains and returns
// the in-memory audit ring (newest-first). Keeping the helper local
// avoids touching the gate_test.go fixtures (different concern).
func serveAuditTest(t *testing.T, req *http.Request, pubKeyErr error) audit.Record {
	t.Helper()
	dir := t.TempDir()
	auditPath := filepath.Join(dir, "audit.jsonl")
	svc, err := audit.NewService(audit.ServiceOptions{Path: auditPath})
	if err != nil {
		t.Fatalf("audit.NewService: %v", err)
	}
	defer func() { _ = svc.Close() }()

	pubKey := func() (*ecdsa.PublicKey, error) {
		if pubKeyErr != nil {
			return nil, pubKeyErr
		}
		return nil, errors.New("not used")
	}
	s := newServer(BuildInfo{Version: "test"}, pubKey, resourcesConfig{})
	s.audit = svc

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	// Recorder Submit is async; flush by closing the service. Close
	// drains the writer goroutine + flushes to JSONL. After Close
	// the ring is still readable.
	_ = svc.Close()

	// Read the JSONL file — the ring's Recent() is the convenient
	// path but reading the file directly is the strongest assertion:
	// it's what an operator greps. Empty file = no audit landed.
	body, rerr := os.ReadFile(auditPath) // #nosec G304 -- t.TempDir
	if rerr != nil {
		t.Fatalf("read audit.jsonl: %v", rerr)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatalf("audit.jsonl empty — no record landed for the request")
	}
	var r audit.Record
	if jerr := json.Unmarshal([]byte(lines[0]), &r); jerr != nil {
		t.Fatalf("unmarshal first audit line: %v\nline=%q", jerr, lines[0])
	}
	return r
}

// Loopback + valid JWT → audit record must carry user_id + username
// from the JWT claims. THIS IS THE BUG: today the record is
// user_id:null because jwt.HTTPJWT skips loopback and the audit
// middleware sees no user_id header.
func TestAudit_LoopbackWithJWT_RecordsUserIdentity(t *testing.T) {
	priv, _, err := jwt.GenerateKeyPair()
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	const wantID = 42
	const wantUser = "alice"
	tok, err := jwt.GenerateToken(wantUser, priv, wantID, "powerlab", time.Hour)
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	req := mcpInitReq("127.0.0.1:54321", tok)

	got := serveAuditTest(t, req, nil)

	if got.UserID == nil {
		t.Fatalf("user_id is nil; want %d (loopback request with Bearer JWT must enrich identity for audit)", wantID)
	}
	if *got.UserID != wantID {
		t.Errorf("user_id=%d; want %d", *got.UserID, wantID)
	}
	if got.Username == nil {
		t.Fatalf("username is nil; want %q (loopback request with Bearer JWT must enrich identity for audit)", wantUser)
	}
	if *got.Username != wantUser {
		t.Errorf("username=%q; want %q", *got.Username, wantUser)
	}
}

// Loopback + NO JWT → audit record must stay user_id:null. The
// loopback-trusted local agent without a token is anonymous BY
// DESIGN (per ADR-0034 / ADR-0047). This is the negative test
// proving the Layer 1 fix doesn't fabricate identity.
func TestAudit_LoopbackWithoutJWT_StaysAnonymous(t *testing.T) {
	req := mcpInitReq("127.0.0.1:54321", "")

	got := serveAuditTest(t, req, nil)

	if got.UserID != nil {
		t.Errorf("user_id=%v; want nil (loopback without JWT must remain anonymous — no identity to enrich)", *got.UserID)
	}
	if got.Username != nil {
		t.Errorf("username=%v; want nil", *got.Username)
	}
	if got.RemoteIP != audit.LoopbackSentinel {
		t.Errorf("remote_ip=%q; want %q (loopback sentinel preserved)", got.RemoteIP, audit.LoopbackSentinel)
	}
}

// Loopback + malformed Bearer → audit record stays anonymous. Defensive:
// a garbage token on loopback is still loopback-trusted (jwt.HTTPJWT
// skipped it), and we MUST NOT crash or fabricate identity from a
// malformed payload.
func TestAudit_LoopbackWithMalformedBearer_StaysAnonymous(t *testing.T) {
	req := mcpInitReq("127.0.0.1:54321", "not-a-jwt")

	got := serveAuditTest(t, req, nil)

	if got.UserID != nil {
		t.Errorf("user_id=%v; want nil (malformed JWT must not poison the audit record)", *got.UserID)
	}
	if got.Username != nil {
		t.Errorf("username=%v; want nil", *got.Username)
	}
}

// LAN + valid JWT → audit record carries the JWT identity. The LAN
// path goes through jwt.HTTPJWT's full validation, which sets the
// user_id / user_name headers from the verified Claims. The
// enrichAuditIdentity middleware MUST NOT clobber them (the security
// path's decision is authoritative). End-to-end this exercises the
// "real LAN call with JWT → correct user_id in audit" acceptance
// criterion from issue #644.
func TestAudit_LANWithValidJWT_RecordsUserIdentity(t *testing.T) {
	priv, pub, err := jwt.GenerateKeyPair()
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	const wantID = 7
	const wantUser = "bob"
	tok, err := jwt.GenerateToken(wantUser, priv, wantID, "powerlab", time.Hour)
	if err != nil {
		t.Fatalf("token: %v", err)
	}

	dir := t.TempDir()
	auditPath := filepath.Join(dir, "audit.jsonl")
	svc, err := audit.NewService(audit.ServiceOptions{Path: auditPath})
	if err != nil {
		t.Fatalf("audit.NewService: %v", err)
	}
	defer func() { _ = svc.Close() }()

	s := newServer(BuildInfo{Version: "test"}, func() (*ecdsa.PublicKey, error) {
		return pub, nil
	}, resourcesConfig{})
	s.audit = svc

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, mcpInitReq(lanAddr, tok))
	_ = svc.Close()

	body, rerr := os.ReadFile(auditPath) // #nosec G304 -- t.TempDir
	if rerr != nil {
		t.Fatalf("read audit.jsonl: %v", rerr)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatalf("audit.jsonl empty")
	}
	var got audit.Record
	if jerr := json.Unmarshal([]byte(lines[0]), &got); jerr != nil {
		t.Fatalf("unmarshal: %v", jerr)
	}

	if got.UserID == nil || *got.UserID != wantID {
		var have int64
		if got.UserID != nil {
			have = *got.UserID
		}
		t.Errorf("user_id=%d; want %d (LAN path with valid JWT must record verified identity)", have, wantID)
	}
	if got.Username == nil || *got.Username != wantUser {
		var have string
		if got.Username != nil {
			have = *got.Username
		}
		t.Errorf("username=%q; want %q", have, wantUser)
	}
}
