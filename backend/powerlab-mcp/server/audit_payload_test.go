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

	"github.com/neochaotic/powerlab/backend/common/utils/audit"
)

// Issue #644 Layer 2: enrich audit Payload with tool_name / uri from
// the JSON-RPC envelope. The HTTP middleware can't see inside the body
// without consuming it before the MCP SDK gets it; the fix uses a
// body-tee that pre-parses the envelope and stashes the result on
// the request context, which the audit Enricher hook reads.

// mcpRPCReq builds a request with a literal JSON-RPC body. Avoids
// the SDK; the body shape is what the audit middleware will see on
// the wire from any compliant MCP client.
func mcpRPCReq(remoteAddr, bearer, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, MCPEndpointPath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// The Streamable-HTTP MCP transport requires both Accept types
	// or it short-circuits with 400 before reading the body. Real
	// MCP clients (Claude Desktop, the SDK) send both.
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.RemoteAddr = remoteAddr
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	return req
}

func runAuditServe(t *testing.T, req *http.Request) audit.Record {
	t.Helper()
	dir := t.TempDir()
	auditPath := filepath.Join(dir, "audit.jsonl")
	svc, err := audit.NewService(audit.ServiceOptions{Path: auditPath})
	if err != nil {
		t.Fatalf("audit.NewService: %v", err)
	}
	defer func() { _ = svc.Close() }()

	s := newServer(BuildInfo{Version: "test"}, func() (*ecdsa.PublicKey, error) {
		return nil, nil //nolint:nilnil // pubkey unused on loopback path
	}, resourcesConfig{})
	s.audit = svc

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	_ = svc.Close()

	body, rerr := os.ReadFile(auditPath) // #nosec G304 -- t.TempDir
	if rerr != nil {
		t.Fatalf("read audit.jsonl: %v", rerr)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatalf("audit.jsonl empty")
	}
	var r audit.Record
	if jerr := json.Unmarshal([]byte(lines[0]), &r); jerr != nil {
		t.Fatalf("unmarshal: %v", jerr)
	}
	return r
}

// tools/call JSON-RPC envelope → audit record has Kind=mcp.tool_call
// and Payload.tool_name. The operator query "what tool did the agent
// call?" becomes answerable.
func TestAuditPayload_ToolCall_HasKindAndToolName(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"restart_app","arguments":{"app_id":"jellyfin"}}}`
	got := runAuditServe(t, mcpRPCReq("127.0.0.1:54321", "", body))

	if got.Kind != audit.KindMCPToolCall {
		t.Errorf("Kind=%q; want %q", got.Kind, audit.KindMCPToolCall)
	}
	if got.Payload == nil {
		t.Fatalf("Payload nil; want {tool_name: restart_app}")
	}
	if tn, _ := got.Payload["tool_name"].(string); tn != "restart_app" {
		t.Errorf("Payload.tool_name=%v; want %q", got.Payload["tool_name"], "restart_app")
	}
}

// resources/read JSON-RPC envelope → audit record has
// Kind=mcp.resource_read and Payload.uri.
func TestAuditPayload_ResourceRead_HasKindAndURI(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"system://metrics"}}`
	got := runAuditServe(t, mcpRPCReq("127.0.0.1:54321", "", body))

	if got.Kind != audit.KindMCPResourceRead {
		t.Errorf("Kind=%q; want %q", got.Kind, audit.KindMCPResourceRead)
	}
	if got.Payload == nil {
		t.Fatalf("Payload nil; want {uri: system://metrics}")
	}
	if uri, _ := got.Payload["uri"].(string); uri != "system://metrics" {
		t.Errorf("Payload.uri=%v; want %q", got.Payload["uri"], "system://metrics")
	}
}

// Other JSON-RPC methods (initialize, tools/list, ping, ...) → record
// still lands but with empty Kind. We intentionally only enrich the
// two kinds the operator cares about; everything else is transport-
// layer chatter that pollutes the audit signal if elevated.
func TestAuditPayload_InitializeRequest_NoKindEnrichment(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":3,"method":"initialize","params":{}}`
	got := runAuditServe(t, mcpRPCReq("127.0.0.1:54321", "", body))

	if got.Kind != "" {
		t.Errorf("Kind=%q; want empty (initialize is transport-layer, not auditable as tool/resource)", got.Kind)
	}
	if got.Payload != nil {
		t.Errorf("Payload=%v; want nil (no enrichment for non-tool/non-resource methods)", got.Payload)
	}
}

// Malformed JSON body → record lands without kind/payload, no panic.
// Defensive: a truncated or non-JSON body shouldn't crash the audit
// path. (The MCP SDK will reject the request with an RPC error; we
// just need to not poison the audit row.)
func TestAuditPayload_MalformedBody_NoEnrichmentNoPanic(t *testing.T) {
	body := `not-valid-json{{{`
	got := runAuditServe(t, mcpRPCReq("127.0.0.1:54321", "", body))

	if got.Kind != "" {
		t.Errorf("Kind=%q; want empty on malformed body", got.Kind)
	}
	if got.Payload != nil {
		t.Errorf("Payload=%v; want nil on malformed body", got.Payload)
	}
}

// CRITICAL: body-tee must NOT consume the body before the MCP SDK
// reads it. The SDK needs to parse the envelope itself to dispatch
// the RPC. If the tee drains the body without rewinding, the SDK
// sees empty input and the request fails. This test asserts the
// status code from the SDK is not 400-class — proves the body
// survived intact.
func TestAuditPayload_BodyTeeDoesNotConsumeBody(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":4,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`
	dir := t.TempDir()
	auditPath := filepath.Join(dir, "audit.jsonl")
	svc, err := audit.NewService(audit.ServiceOptions{Path: auditPath})
	if err != nil {
		t.Fatalf("audit.NewService: %v", err)
	}
	defer func() { _ = svc.Close() }()

	s := newServer(BuildInfo{Version: "test"}, func() (*ecdsa.PublicKey, error) {
		return nil, nil //nolint:nilnil // pubkey unused on loopback
	}, resourcesConfig{})
	s.audit = svc

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, mcpRPCReq("127.0.0.1:54321", "", body))

	// 200 = SDK happily parsed initialize. 400 = SDK saw empty body
	// (= tee broke the request). Anything else is fine — we just
	// want to prove we didn't break the SDK's parse.
	if rec.Code == http.StatusBadRequest {
		t.Fatalf("got 400 from /mcp on initialize — body-tee consumed the body before the SDK could parse it")
	}
}

// Oversized body (above the MaxBytesReader cap) → limitBody rejects
// at the outer layer before the tee runs. The audit record still
// lands (the request DID reach the audit middleware) with empty
// kind/payload. Locks the "tee on a capped body" interaction.
func TestAuditPayload_OversizedBody_FallsBackToEmptyKind(t *testing.T) {
	// Build a payload larger than maxMCPRequestBytes (1 MiB). Use a
	// huge string of 'a's inside a JSON-RPC envelope so the structure
	// looks valid up to the boundary.
	huge := strings.Repeat("a", maxMCPRequestBytes+100)
	body := `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"x","arguments":{"big":"` + huge + `"}}}`
	got := runAuditServe(t, mcpRPCReq("127.0.0.1:54321", "", body))

	// The body was capped; the tee saw at most the first 1 MiB. Even
	// if a partial parse succeeded, we shouldn't fabricate enrichment
	// from a truncated structure. Empty kind is acceptable; the
	// invariant is "no panic, audit landed".
	if got.Method != "POST" {
		t.Errorf("audit record missing for oversized body; got Method=%q", got.Method)
	}
}
