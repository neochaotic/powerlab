package server

import (
	"encoding/json"
	"net/http"
)

// MCP HTTP transport clients (Claude Desktop, mcp-remote, Claude
// Code) abort connect when /.well-known/oauth-* return 404 (MCP
// 2025-03-26 §2.4). PowerLab uses Bearer JWT from user-service, not
// a full OAuth 2.1 server — these stubs declare "Bearer-only, no
// dynamic registration" so compliant clients fall through to the
// Authorization header they already hold.

type protectedResourceMetadata struct {
	Resource               string   `json:"resource"`
	AuthorizationServers   []string `json:"authorization_servers,omitempty"`
	BearerMethodsSupported []string `json:"bearer_methods_supported"`
}

type authorizationServerMetadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	RegistrationEndpoint              string   `json:"registration_endpoint"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
}

func (s *Server) handleOAuthProtectedResource(w http.ResponseWriter, r *http.Request) {
	// authorization_servers DELIBERATELY omitted (RFC 9728 §3.2
	// allows it). Advertising it sends MCP clients into a full
	// OAuth 2.1 + DCR + authorization-code dance (mcp-remote does
	// exactly this), and PowerLab does not run an authorization
	// server. Without authorization_servers the client uses the
	// Bearer header it was launched with — which is the actual
	// PowerLab auth model.
	writeJSON(w, protectedResourceMetadata{
		Resource:               absoluteURL(r, MCPEndpointPath),
		BearerMethodsSupported: []string{"header"},
	})
}

func (s *Server) handleOAuthAuthorizationServer(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, authorizationServerMetadata{
		Issuer:                            absoluteURL(r, ""),
		AuthorizationEndpoint:             absoluteURL(r, "/oauth/authorize"),
		TokenEndpoint:                     absoluteURL(r, "/oauth/token"),
		RegistrationEndpoint:              absoluteURL(r, "/oauth/register"),
		ResponseTypesSupported:            []string{"code"},
		GrantTypesSupported:               []string{"authorization_code"},
		TokenEndpointAuthMethodsSupported: []string{"none"},
		CodeChallengeMethodsSupported:     []string{"S256"},
	})
}

// handleOAuthRegister is a DCR stub (RFC 7591). PowerLab does not
// run a real OAuth server — the actual auth is the pre-provisioned
// Bearer header the agent was launched with. This handler exists
// solely so mcp-remote's DCR step succeeds; the returned client_id
// is never used to mint tokens.
func (s *Server) handleOAuthRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RedirectURIs []string `json:"redirect_uris"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.RedirectURIs == nil {
		req.RedirectURIs = []string{}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"client_id":                  "mcp-loopback",
		"token_endpoint_auth_method": "none",
		"redirect_uris":              req.RedirectURIs,
	})
}

func absoluteURL(r *http.Request, path string) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host + path
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
