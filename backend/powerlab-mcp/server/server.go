// Package server wires powerlab-mcp's HTTP surface: the control
// endpoints (/healthz, /version) the systemd unit and install smoke
// poll, plus the MCP transport (Streamable HTTP, the 2025-06-18 spec
// transport) mounted at /mcp.
//
// The MCP endpoint is gated at the read tier (ADR-0034): reachable
// freely from loopback (the trusted local agent / dogfood case), but a
// LAN caller must present a valid PowerLab user-service JWT. The
// control endpoints stay open — a health probe that needs a token is
// not a health probe. The auth/admin tiers for state-changing tools are
// enforced per-tool via MCP middleware once tools exist.
//
// No resources or tools are registered yet — those land in the
// follow-up PRs (system://, journal://, audit://).
package server

import (
	"crypto/ecdsa"
	"encoding/json"
	"net/http"
	"strings"

	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/neochaotic/powerlab/backend/common/external"
	"github.com/neochaotic/powerlab/backend/common/utils/jwt"
	"github.com/neochaotic/powerlab/backend/powerlab-mcp/config"
)

// MCPEndpointPath is the single HTTP path the MCP Streamable-HTTP
// transport is served on. Exported so tests (and later the CLI client)
// can address it without hardcoding the string twice.
const MCPEndpointPath = "/mcp"

// BuildInfo carries the ldflags-injected build identity surfaced by
// /version. main sets these from -X main.{version,commit,date}.
type BuildInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

// publicKeyFunc resolves the JWT-validation public key. It matches the
// signature jwt.Validate / jwt.HTTPJWT expect so the gate can be reused
// as-is.
type publicKeyFunc func() (*ecdsa.PublicKey, error)

// Server holds the constructed MCP transport, the build identity, and
// the JWT public-key resolver used by the read-tier gate. Build it with
// New, mount it with Handler.
type Server struct {
	info    BuildInfo
	httpMCP *mcpserver.StreamableHTTPServer
	pubKey  publicKeyFunc
}

// New constructs the MCP server and its Streamable-HTTP transport,
// resolving the JWT public key from the user-service JWKS published
// under cfg.RuntimePath. It does not bind a listener — Handler returns
// the mux for the caller to serve.
//
// No resources or tools are registered yet; the follow-up PRs will
// reach the underlying MCPServer to register them.
func New(cfg config.Config, info BuildInfo) (*Server, error) {
	return newServer(info, func() (*ecdsa.PublicKey, error) {
		return external.GetPublicKey(cfg.RuntimePath)
	}), nil
}

// newServer is the dependency-injected constructor: tests pass a
// pubKeyFunc backed by a known test key so the gate's JWT validation is
// exercised for real (no mock), without standing up a user-service. It
// reads host metrics from the real /proc.
func newServer(info BuildInfo, pubKey publicKeyFunc) *Server {
	return newServerWithProcRoot(info, pubKey, "/proc")
}

// newServerWithProcRoot additionally lets a test point the system://
// resource at a fixture /proc directory, so the MCP read path is
// exercised end-to-end with deterministic data on any OS.
func newServerWithProcRoot(info BuildInfo, pubKey publicKeyFunc, procRoot string) *Server {
	m := newMCPServer(info, procRoot)
	httpMCP := mcpserver.NewStreamableHTTPServer(m, mcpserver.WithEndpointPath(MCPEndpointPath))
	return &Server{info: info, httpMCP: httpMCP, pubKey: pubKey}
}

// newMCPServer builds the MCP server and registers its resources/tools.
// Factored out so the integration test can drive it directly through an
// in-process MCP client (no HTTP transport, no auth gate) and exercise
// the real protocol path.
func newMCPServer(info BuildInfo, procRoot string) *mcpserver.MCPServer {
	m := mcpserver.NewMCPServer("powerlab-mcp", info.Version)
	registerSystemMetrics(m, procRoot)
	return m
}

// Handler returns the HTTP handler serving the open control endpoints
// and the read-tier-gated MCP transport.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/version", s.handleVersion)

	// Read-tier gate: jwt.HTTPJWT skips loopback (trusted local agent)
	// and requires a valid Bearer token from the LAN, writing the
	// identity headers downstream consumers (audit, tool tiers) read.
	// StreamableHTTPServer.ServeHTTP dispatches by HTTP method (it does
	// not re-check the path), so mounting the gated handler at the exact
	// endpoint path is enough.
	//
	// TRUST BOUNDARY: jwt.HTTPJWT grants the loopback skip purely from
	// the TCP peer (r.RemoteAddr == 127.0.0.1/::1). powerlab-mcp is
	// designed to bind its own port directly (ADR-0034: standalone, NOT
	// behind the gateway) — so the peer is the real client. If it were
	// ever fronted by a same-host reverse proxy, every forwarded request
	// would arrive from 127.0.0.1 and inherit loopback trust — an auth
	// bypass. preventProxyLoopbackTrust closes that: a "loopback" request
	// that carries proxy headers is treated as remote, forcing the JWT
	// check. Fails safe (deny), and the genuine local-agent path (no
	// proxy headers) keeps its zero-config trust.
	gated := preventProxyLoopbackTrust(jwt.HTTPJWT(s.pubKey)(s.httpMCP))
	mux.Handle(MCPEndpointPath, gated)
	return mux
}

// proxyHeaders are the request headers a reverse proxy adds. Their
// presence on a "loopback" request means the real client is upstream of
// a proxy, not the local machine.
var proxyHeaders = []string{"X-Forwarded-For", "Forwarded", "X-Real-Ip"}

// preventProxyLoopbackTrust strips the loopback trust from a request
// that claims to come from loopback but carries reverse-proxy headers.
// It does so by rewriting RemoteAddr to a non-loopback sentinel before
// the JWT gate sees it, so the gate enforces the token instead of
// skipping. Requests with no proxy headers are passed through untouched
// (genuine local agents keep loopback trust).
func preventProxyLoopbackTrust(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isLoopbackAddr(r.RemoteAddr) && hasAnyProxyHeader(r) {
			// 192.0.2.1 is TEST-NET-1 — guaranteed non-loopback, so the
			// downstream gate will require a valid token.
			r.RemoteAddr = "192.0.2.1:0"
		}
		next.ServeHTTP(w, r)
	})
}

func hasAnyProxyHeader(r *http.Request) bool {
	for _, h := range proxyHeaders {
		if r.Header.Get(h) != "" {
			return true
		}
	}
	return false
}

// isLoopbackAddr mirrors jwt.HTTPJWT's host check: strip the port, then
// compare against the loopback literals.
func isLoopbackAddr(remoteAddr string) bool {
	host := remoteAddr
	if i := strings.LastIndexByte(host, ':'); i > 0 {
		host = host[:i]
	}
	return host == "127.0.0.1" || host == "::1"
}

// handleHealthz is the unauthenticated liveness probe.
func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

// handleVersion returns the ldflags-injected build identity as JSON.
func (s *Server) handleVersion(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.info)
}
