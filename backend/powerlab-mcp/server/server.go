// Package server wires powerlab-mcp's HTTP surface: the control
// endpoints (/healthz, /version) the systemd unit and install smoke
// poll, plus the MCP transport (Streamable HTTP, the 2025-06-18 spec
// transport) mounted at /mcp.
//
// The MCP endpoint is gated two-tier (ADR-0034): reachable freely from
// loopback (the trusted local agent / dogfood case), but a LAN caller
// must present a valid PowerLab user-service JWT. The control endpoints
// stay open — a health probe that needs a token is not a health probe.
//
// There is intentionally NO third "admin" tier: jwt.Claims carries no
// role field today and every PowerLab user is hardcoded admin at
// registration. Pretending a role gate exists would be doc-theatre
// without enforcement. Real RBAC is a separate ADR + backlog item.
//
// Tools added later inherit the same two-tier gate — any caller that
// passed the JWT check can call any tool until RBAC lands.
package server

import (
	"crypto/ecdsa"
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/neochaotic/powerlab/backend/common/external"
	"github.com/neochaotic/powerlab/backend/common/utils/jwt"
	"github.com/neochaotic/powerlab/backend/powerlab-mcp/coreproxy"
	"github.com/neochaotic/powerlab/backend/powerlab-mcp/config"
	"github.com/neochaotic/powerlab/backend/powerlab-mcp/journal"
)

// MCPEndpointPath is the single HTTP path the MCP Streamable-HTTP
// transport is served on. Exported so tests (and later the CLI client)
// can address it without hardcoding the string twice.
const MCPEndpointPath = "/mcp"

// maxMCPRequestBytes caps the /mcp request body. MCP JSON-RPC messages
// are tiny (a few KB at most); the transport otherwise reads the whole
// body into memory, so an unbounded POST is an OOM/DoS vector — sharper
// because loopback callers are unauthenticated. 1 MiB is generous
// headroom while bounding the blast radius.
const maxMCPRequestBytes = 1 << 20

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
	httpMCP *mcp.StreamableHTTPHandler
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
	pubKey := func() (*ecdsa.PublicKey, error) { return external.GetPublicKey(cfg.RuntimePath) }
	return newServer(info, pubKey, resourcesConfig{
		auditPath:        filepath.Join(cfg.AuditDir, "audit.jsonl"),
		procRoot:         "/proc",
		openAPIDir:       cfg.OpenAPIDir,
		systemdSystemDir: cfg.SystemdSystemDir,
		coreClient:       coreproxy.NewClient(cfg.RuntimePath, nil),
	}), nil
}

// resourcesConfig bags together the runtime paths and clients the
// resource layer reads from. Bundling them avoids a 6-arg
// `newMCPServer` signature as the resource surface grows; tests
// construct it directly with t.TempDir fixtures and (where they need
// to) a coreproxy.Client backed by an httptest server.
type resourcesConfig struct {
	auditPath        string
	procRoot         string
	openAPIDir       string
	systemdSystemDir string
	coreClient       *coreproxy.Client
}

// newServer is the dependency-injected constructor: tests pass a
// pubKeyFunc backed by a known test key so the gate's JWT validation is
// exercised for real (no mock), without standing up a user-service.
// resourcesConfig holds the per-deploy paths; a zero value is fine for
// the tests that only exercise the auth/transport layers.
func newServer(info BuildInfo, pubKey publicKeyFunc, rc resourcesConfig) *Server {
	if rc.procRoot == "" {
		rc.procRoot = "/proc"
	}
	m := newMCPServer(info, rc, journal.Exec)
	// The same server instance handles every request; the getServer
	// callback hands it to each incoming MCP session.
	httpMCP := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return m }, nil)
	return &Server{info: info, httpMCP: httpMCP, pubKey: pubKey}
}

// newMCPServer builds the MCP server and registers its resources. rc
// bundles the read paths; journalRun is the runtime journalctl wrapper
// (prod = journal.Exec; tests pass a fixture-backed Runner). Factored
// out so the integration test can drive it directly through an
// in-process MCP client (no HTTP transport, no auth gate) and exercise
// the real protocol path.
func newMCPServer(info BuildInfo, rc resourcesConfig, journalRun journal.Runner) *mcp.Server {
	m := mcp.NewServer(&mcp.Implementation{Name: "powerlab-mcp", Version: info.Version}, nil)
	registerSystemMetrics(m, rc.procRoot)
	registerSystemUtilization(m, rc.coreClient)
	registerJournal(m, journalRun)
	registerJournalUnits(m, rc.systemdSystemDir)
	registerAudit(m, rc.auditPath)
	registerDocs(m, rc.openAPIDir)
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
	// The MCP StreamableHTTPHandler is a plain http.Handler, mounted at
	// the endpoint path behind the gate.
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
	// limitBody is outermost so the body cap applies before anything
	// reads the request — even on the auth-rejected path.
	gated := limitBody(preventProxyLoopbackTrust(jwt.HTTPJWT(s.pubKey)(s.httpMCP)), maxMCPRequestBytes)
	mux.Handle(MCPEndpointPath, gated)
	return mux
}

// limitBody caps the request body at max bytes via http.MaxBytesReader,
// so the MCP transport (which reads the whole body into memory) can't be
// driven to OOM by an oversized POST.
func limitBody(next http.Handler, max int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, max)
		}
		next.ServeHTTP(w, r)
	})
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

// isLoopbackAddr reports whether remoteAddr's host is loopback. It uses
// net.SplitHostPort so IPv6 forms like "[::1]:9090" are handled
// correctly (a manual ':' split would leave the brackets and never
// match "::1").
func isLoopbackAddr(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr // no port present — treat the whole string as the host
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
