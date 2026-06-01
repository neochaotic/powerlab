package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/neochaotic/powerlab/backend/common/utils/audit"
)

// Issue #644 Layer 2: enrich the audit Record with the JSON-RPC
// `method` + relevant params so operators can answer "what tool did
// the agent call?" — not just "the agent hit /mcp 200". The HTTP
// middleware can't see inside the body without consuming it before
// the MCP SDK gets it, so this file pairs a body-tee that pre-parses
// the envelope with an Enricher (audit.HTTPMiddlewareOptions hook)
// that fills Kind + Payload from the parsed value.
//
// Architectural note: ideally we'd hook at the MCP SDK level
// (server.AddTool's handler chain) so the audit record reflects the
// SDK's own dispatch decision. The SDK doesn't (yet) expose a
// per-request audit hook, so we parse the envelope ourselves at the
// HTTP boundary. Trade-off: a malformed JSON body that the SDK would
// reject still produces an empty-kind audit row (correct: the request
// landed, no semantic action). When the SDK adds a dispatcher hook,
// this code becomes redundant and can be deleted in one PR.

// mcpAuditCtxKey scopes the per-request parsed JSON-RPC envelope on
// the request context. Unexported so no caller outside this package
// can pollute it.
type mcpAuditCtxKey struct{}

// mcpAuditPayload is the subset of the JSON-RPC envelope the audit
// enricher cares about. Decoded once per request by parseMCPBody;
// read out by enrichAuditWithMCPPayload.
type mcpAuditPayload struct {
	Method   string `json:"method"`
	ToolName string // resolved from params.name (tools/call)
	URI      string // resolved from params.uri  (resources/read)
}

// mcpPayloadMaxParseBytes caps how much of the request body the tee
// will buffer for parsing. The full body is still forwarded to the
// SDK via the rewound reader; this cap only bounds the audit-side
// memory we hold for parsing. 64 KiB is generous for JSON-RPC
// envelopes (the typical message is < 4 KiB; the rare large tool
// call with a big arguments map stays well under 64 KiB).
const mcpPayloadMaxParseBytes = 64 * 1024

// teeMCPBodyForAudit reads the request body up to mcpPayloadMaxParseBytes,
// parses the JSON-RPC envelope, stashes the relevant fields on the
// request context, then REPLACES r.Body with an io.NopCloser around
// the buffered bytes so the SDK reads the same bytes verbatim. This
// preserves the SDK's parse + dispatch — the tee is invisible to the
// inner handler.
//
// On any failure (no body, body larger than the cap, invalid JSON,
// missing method) the request continues unmodified and the audit
// record will land with empty Kind + Payload. The MCP SDK gets the
// untouched body via the rewound reader.
//
// Chain placement: must run BEFORE the MCP transport handler so the
// body-replacement happens before any reader. Sits inside the audit
// middleware (which reads the context AFTER ServeHTTP).
func teeMCPBodyForAudit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil {
			next.ServeHTTP(w, r)
			return
		}
		// Read up to cap+1 so we can detect oversize. ReadAll on a
		// limited reader returns the bytes consumed without error
		// when the limit is hit — we treat hitting the cap as "skip
		// enrichment" to avoid acting on a truncated structure.
		limited := io.LimitReader(r.Body, mcpPayloadMaxParseBytes+1)
		buf, err := io.ReadAll(limited)
		if err != nil {
			// Read failure → forward the (now potentially partial)
			// body and skip enrichment. Don't fail the request from
			// the audit side; the MCP SDK will surface a real error
			// if the body is unusable.
			r.Body = io.NopCloser(bytes.NewReader(buf))
			next.ServeHTTP(w, r)
			return
		}
		// Drain whatever the LimitReader left behind so the original
		// body is fully consumed (irrelevant once we replace r.Body,
		// but defensive against future Close() blocking on unread
		// bytes). Errors are intentionally ignored — we already have
		// what we need.
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
		// Reset r.Body to the buffered bytes so the SDK reads the
		// same envelope verbatim.
		r.Body = io.NopCloser(bytes.NewReader(buf))

		// Skip enrichment if the body exceeded the parse cap (a
		// truncated structure can't be trusted to identify the tool).
		if len(buf) > mcpPayloadMaxParseBytes {
			next.ServeHTTP(w, r)
			return
		}
		if payload, ok := parseMCPEnvelope(buf); ok {
			r = r.WithContext(context.WithValue(r.Context(), mcpAuditCtxKey{}, payload))
		}
		next.ServeHTTP(w, r)
	})
}

// parseMCPEnvelope decodes the JSON-RPC envelope and pulls out the
// method + the per-method identifier (tool name or resource URI).
// Returns ok=false on any failure — the caller treats that as "no
// enrichment".
func parseMCPEnvelope(buf []byte) (mcpAuditPayload, bool) {
	// Two-pass decode: first the envelope shape, then the params
	// based on method. Lets us keep the params decoding cheap (only
	// the methods we audit get full param decode).
	var env struct {
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(buf, &env); err != nil || env.Method == "" {
		return mcpAuditPayload{}, false
	}
	out := mcpAuditPayload{Method: env.Method}
	switch env.Method {
	case "tools/call":
		var p struct {
			Name string `json:"name"`
		}
		if jerr := json.Unmarshal(env.Params, &p); jerr == nil {
			out.ToolName = p.Name
		}
	case "resources/read":
		var p struct {
			URI string `json:"uri"`
		}
		if jerr := json.Unmarshal(env.Params, &p); jerr == nil {
			out.URI = p.URI
		}
	}
	return out, true
}

// enrichAuditWithMCPPayload is the audit.HTTPMiddlewareOptions.Enricher
// for /mcp. It reads the parsed envelope off the request context (set
// by teeMCPBodyForAudit) and fills Kind + Payload on the captured
// Record. Only the two methods the operator cares about
// (tools/call + resources/read) get elevated to a Kind value; other
// methods (initialize, ping, tools/list, resources/list, ...) leave
// the record at the default Kind="" — they're transport-layer
// chatter, not auditable business events. Same kind taxonomy as
// ADR-0047 §"Decision/1. MCP adopts audit.Middleware()".
func enrichAuditWithMCPPayload(r *http.Request, rec *audit.Record) {
	v := r.Context().Value(mcpAuditCtxKey{})
	if v == nil {
		return
	}
	p, ok := v.(mcpAuditPayload)
	if !ok {
		return
	}
	switch p.Method {
	case "tools/call":
		rec.Kind = audit.KindMCPToolCall
		if p.ToolName != "" {
			rec.Payload = map[string]any{"tool_name": p.ToolName}
		}
	case "resources/read":
		rec.Kind = audit.KindMCPResourceRead
		if p.URI != "" {
			rec.Payload = map[string]any{"uri": p.URI}
		}
	}
}
