package server

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// P2.8 from the 2026-05-31 MCP-only chat-mode retro: agents spent
// turns trying gated tools (install_app, journal://system/auth)
// without knowing which tiers the operator had opted into. The Tool
// here reports the active tier state in one structured call so the
// agent can plan its approach without trial-and-error against the
// gate.
//
// Side-effect class: READ ONLY. Returns the registration state
// derived from the resourcesConfig the server was built with. Same
// information the agent could derive from tools/list + resources/list
// patterns, but explicit + ergonomic for chat-mode reasoning.

// ListCapabilitiesOutput names the two opt-in tiers PowerLab gates
// with operator flags today: destructive write tools (install_app /
// uninstall_app / restart_app) and the sensitive sysadmin resources
// (journal://system/auth + failures). The Summary is a one-line
// human-readable statement of both, so an agent that doesn't parse
// the structured fields still surfaces the right state.
type ListCapabilitiesOutput struct {
	DestructiveToolsEnabled bool     `json:"destructive_tools_enabled"`
	DestructiveTools        []string `json:"destructive_tools"`
	SensitiveTierEnabled    bool     `json:"sensitive_tier_enabled"`
	SensitiveResources      []string `json:"sensitive_resources"`
	Summary                 string   `json:"summary"`
}

type listCapabilitiesInput struct{}

// destructiveToolNames + sensitiveResourceURIs are hardcoded
// rather than introspected from the live MCP server because:
//   (a) the tier names are operator-facing knobs (mcp.conf), so
//       the names must stay stable across server restarts and
//       deployments — introspection would surface refactor noise
//       to the agent.
//   (b) the agent's reasoning is "which tier am I in?", not
//       "which exact symbols are registered" — keeping the names
//       at the human-tier level matches the intent.
var (
	destructiveToolNames   = []string{"install_app", "uninstall_app", "restart_app"}
	sensitiveResourceURIs  = []string{"journal://system/auth", "journal://system/failures"}
)

func registerListCapabilities(s *mcp.Server, destructiveEnabled, sensitiveEnabled bool) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_capabilities",
		Description: "READ ONLY — report the operator-opt-in tiers active on this PowerLab MCP server. Two tiers are gated by `mcp.conf`: destructive write tools (install_app, uninstall_app, restart_app) and the sensitive sysadmin resources (journal://system/auth + journal://system/failures). Call this BEFORE attempting a gated capability — saves a trial-and-error round-trip when the tier is off.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ listCapabilitiesInput) (*mcp.CallToolResult, ListCapabilitiesOutput, error) {
		out := ListCapabilitiesOutput{
			DestructiveToolsEnabled: destructiveEnabled,
			SensitiveTierEnabled:    sensitiveEnabled,
			DestructiveTools:        []string{},
			SensitiveResources:      []string{},
		}
		if destructiveEnabled {
			out.DestructiveTools = append(out.DestructiveTools, destructiveToolNames...)
		}
		if sensitiveEnabled {
			out.SensitiveResources = append(out.SensitiveResources, sensitiveResourceURIs...)
		}
		out.Summary = capabilitiesSummary(destructiveEnabled, sensitiveEnabled)
		return nil, out, nil
	})
}

func capabilitiesSummary(destructive, sensitive bool) string {
	parts := []string{}
	if destructive {
		parts = append(parts, "destructive tools enabled (install_app / uninstall_app / restart_app)")
	} else {
		parts = append(parts, "destructive tools disabled (no install / uninstall / restart)")
	}
	if sensitive {
		parts = append(parts, "sensitive journal tier enabled (auth + failures readable)")
	} else {
		parts = append(parts, "sensitive journal tier disabled")
	}
	return strings.Join(parts, "; ")
}
