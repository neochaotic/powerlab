// Package server wires powerlab-mcp's HTTP surface: the control
// endpoints (/healthz, /version) the systemd unit and install smoke
// poll, plus the MCP transport (Streamable HTTP, the 2025-06-18 spec
// transport) mounted at /mcp.
//
// This is the Foundation skeleton: the MCP server is created and
// mounted but registers no resources or tools yet — those land in the
// follow-up PRs (system://, journal://, audit://) once the auth-tier
// middleware is in place. Keeping the skeleton dependency-light (only
// the MCP SDK) is deliberate; the foundation middleware chain
// (correlation-id, recover, audit) is wired when the first real
// resource needs it.
package server

import (
	"encoding/json"
	"net/http"

	mcpserver "github.com/mark3labs/mcp-go/server"

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

// Server holds the constructed MCP transport and the build identity.
// Build it with New, mount it with Handler.
type Server struct {
	info    BuildInfo
	httpMCP *mcpserver.StreamableHTTPServer
}

// New constructs the MCP server and its Streamable-HTTP transport. It
// does not bind a listener — Handler returns the mux for the caller to
// serve, which also keeps the whole surface testable via httptest.
//
// The skeleton registers no resources or tools yet; the follow-up PRs
// will reach the underlying MCPServer to register them.
func New(cfg config.Config, info BuildInfo) (*Server, error) {
	m := mcpserver.NewMCPServer("powerlab-mcp", info.Version)
	httpMCP := mcpserver.NewStreamableHTTPServer(m, mcpserver.WithEndpointPath(MCPEndpointPath))
	return &Server{info: info, httpMCP: httpMCP}, nil
}

// Handler returns the HTTP handler serving the control endpoints and
// the mounted MCP transport.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/version", s.handleVersion)
	// StreamableHTTPServer.ServeHTTP dispatches by HTTP method (it does
	// not re-check the path), so mounting it at the exact endpoint path
	// is enough.
	mux.Handle(MCPEndpointPath, s.httpMCP)
	return mux
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
