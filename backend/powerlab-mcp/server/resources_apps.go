package server

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/neochaotic/powerlab/backend/powerlab-mcp/coreproxy"
)

// ADR-0045 — apps://* and docker://logs/{id} are thin HTTP proxies to
// app-management. Storage-agnostic by construction: when PowerLab
// migrates from SQLite to PostgreSQL (or any other backend), MCP
// requires no changes because the HTTP /v2/ contract is the
// abstraction, not the storage layer.

const (
	// apps://* URIs. Schema first (agents read it for discovery),
	// then concrete + templated resources.
	appsSchemaURI = "apps://schema"
	appsListURI   = "apps://list"

	// appsStateTemplate is the RFC 6570 form for per-app state reads.
	// The agent passes the compose app id as {id}; the inner sub-path
	// resources (/containers, /health, /stats, /disk) ride the same
	// template since they're all GETs against /v2/app_management/compose/{id}/...
	appsStateTemplate          = "apps://state/{id}"
	appsStateContainersTmpl    = "apps://state/{id}/containers"
	appsStateHealthTmpl        = "apps://state/{id}/health"
	appsStateStatsTmpl         = "apps://state/{id}/stats"
	appsStateDiskTmpl          = "apps://state/{id}/disk"
	appsStatePrefix            = "apps://state/"
	appsStateContainersSuffix  = "/containers"
	appsStateHealthSuffix      = "/health"
	appsStateStatsSuffix       = "/stats"
	appsStateDiskSuffix        = "/disk"

	// docker://logs/{id} — proxied through app-management's
	// ComposeAppLogs endpoint (the same path the panel reads), so
	// MCP never needs Docker socket access (ADR-0045 win #2).
	dockerLogsTemplate = "docker://logs/{id}"
	dockerLogsPrefix   = "docker://logs/"
)

// appsSchemaDoc is the self-describing index the agent reads once to
// learn which apps:// resources exist + what each returns. Updating
// any handler MUST update this doc in lockstep.
const appsSchemaDoc = `{
  "description": "PowerLab installed-apps surface — installed compose apps, their lifecycle state, containers, health, stats, and disk usage. Storage-agnostic: thin HTTP proxies to app-management's /v2/ API (ADR-0045).",
  "resources": {
    "apps://schema": "this document",
    "apps://list": "manifest of installed apps — id, name, status, version (proxied from /v2/app_management/compose)",
    "apps://state/{id}": "compose source + status for one app",
    "apps://state/{id}/containers": "live containers for one app",
    "apps://state/{id}/health": "aggregate health for one app",
    "apps://state/{id}/stats": "per-container CPU/RAM/IO for one app",
    "apps://state/{id}/disk": "per-app disk footprint",
    "docker://logs/{id}": "container logs proxied through app-management's ComposeAppLogs — MCP does NOT have Docker socket access; the surface goes through the same path the panel reads",
    "docker://containers": "all containers on the host Docker daemon (PowerLab + non-PowerLab) — name, image, state, ports, created_at, labels (#630)",
    "docker://images": "all local Docker images — id, tags[], size, created_at (#630)",
    "docker://networks": "all Docker networks — name, driver, scope, IPAM, attached_containers[] (#630)",
    "docker://volumes": "all Docker volumes — name, driver, mountpoint, size (bytes; -1 when daemon couldn't compute), in_use_by[]{id,name} (containers mounting the volume) (#630, #645)",
    "docker://system": "Docker daemon info + 'docker system df' snapshot — version, containers/images count, disk_usage{containers,images,volumes,build_cache} (#630)"
  },
  "proxy_error_shape": {
    "error": "apps_unavailable | apps_status_NNN — pattern-match this and pivot to audit:// + journal://powerlab-app-management",
    "detail": "human-readable cause (app-management .url file missing, transport failure, upstream non-2xx)",
    "fallback": "always mentions audit + journal — the independent resources that don't need any upstream"
  },
  "future_proof": "ADR-0045 locks the architectural promise: when PowerLab migrates from SQLite to PostgreSQL, MCP requires no changes — storage is an implementation detail app-management owns."
}`

