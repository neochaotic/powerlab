package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// debug_unhealthy_service — guides the agent through the
// observability Tool chain for diagnosing a service that the system
// health surface flagged as degraded. The recipe pivots on
// get_system_health (which Tool reports the service state) into
// journal_search (correlated logs in the same window) and
// check_disk_free (the most common host-level cause). The journal
// step is per-service when a name is supplied; system-wide otherwise.

const debugUnhealthyServicePromptName = "debug_unhealthy_service"

func registerDebugUnhealthyServicePrompt(s *mcp.Server) {
	s.AddPrompt(
		&mcp.Prompt{
			Name:        debugUnhealthyServicePromptName,
			Description: "Walks the agent through diagnosing an unhealthy PowerLab service: get_system_health for the current snapshot → journal_search for the service's recent stderr → check_disk_free to rule out the most common host-level cause. Optional service narrows journal filters to one PowerLab unit (gateway, core, app-management, user); since_minutes scopes the time window. Returns ordered Tool-call recipe with concrete parameters.",
			Arguments: []*mcp.PromptArgument{
				{
					Name:        "service",
					Description: "Optional. PowerLab service name (e.g., 'gateway', 'core', 'app-management', 'user'). Narrows the journal_search unit filter. Empty yields a generic playbook that queries all units.",
					Required:    false,
				},
				{
					Name:        "since_minutes",
					Description: "Optional. Time window in minutes for journal_search (e.g., '15' for the last 15 minutes). Empty defaults to 30.",
					Required:    false,
				},
			},
		},
		func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			service := ""
			sinceMin := ""
			if req.Params.Arguments != nil {
				service = strings.TrimSpace(req.Params.Arguments["service"])
				sinceMin = strings.TrimSpace(req.Params.Arguments["since_minutes"])
			}
			return buildDebugUnhealthyServiceResult(service, sinceMin), nil
		},
	)
}

func buildDebugUnhealthyServiceResult(service, sinceMin string) *mcp.GetPromptResult {
	if sinceMin == "" {
		sinceMin = "30"
	}

	intro := "You are diagnosing an unhealthy PowerLab service. Three Tools, in this order, reproduce the operator's mental model on a live box: aggregate health first, then service logs, then host capacity. Run them; then synthesize a root cause + remediation."
	if service != "" {
		intro = fmt.Sprintf("You are diagnosing why the PowerLab %s service is unhealthy. Three Tools, in this order, reproduce the operator's mental model on a live box: aggregate health first, then %s logs, then host capacity. Run them; then synthesize a root cause + remediation.", service, service)
	}

	step1 := "1. **get_system_health** — single snapshot of memory + disk + services + updates. Identify which sub-status is `warn` or `critical`; that anchors the rest of the investigation."

	step2 := "2. **journal_search** — pull recent stderr for the PowerLab units. Filter: since_minutes=" + sinceMin + ", level≤\"warn\". Look for repeated error patterns, panics, or restart cycles."
	if service != "" {
		step2 = fmt.Sprintf("2. **journal_search** — pull recent stderr for %s. Filter: unit=%q, since_minutes=%s, level≤\"warn\". Look for repeated error patterns, panics, or restart cycles.", service, service, sinceMin)
	}

	step3 := "3. **check_disk_free** — check the data + log mount points (`/var/lib/docker`, `/var/log`, `/`). Free space ≤ 5% turns half of \"why is X broken\" into \"the disk is full\". Cheap to rule out first."

	synthesis := "4. **Synthesize**: (a) what is `get_system_health` saying? (b) what error pattern does journal_search show? (c) is disk capacity OK? (d) propose one concrete remediation — restart, rotate logs, free disk, or escalate."

	playbook := strings.Join([]string{step1, step2, step3, synthesis}, "\n\n")

	closing := "Do NOT propose a remediation before steps 1-3. The order matters: a critical disk shortage looks like a service bug until you check capacity."

	return &mcp.GetPromptResult{
		Description: "Unhealthy-service diagnostic playbook chaining get_system_health → journal_search → check_disk_free.",
		Messages: []*mcp.PromptMessage{
			{Role: "user", Content: &mcp.TextContent{Text: intro}},
			{Role: "user", Content: &mcp.TextContent{Text: "## Tool-call recipe\n\n" + playbook}},
			{Role: "user", Content: &mcp.TextContent{Text: closing}},
		},
	}
}
