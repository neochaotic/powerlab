package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/neochaotic/powerlab/backend/powerlab-mcp/coreproxy"
)

// ADR-0046 batch 2 — reversible side-effect tools. restart_app cycles
// every container of one installed app. Effect is real (the containers
// briefly go down then come back up) but bounded: no data loss, no
// state mutation, the app reaches the same end-state it was in
// before the call. That makes it safe to ship before destructive
// tools (install_app, prune_orphans) without the EnableDestructiveTools
// gate or the panel-side approval flow.
//
// Per ADR-0046 §1: Description leads with the side-effect class so
// Claude Desktop / Code surface the SIDE EFFECT marker to the user
// in the "Claude wants to use the tool X" prompt UX.

// RestartAppInput is the typed input. The id is the compose app id —
// the same one apps://list publishes.
type RestartAppInput struct {
	ID string `json:"id" jsonschema:"Compose app id — exactly as it appears in apps://list (e.g. 'plex' or 'jellyfin')"`
}

// RestartAppOutput is what the agent reads after a successful
// restart. Carries the id + a short status the agent can echo back
// to the user.
type RestartAppOutput struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// registerRestartApp wires the restart_app tool. proxy is the
// multi-service coreproxy (nil-tolerant — falls back to the
// canonical apps_unavailable error shape).
//
// Architecture note (audit correlation): MCP calls app-management
// directly via coreproxy, not through the gateway. The gateway's
// audit middleware (ADR-0033) only records requests that go through
// the gateway, so an MCP-driven restart does NOT show up in the
// gateway-written audit.jsonl that audit:// reads. Full audit
// correlation lands with the MCP audit-recorder dogfood
// (ADR-0034 deferred item) — at that point each tool call gets its
// own audit record on the MCP side, and the correlation id can join
// across MCP + upstream. Until then the agent's trail is in
// powerlab-mcp's journald output (journal://mcp).
func registerRestartApp(s *mcp.Server, proxy *coreproxy.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "restart_app",
		Description: "Restart every container of one installed PowerLab app (start/stop cycle). SIDE EFFECT — containers briefly go down then come back up; no data loss; app ends in the same state it was in before the call. Use apps://list to discover valid ids.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in RestartAppInput) (*mcp.CallToolResult, RestartAppOutput, error) {
		id := strings.TrimSpace(in.ID)
		if id == "" {
			return nil, RestartAppOutput{}, errors.New("id is required (see apps://list)")
		}
		// Defensive: reject ids containing slashes / dots so a typo
		// or attempted path traversal can't reach an unexpected
		// app-management endpoint. The codegen.ComposeAppID type
		// is a plain string but our HTTP path is templated.
		if strings.ContainsAny(id, "/\\.?#") {
			return nil, RestartAppOutput{}, fmt.Errorf("invalid app id %q (must match the apps://list manifest)", id)
		}
		if proxy == nil {
			return nil, RestartAppOutput{}, errors.New("apps_unavailable: coreproxy not configured")
		}
		_, token, _ := tokenFromToolRequest(req)
		// app-management expects a JSON-encoded enum string body:
		// '"restart"'. We marshal once at registration to avoid
		// re-allocating per call.
		body := []byte(`"restart"`)
		path := "/v2/app_management/compose/" + url.PathEscape(id) + "/status"
		if _, err := proxy.RequestFrom(ctx, http.MethodPut, coreproxy.ServiceApps, path, token, body, "application/json"); err != nil {
			// Surface the structured error to the agent — same shape
			// the apps:// resources use on degraded reads. The tool's
			// error path is the same as the resource family's so an
			// agent that handles core_unavailable also handles tool
			// failures uniformly.
			payload := coreproxy.AsErrorPayload(err)
			return &mcp.CallToolResult{
					IsError: true,
					Content: []mcp.Content{&mcp.TextContent{Text: string(payload)}},
				}, RestartAppOutput{},
				nil
		}
		return nil, RestartAppOutput{ID: id, Status: "restart requested"}, nil
	})
}

