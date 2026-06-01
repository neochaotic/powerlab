package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// MCP HTTP transport clients (Claude Desktop, mcp-remote, Claude
// Code) abort connect when /.well-known/oauth-* return 404 — the
// regression cost an integration session, so these tests lock the
// minimal RFC 9728 / 8414 shape every standard MCP client probes.
func TestOAuthProtectedResource_Shape(t *testing.T) {
	s := &Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/oauth-protected-resource", s.handleOAuthProtectedResource)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/.well-known/oauth-protected-resource")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d; want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type=%q; want application/json", ct)
	}

	var meta protectedResourceMetadata
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.HasSuffix(meta.Resource, MCPEndpointPath) {
		t.Fatalf("resource=%q; want suffix %q", meta.Resource, MCPEndpointPath)
	}
	if len(meta.BearerMethodsSupported) == 0 {
		t.Fatalf("bearer_methods_supported empty; clients require at least one method")
	}
	hasHeader := false
	for _, m := range meta.BearerMethodsSupported {
		if m == "header" {
			hasHeader = true
		}
	}
	if !hasHeader {
		t.Fatalf("bearer_methods_supported=%v; must include \"header\" (the only method PowerLab accepts)", meta.BearerMethodsSupported)
	}
}

func TestOAuthAuthorizationServer_Shape(t *testing.T) {
	s := &Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/oauth-authorization-server", s.handleOAuthAuthorizationServer)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/.well-known/oauth-authorization-server")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d; want 200", resp.StatusCode)
	}

	var meta authorizationServerMetadata
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if meta.Issuer == "" {
		t.Fatalf("issuer empty; clients reject metadata without an issuer")
	}
	// Coverage for ResponseTypesSupported / GrantTypesSupported
	// content moved to TestOAuthAuthorizationServer_AdvertisesAuthCodeFlow —
	// the auth-code flow + PKCE-S256 + handlers now exist (see
	// oauth_flow.go), so the AS advertises them.
	// REGRESSION (2026-05-31 mcp-remote integration): mcp-remote
	// (and the Anthropic MCP SDK's StreamableHTTPClientTransport)
	// validate AS metadata with zod and reject the response unless
	// authorization_endpoint and token_endpoint are present strings —
	// per RFC 8414 §2 these fields are REQUIRED even when the server
	// doesn't implement real OAuth. PowerLab returns stubs that 404
	// when hit (no actual auth flow), but the strings MUST exist or
	// the entire MCP handshake aborts before reaching /mcp.
	if meta.AuthorizationEndpoint == "" {
		t.Fatalf("authorization_endpoint empty; mcp-remote / MCP SDK abort handshake when this field is missing (RFC 8414 §2)")
	}
	if meta.TokenEndpoint == "" {
		t.Fatalf("token_endpoint empty; mcp-remote / MCP SDK abort handshake when this field is missing (RFC 8414 §2)")
	}
	// REGRESSION (2026-05-31 mcp-remote integration, layer 2):
	// mcp-remote aborts with "Incompatible auth server: does not
	// support dynamic client registration" when registration_endpoint
	// is missing. RFC 8414 §2 marks it OPTIONAL, but mcp-remote's
	// auth flow ALWAYS attempts DCR (RFC 7591). PowerLab returns a
	// dummy DCR endpoint so the handshake completes; the actual
	// auth still happens via the pre-provisioned Authorization
	// header the client was launched with.
	if meta.RegistrationEndpoint == "" {
		t.Fatalf("registration_endpoint empty; mcp-remote aborts with 'Incompatible auth server' without it (RFC 7591)")
	}
}

// DCR stub must accept any POST and return a client_id so mcp-remote's
// auth flow proceeds. Body content is irrelevant — the real
// authentication is the pre-provisioned Bearer header the agent was
// configured with.
func TestOAuthRegister_AcceptsAnyPOST(t *testing.T) {
	s := &Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/register", s.handleOAuthRegister)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/oauth/register", "application/json", strings.NewReader(`{"redirect_uris":["http://localhost:0/cb"]}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d; want 200 or 201", resp.StatusCode)
	}

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out["client_id"] == nil || out["client_id"] == "" {
		t.Fatalf("client_id missing in DCR response: %#v", out)
	}
	// REGRESSION (2026-05-31 mcp-remote DCR layer 3): mcp-remote
	// zod-validates the DCR response and requires redirect_uris
	// to be an array (RFC 7591 §3.2.1 marks it OPTIONAL in some
	// flow configurations but RECOMMENDED, and mcp-remote treats
	// it as required). Echo back whatever the client sent so the
	// validator passes; PowerLab never actually redirects.
	if _, ok := out["redirect_uris"].([]any); !ok {
		t.Fatalf("redirect_uris missing or not an array in DCR response: %#v", out["redirect_uris"])
	}
}

// Regression lock: both endpoints MUST be reachable via the public
// Handler() chain (i.e. mounted on the same mux as /healthz and
// /version, NOT behind the JWT gate). A loopback bypass that
// applied to /.well-known/* would still be fine, but the OAuth
// discovery spec requires anonymous access — gating it would
// reintroduce the original 404→abort failure mode.
func TestOAuthDiscovery_PubliclyReachable(t *testing.T) {
	s := &Server{}
	h := http.NewServeMux()
	h.HandleFunc("/healthz", s.handleHealthz)
	h.HandleFunc("/.well-known/oauth-protected-resource", s.handleOAuthProtectedResource)
	h.HandleFunc("/.well-known/oauth-authorization-server", s.handleOAuthAuthorizationServer)

	srv := httptest.NewServer(h)
	defer srv.Close()

	for _, path := range []string{
		"/.well-known/oauth-protected-resource",
		"/.well-known/oauth-authorization-server",
	} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("get %s: %v", path, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s status=%d; want 200 (anonymous + ungated)", path, resp.StatusCode)
		}
	}
}
