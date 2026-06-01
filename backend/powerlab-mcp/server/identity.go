package server

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
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
	c, ok := decodeJWTClaims(token)
	if !ok {
		return ""
	}
	if c.Sub != "" {
		return c.Sub
	}
	// PowerLab's JWT uses `username` as the canonical name claim
	// (see backend/user-service/route/v1/user.go and the existing
	// jwt.HTTPJWT middleware that reads claims.Username). Fall
	// through to that when `sub` is absent — defensive across
	// future claim renames.
	return c.Username
}

// jwtClaims is the subset of PowerLab's JWT payload the audit
// enrichment cares about. Defined here (not imported from common/jwt)
// to keep this helper dependency-free and side-effect-free: any
// failure to decode degrades to "no identity", never panics.
type jwtClaims struct {
	Sub      string `json:"sub"`
	Username string `json:"username"`
	ID       int    `json:"id"`
}

// decodeJWTClaims parses the (unverified) payload of a JWT. ok=false
// on any parse failure. SAME safety note as AgentIdentity: this is
// audit-only / informational; the validator already ran upstream OR
// chose to skip (loopback) — we are not re-validating.
func decodeJWTClaims(token string) (claims jwtClaims, ok bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return jwtClaims{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Some JWT issuers pad — try standard URL decoding too.
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return jwtClaims{}, false
		}
	}
	if jerr := json.Unmarshal(payload, &claims); jerr != nil {
		return jwtClaims{}, false
	}
	return claims, true
}

// enrichAuditIdentity surfaces JWT-borne identity into the
// user_id / user_name request headers when jwt.HTTPJWT did NOT set
// them. The audit middleware reads those headers verbatim, so this
// makes audit records for loopback callers carrying a Bearer JWT
// (e.g. Claude Desktop on the same box authenticated against PowerLab)
// record the AGENT's identity instead of null.
//
// Chain placement (per server.Handler): runs AFTER jwt.HTTPJWT, BEFORE
// audit.HTTPMiddleware. The order matters:
//
//   - jwt.HTTPJWT — the security gate. On LAN it validates the token
//     and sets user_id + user_name from the verified Claims. On
//     loopback it skips entirely (ADR-0034 trusted local agent).
//   - enrichAuditIdentity — IF the headers are already set (LAN path),
//     no-op. Otherwise (loopback + Bearer present) decode the JWT and
//     set the headers from `id` + `username` claims. Never overwrites
//     a header set by jwt.HTTPJWT — the security decision wins.
//   - audit.HTTPMiddleware — reads user_id + user_name into the audit
//     record.
//
// TRUST MODEL: this enrichment does NOT re-validate the signature.
// Closes issue #644: ADR-0047 audit shipped but every /mcp record
// carried user_id:null. The threat model:
//   - Loopback caller with a forged JWT: already trusted (had loopback
//     access); record reflects the agent's chosen identity. Not a new
//     trust escalation — the action would have succeeded as anonymous-
//     loopback regardless; the audit row is more accurate, not less
//     secure. Operators reading audit lines understand "loopback"
//     identity is host-asserted.
//   - LAN caller: jwt.HTTPJWT already ran the full validation; the
//     headers it set always win because we only fill empties.
//   - Garbage token: decodeJWTClaims returns ok=false; the headers
//     stay empty and the audit record stays null. Same behaviour as
//     before the fix.
func enrichAuditIdentity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If jwt.HTTPJWT already populated identity, don't touch it.
		// The security path's decision is authoritative; we are an
		// audit-only fallback for the loopback-skip path.
		if r.Header.Get("user_id") != "" || r.Header.Get("user_name") != "" {
			next.ServeHTTP(w, r)
			return
		}
		_, token, isLoopback := AgentIdentity(r.Header)
		if isLoopback || token == "" {
			// No Authorization Bearer present — audit record stays
			// anonymous, which is the correct shape for the
			// trusted-local-agent-without-JWT path.
			next.ServeHTTP(w, r)
			return
		}
		claims, ok := decodeJWTClaims(token)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		if claims.ID != 0 {
			r.Header.Set("user_id", strconv.Itoa(claims.ID))
		}
		if claims.Username != "" {
			r.Header.Set("user_name", claims.Username)
		} else if claims.Sub != "" {
			// Defensive: future tokens may use `sub` as the canonical
			// name claim. Same fallback chain as jwtSubClaim.
			r.Header.Set("user_name", claims.Sub)
		}
		next.ServeHTTP(w, r)
	})
}
