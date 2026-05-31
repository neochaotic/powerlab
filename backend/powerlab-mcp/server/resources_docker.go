package server

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/neochaotic/powerlab/backend/powerlab-mcp/coreproxy"
)

// Raw Docker visibility surface (#630). The five resources expose
// daemon-wide state that goes beyond PowerLab-managed compose apps:
// non-PowerLab containers the operator started with `docker run`,
// dangling images, third-party networks, orphan volumes, daemon
// version and disk-usage snapshot. Every one is READ ONLY by design
// — no exec, no run, no shell-in-container; that surface is out of
// scope per the issue (security: an agent with exec is rootkit-grade).
//
// Architecturally identical to apps://list: thin HTTP proxy through
// app-management. ADR-0044 establishes the proxy pattern; ADR-0045
// locks the rule that MCP NEVER touches the Docker socket directly —
// app-management is the single PowerLab service that talks to the
// daemon, and MCP calls its HTTP API. This file adds five more
// proxied endpoints; the degradation contract (apps_unavailable +
// audit/journal fallback hint) is inherited verbatim from
// proxiedFromApps.
const (
	dockerContainersURI = "docker://containers"
	dockerImagesURI     = "docker://images"
	dockerNetworksURI   = "docker://networks"
	dockerVolumesURI    = "docker://volumes"
	dockerSystemURI     = "docker://system"
)

// dockerRawVisibilityRoute is one row of the raw-visibility resource
// table — distinct values so the registrar is data-driven (adding a
// sixth surface = one row, not a copy-paste of an AddResource block).
type dockerRawVisibilityRoute struct {
	uri          string
	name         string
	description  string
	upstreamPath string
}

// dockerRawVisibilityRoutes is the source of truth for the five
// resources. Both registerDockerRawVisibility and the docker schema
// doc consume it so registration drift (resource exists in code but
// not in the schema) is impossible.
var dockerRawVisibilityRoutes = []dockerRawVisibilityRoute{
	{
		uri:          dockerContainersURI,
		name:         "Docker containers (raw)",
		description:  "All containers on the host Docker daemon — PowerLab-managed AND non-PowerLab (operator-run docker containers, third-party tooling). Equivalent to `docker ps -a`. Fields: name, image, state, ports, created_at, labels. Proxied through app-management; MCP NEVER touches the Docker socket (ADR-0045).",
		upstreamPath: "/v2/app_management/docker/containers",
	},
	{
		uri:          dockerImagesURI,
		name:         "Docker images (raw)",
		description:  "All local Docker images. Fields: id, tags[], size, created_at. Proxied through app-management; MCP NEVER touches the Docker socket (ADR-0045).",
		upstreamPath: "/v2/app_management/docker/images",
	},
	{
		uri:          dockerNetworksURI,
		name:         "Docker networks (raw)",
		description:  "All Docker networks. Fields: name, driver, scope, IPAM, attached_containers[]. Proxied through app-management; MCP NEVER touches the Docker socket (ADR-0045).",
		upstreamPath: "/v2/app_management/docker/networks",
	},
	{
		uri:          dockerVolumesURI,
		name:         "Docker volumes (raw)",
		description:  "All Docker volumes. Fields: name, driver, mountpoint, size, in_use_by[]. Proxied through app-management; MCP NEVER touches the Docker socket (ADR-0045).",
		upstreamPath: "/v2/app_management/docker/volumes",
	},
	{
		uri:          dockerSystemURI,
		name:         "Docker system info + disk usage",
		description:  "Daemon info (version, containers/images count) + `docker system df` snapshot (disk usage by category: containers, images, volumes, build_cache). Proxied through app-management; MCP NEVER touches the Docker socket (ADR-0045).",
		upstreamPath: "/v2/app_management/docker/system",
	},
}

// registerDockerRawVisibility wires the five docker://* raw-visibility
// resources. proxy may be nil (early-boot tests) — the proxiedFromApps
// helper serves the canonical apps_unavailable payload in that case.
//
// All five are concrete URIs (no template parameters); raw daemon
// queries don't take a per-app id, so resources/list advertises each
// one directly. Listed at the daemon level alongside docker://logs/{id},
// which stays template-shaped because it's per-app.
func registerDockerRawVisibility(s *mcp.Server, proxy *coreproxy.Client) {
	for _, route := range dockerRawVisibilityRoutes {
		// Loop-local copy: the closure captures the iteration variable;
		// without this every handler would race on the last route value.
		r := route
		s.AddResource(&mcp.Resource{
			URI:         r.uri,
			Name:        r.name,
			Description: r.description,
			MIMEType:    "application/json",
		}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			// JWT forwarding (ADR-0047). When the agent's request
			// carries a Bearer token, propagate it to app-management
			// so the upstream's audit middleware records the right
			// user instead of "loopback".
			_, token, _ := tokenFromRequest(req)
			return proxiedFromApps(ctx, proxy, r.uri, r.upstreamPath, token)
		})
	}
}
