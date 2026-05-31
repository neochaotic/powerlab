package server

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// tokenFromRequest is the typed convenience wrapper around
// AgentIdentity that handler call sites use. Defends against a nil
// request or a nil Extra (the SDK guarantees Extra is non-nil in
// transport-driven calls but the in-process in-test path can pass
// a bare request).
//
// Returns (sub, token, isLoopback) like AgentIdentity. Splitting
// the per-call header-read here keeps the resource/tool handlers'
// JWT-forwarding code one line:
//
//	_, token, _ := tokenFromRequest(req)
//	body, err := proxy.GetFrom(ctx, service, path, token)
func tokenFromRequest(req *mcp.ReadResourceRequest) (sub string, token string, isLoopback bool) {
	if req == nil || req.Extra == nil || req.Extra.Header == nil {
		return "", "", true
	}
	return AgentIdentity(req.Extra.Header)
}

// tokenFromToolRequest mirrors tokenFromRequest for the tool-call
// surface (CallToolRequest). Same SDK shape (RequestExtra) but the
// generic parameter differs, so a separate typed wrapper avoids
// reflection / type-assertion at every tool handler.
func tokenFromToolRequest(req *mcp.CallToolRequest) (sub string, token string, isLoopback bool) {
	if req == nil || req.Extra == nil || req.Extra.Header == nil {
		return "", "", true
	}
	return AgentIdentity(req.Extra.Header)
}

// AgentIdentity extracts the JWT subject + raw token from the HTTP
// headers an MCP handler receives via the SDK's req.Extra.Header
// (the SDK's RequestExtra struct surfaces the original HTTP request
// headers so handlers don't need to reach across the transport
// boundary). Per ADR-0047 this is the foundation for two concerns:
//
//  1. Audit dogfood — the audit middleware reads user_id / user_name
//     headers (set upstream by jwt.HTTPJWT after validation); this
//     helper is the inner-handler equivalent so MCP resource/tool
//     handlers can shape per-call audit records.
//  2. JWT forwarding — handlers pass the raw token to
//     coreproxy.Client so the upstream's own audit middleware
//     records the agent's user, not "loopback".
//
// Returns three values to make the calling-site read naturally:
//   - sub: the JWT 'sub' claim, OR empty when no valid Bearer header
//   - token: the raw JWT string (forward verbatim to upstream)
//   - isLoopback: true when no Authorization header is present
//     OR the scheme is not Bearer. The caller treats isLoopback as
//     the "anonymous trusted local agent" path — same semantics as
//     jwt.HTTPJWT's loopback skip at the gate layer.
//
// SAFETY NOTE: this helper does NOT validate the JWT signature. The
// validation already ran at jwt.HTTPJWT BEFORE the request reached
// the MCP handler. If the validation had failed, the request would
// have been rejected with 401 before getting here. We parse the
// payload's `sub` claim purely to identify the user for audit +
// forwarding; the signature trust comes from the upstream gate.
func AgentIdentity(h http.Header) (sub string, token string, isLoopback bool) {
	authz := h.Get("Authorization")
	if authz == "" {
		return "", "", true
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(authz, prefix) {
		// Non-Bearer scheme (Basic, etc.) — we don't speak those.
		// Defensive fallback: treat as loopback so the handler still
		// works in the "no usable identity" path.
		return "", "", true
	}
	token = strings.TrimSpace(strings.TrimPrefix(authz, prefix))
	if token == "" {
		return "", "", true
	}
	sub = jwtSubClaim(token)
	// We keep isLoopback=false here even if the sub turned out
	// empty (malformed payload). The token IS present; an upstream
	// might still want to receive and re-validate it. Forwarding the
	// token verbatim is the safer default.
	return sub, token, false
}

// jwtSubClaim parses the (unverified) payload of a JWT and returns
// the 'sub' claim. Returns empty string on any parse failure —
// malformed tokens are not our problem to surface; the validator
// already approved this one.
//
// We intentionally do NOT pull in a JWT library here. A JWT is
// three base64-url segments separated by '.'; the payload is the
// middle one. Hand-rolled because (a) the trust is established
// upstream and (b) the dependency surface stays smaller — same
// reasoning as ADR-0044's "no gopsutil bumps in MCP" discipline.
func jwtSubClaim(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Some JWT issuers pad — try standard URL decoding too.
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return ""
		}
	}
	var claims struct {
		Sub      string `json:"sub"`
		Username string `json:"username"`
	}
	if jerr := json.Unmarshal(payload, &claims); jerr != nil {
		return ""
	}
	if claims.Sub != "" {
		return claims.Sub
	}
	// PowerLab's JWT uses `username` as the canonical name claim
	// (see backend/user-service/route/v1/user.go and the existing
	// jwt.HTTPJWT middleware that reads claims.Username). Fall
	// through to that when `sub` is absent — defensive across
	// future claim renames.
	return claims.Username
}
