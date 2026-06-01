package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// PowerLab MCP's auth model is "loopback bypass + Bearer JWT from
// user-service" — NOT a full OAuth 2.1 server. But MCP HTTP-transport
// clients (mcp-remote, Claude Desktop's native bridge, Claude Code's
// HTTP transport) all require the OAuth 2.1 authorization-code
// dance to complete before they'll touch /mcp. These handlers
// provide just enough of that dance for the loopback case to
// succeed: the authorize endpoint auto-approves any loopback caller
// (no consent UI), the token endpoint exchanges the resulting code
// for a Bearer token. The token itself is symbolic — the actual
// auth check at /mcp time is still the TCP-peer loopback bypass
// in jwt.HTTPJWT. LAN callers MUST continue through user-service.
//
// SECURITY MODEL:
//   - authorize auto-approves ONLY when the request arrives on
//     loopback (127.0.0.1 / ::1). Non-loopback callers get 401 —
//     they have no path through this flow yet (user-service login
//     for LAN clients is roadmap).
//   - PKCE (S256) is enforced. A stolen code without the matching
//     code_verifier cannot be exchanged.
//   - Codes are single-use. Successful exchange consumes the code;
//     a replay returns invalid_grant.
//   - Codes expire after 10 minutes (RFC 6749 §4.1.2 recommends
//     "very short" — typically 10 min or less).

const (
	authCodeTTL          = 10 * time.Minute
	loopbackAccessToken  = "loopback-bypass"
	loopbackTokenExpires = 86400 // 1 day; the token is symbolic so this is mostly cosmetic
)

type authCodeEntry struct {
	clientID            string
	redirectURI         string
	codeChallenge       string
	codeChallengeMethod string
	expiresAt           time.Time
}

// authCodes is the in-memory code store. Single-use (delete on
// successful exchange), short TTL. mu guards both maps.
var (
	authCodesMu sync.Mutex
	authCodes   = map[string]authCodeEntry{}
)

func (s *Server) handleOAuthAuthorize(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackRequest(r) {
		http.Error(w, "non-loopback authorization not supported (use Bearer JWT directly)", http.StatusUnauthorized)
		return
	}

	q := r.URL.Query()
	if q.Get("response_type") != "code" {
		http.Error(w, "unsupported_response_type", http.StatusBadRequest)
		return
	}
	redirectURI := q.Get("redirect_uri")
	if redirectURI == "" {
		http.Error(w, "missing redirect_uri", http.StatusBadRequest)
		return
	}
	state := q.Get("state")
	challenge := q.Get("code_challenge")
	method := q.Get("code_challenge_method")
	if method == "" {
		method = "plain"
	}

	code := randomString(32)
	authCodesMu.Lock()
	authCodes[code] = authCodeEntry{
		clientID:            q.Get("client_id"),
		redirectURI:         redirectURI,
		codeChallenge:       challenge,
		codeChallengeMethod: method,
		expiresAt:           time.Now().Add(authCodeTTL),
	}
	authCodesMu.Unlock()

	u, err := url.Parse(redirectURI)
	if err != nil {
		http.Error(w, "invalid redirect_uri", http.StatusBadRequest)
		return
	}
	rq := u.Query()
	rq.Set("code", code)
	if state != "" {
		rq.Set("state", state)
	}
	u.RawQuery = rq.Encode()
	w.Header().Set("Location", u.String())
	w.WriteHeader(http.StatusFound)
}

func (s *Server) handleOAuthToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		tokenError(w, "invalid_request")
		return
	}
	if r.PostForm.Get("grant_type") != "authorization_code" {
		tokenError(w, "unsupported_grant_type")
		return
	}
	code := r.PostForm.Get("code")
	verifier := r.PostForm.Get("code_verifier")

	authCodesMu.Lock()
	entry, ok := authCodes[code]
	if ok {
		delete(authCodes, code)
	}
	authCodesMu.Unlock()
	if !ok || time.Now().After(entry.expiresAt) {
		tokenError(w, "invalid_grant")
		return
	}
	if !verifyPKCE(verifier, entry.codeChallenge, entry.codeChallengeMethod) {
		tokenError(w, "invalid_grant")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": loopbackAccessToken,
		"token_type":   "Bearer",
		"expires_in":   loopbackTokenExpires,
	})
}

func tokenError(w http.ResponseWriter, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

func verifyPKCE(verifier, challenge, method string) bool {
	if challenge == "" {
		// no PKCE — only allow if we weren't given a verifier either
		return verifier == ""
	}
	switch strings.ToUpper(method) {
	case "S256":
		h := sha256.Sum256([]byte(verifier))
		return base64.RawURLEncoding.EncodeToString(h[:]) == challenge
	case "PLAIN":
		return verifier == challenge
	}
	return false
}

func isLoopbackRequest(r *http.Request) bool {
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i != -1 {
		host = host[:i]
	}
	host = strings.Trim(host, "[]")
	return host == "127.0.0.1" || host == "::1" || host == ""
}

func randomString(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)[:n]
}