// registerApps wires the apps:// surface. proxy may be nil (early-boot
// tests) — the resources serve the canonical apps_unavailable payload
// in that case rather than panicking.
func registerApps(s *mcp.Server, proxy *coreproxy.Client) {
	// Schema first — agents call resources/list then read schemas
	// before driving concrete reads.
	s.AddResource(&mcp.Resource{
		URI:         appsSchemaURI,
		Name:        "Apps surface schema",
		Description: "Field reference for apps://* resources + the docker://logs proxy. Self-describing.",
		MIMEType:    "application/json",
	}, func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{textJSON(appsSchemaURI, appsSchemaDoc)}}, nil
	})

	// apps://list — concrete URI (no template).
	s.AddResource(&mcp.Resource{
		URI:         appsListURI,
		Name:        "Installed apps",
		Description: "Manifest of installed compose apps — id, name, status, version. Proxied from app-management's /v2/app_management/compose.",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		_, token, _ := tokenFromRequest(req)
		return proxiedFromApps(ctx, proxy, appsListURI, "/v2/app_management/compose", token)
	})

	// apps://state — registered as five distinct templates rather
	// than one parametric pattern because the MCP SDK's URI matcher
	// doesn't let `{id}` straddle a `/`. One template per sub-path
	// keeps the SDK's resources/list output self-documenting and
	// lets agents discover every shape from the schema.
	stateRoutes := []struct {
		template    string
		name        string
		description string
		upstreamFmt string // takes one %s for url.PathEscape(id)
	}{
		{
			appsStateTemplate,
			"App state",
			"Compose source + status for one installed app (proxy /v2/app_management/compose/{id}). Pair with apps://list to discover valid ids.",
			"/v2/app_management/compose/%s",
		},
		{
			appsStateContainersTmpl,
			"App live containers",
			"Live containers for one app (proxy /v2/app_management/compose/{id}/containers).",
			"/v2/app_management/compose/%s/containers",
		},
		{
			appsStateHealthTmpl,
			"App health",
			"Aggregate health for one app (proxy /v2/app_management/compose/{id}/health).",
			"/v2/app_management/compose/%s/health",
		},
		{
			appsStateStatsTmpl,
			"App stats",
			"Per-container CPU/RAM/IO for one app (proxy /v2/app_management/compose/{id}/stats).",
			"/v2/app_management/compose/%s/stats",
		},
		{
			appsStateDiskTmpl,
			"App disk",
			"Disk footprint for one app (proxy /v2/app_management/compose/{id}/disk).",
			"/v2/app_management/compose/%s/disk",
		},
	}
	for _, route := range stateRoutes {
		// Capture the upstream format in the closure — `route` is
		// reused across loop iterations.
		upstreamFmt := route.upstreamFmt
		s.AddResourceTemplate(&mcp.ResourceTemplate{
			URITemplate: route.template,
			Name:        route.name,
			Description: route.description,
			MIMEType:    "application/json",
		}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			id := extractAppID(req.Params.URI)
			if id == "" {
				return nil, fmt.Errorf("apps://state/{id} requires an app id (see apps://list)")
			}
			_, token, _ := tokenFromRequest(req)
			return proxiedFromApps(ctx, proxy, req.Params.URI, fmt.Sprintf(upstreamFmt, url.PathEscape(id)), token)
		})
	}

	// docker://logs/{id} — same architectural pattern, distinct
	// scheme so the agent's grouping in resources/list reflects
	// "logs are a docker concept, not an apps state field."
	s.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: dockerLogsTemplate,
		Name:        "Container logs",
		Description: "Container logs for one PowerLab-managed app. Proxied through app-management's ComposeAppLogs endpoint — MCP does NOT access the Docker socket directly. Use the compose app id (matches apps://list); the endpoint resolves to the right container internally.",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		id := strings.TrimPrefix(req.Params.URI, dockerLogsPrefix)
		if id == "" || strings.Contains(id, "/") {
			return nil, fmt.Errorf("docker://logs/{id} requires a single compose app id")
		}
		_, token, _ := tokenFromRequest(req)
		return proxiedFromApps(ctx, proxy, req.Params.URI, "/v2/app_management/compose/"+url.PathEscape(id)+"/logs", token)
	})
}

// proxiedFromApps runs a GetFrom(ServiceApps) and wraps the body
// into an MCP ReadResourceResult, serving the canonical
// apps_unavailable payload when the proxy errs or is nil. Mirrors
// registerProxiedSystem in shape — both share the same degradation
// contract from ADR-0044 + ADR-0045.
func proxiedFromApps(ctx context.Context, proxy *coreproxy.Client, uri, path, token string) (*mcp.ReadResourceResult, error) {
	if proxy == nil {
		payload := coreproxy.AsErrorPayload(&coreproxy.Error{
			Code:   "apps_unavailable",
			Detail: "coreproxy not configured — server built without a coreproxy client",
		})
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{textJSON(uri, string(payload))}}, nil
	}
	body, err := proxy.GetFrom(ctx, coreproxy.ServiceApps, path, token)
	if err != nil {
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{textJSON(uri, string(coreproxy.AsErrorPayload(err)))}}, nil
	}
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{textJSON(uri, string(body))}}, nil
}

// extractAppID returns the id segment from apps://state/{id}[/sub].
// PowerLab compose app ids don't contain slashes (the upstream
// codegen.ComposeAppID is a plain string), so the id is everything
// between the prefix and the next `/` (or end of string).
func extractAppID(raw string) string {
	if !strings.HasPrefix(raw, appsStatePrefix) {
		return ""
	}
	rest := strings.TrimPrefix(raw, appsStatePrefix)
	if i := strings.IndexAny(rest, "?#"); i >= 0 {
		rest = rest[:i]
	}
	if slash := strings.Index(rest, "/"); slash >= 0 {
		return rest[:slash]
	}
	return rest
}
