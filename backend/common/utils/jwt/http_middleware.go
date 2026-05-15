package jwt

import (
	"crypto/ecdsa"
	"net/http"
	"strconv"
	"strings"
)

// HTTPJWT returns a stdlib http.Handler middleware that mirrors the
// Echo JWT middleware behaviour:
//
//   - Skips validation when the request comes from loopback
//     (127.0.0.1 / ::1) — internal management calls and health
//     probes don't need to round-trip a token.
//   - Otherwise, extracts the bearer token from the Authorization
//     header (or ?token= query for EventSource), validates it via
//     publicKeyFunc, and sets the "user_id" request header for
//     downstream handlers / audit middleware.
//   - On invalid token: writes 401 with a small JSON body and stops
//     the chain.
//
// Used by GatewayRoute (the public stdlib mux) for endpoints the
// gateway serves directly — most notably /v1/audit/*. Routes that
// proxy to backend services don't go through this; backends do
// their own validation against the same JWKS.
func HTTPJWT(publicKeyFunc func() (*ecdsa.PublicKey, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			remote := r.RemoteAddr
			if i := strings.LastIndexByte(remote, ':'); i > 0 {
				remote = remote[:i]
			}
			if remote == "127.0.0.1" || remote == "::1" {
				// Loopback skip — same as Echo Skipper.
				next.ServeHTTP(w, r)
				return
			}

			token := extractTokenFromHTTP(r)
			if token == "" {
				writeJSON(w, http.StatusUnauthorized, `{"success":40001,"message":"missing token"}`)
				return
			}
			valid, claims, err := Validate(token, publicKeyFunc)
			if err != nil || !valid || claims == nil {
				writeJSON(w, http.StatusUnauthorized, `{"success":40001,"message":"invalid token"}`)
				return
			}
			r.Header.Set("user_id", strconv.Itoa(claims.ID))
			r.Header.Set("user_name", claims.Username)
			next.ServeHTTP(w, r)
		})
	}
}

// HTTPDecodeOnly is the audit-friendly variant of HTTPJWT:
//
//   - If a valid JWT is present (Authorization header or ?token=
//     query), decode it and set the "user_id" and "user_name"
//     request headers so downstream middleware (notably the audit
//     middleware) can record the authenticated identity.
//   - If no token is present, or the token is invalid, pass through
//     to next with NO headers set. NEVER returns 401.
//
// Use this on the gateway's PUBLIC mux outer layer so the audit
// middleware captures user_id for proxied requests. Auth enforcement
// happens downstream — either at the backend service (which validates
// the JWT itself) or at HTTPJWT for endpoints the gateway serves
// directly (audit endpoints).
func HTTPDecodeOnly(publicKeyFunc func() (*ecdsa.PublicKey, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractTokenFromHTTP(r)
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}
			valid, claims, err := Validate(token, publicKeyFunc)
			if err == nil && valid && claims != nil {
				r.Header.Set("user_id", strconv.Itoa(claims.ID))
				r.Header.Set("user_name", claims.Username)
			}
			// Token absent / invalid / lookup error → pass through with
			// no headers. Auth enforcement is downstream.
			next.ServeHTTP(w, r)
		})
	}
}

// extractTokenFromHTTP mirrors ExtractTokenFromRequest (Echo) but
// for stdlib http.Request.
func extractTokenFromHTTP(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth != "" {
		// Strip case-insensitive "Bearer " prefix per RFC 6750.
		if len(auth) > 7 && strings.EqualFold(auth[:7], "Bearer ") {
			return strings.TrimSpace(auth[7:])
		}
		return strings.TrimSpace(auth)
	}
	return r.URL.Query().Get("token")
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}
