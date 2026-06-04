package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// troubleshoot_install_failure — guides the agent through the
// observability Tool chain for diagnosing a failed install_app. The
// recipe is encoded in the prompt because the agent otherwise has to
// derive it from per-Tool descriptions, and the ordering matters:
// audit captures intent + outcome, journal captures the underlying
// docker/compose stderr, health surfaces whether the failure is
// systemic (disk full, memory pressure) vs app-specific (bad image).

const troubleshootInstallFailurePromptName = "troubleshoot_install_failure"

func registerTroubleshootInstallFailurePrompt(s *mcp.Server) {
	s.AddPrompt(
		&mcp.Prompt{
			Name:        troubleshootInstallFailurePromptName,
			Description: "Walks the agent through diagnosing a failed install_app: audit_query for the failed attempt → journal_search for the correlated install/app-management logs → get_system_health to rule out host-level causes. Optional app_id narrows audit + journal queries; since_minutes scopes the time window. Returns ordered Tool-call recipe with concrete parameters the agent should substitute.",
			Arguments: []*mcp.PromptArgument{
				{
					Name:        "app_id",
					Description: "Optional. App identifier whose install failed (e.g., 'jellyfin'). Narrows audit_query + journal_search filters. Empty yields a generic playbook the agent can re-parameterize once it identifies the app.",
					Required:    false,
				},
				{
					Name:        "since_minutes",
					Description: "Optional. Time window in minutes to scope audit + journal queries (e.g., '30' for the last 30 minutes). Empty defaults to 60.",
					Required:    false,
				},
			},
		},
		func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			appID := ""
			sinceMin := ""
			if req.Params.Arguments != nil {
				appID = strings.TrimSpace(req.Params.Arguments["app_id"])
				sinceMin = strings.TrimSpace(req.Params.Arguments["since_minutes"])
			}
			return buildTroubleshootInstallFailureResult(appID, sinceMin), nil
		},
	)
}

func buildTroubleshootInstallFailureResult(appID, sinceMin string) *mcp.GetPromptResult {
	if sinceMin == "" {
		sinceMin = "60"
	}

	intro := "You are diagnosing a failed PowerLab install_app. Three observability Tools, called in order, surface the failure cause in ≥90% of cases. Run them; then synthesize a root-cause + remediation."
	if appID != "" {
		intro = fmt.Sprintf("You are diagnosing why install_app for %q failed on this PowerLab host. Three observability Tools, called in order, surface the cause in ≥90%% of cases. Run them; then synthesize a root-cause + remediation.", appID)
	}

	// Tool-call recipe. Concrete parameters when we have them;
	// placeholder phrasing when we don't. Avoid generating an
	// app_id="" call — that would be a wrong invocation.
	step1 := "1. **audit_query** — find the failed install_app entry. Filter: action=\"install_app\", outcome=\"failure\", since_minutes=" + sinceMin + "."
	if appID != "" {
		step1 = fmt.Sprintf("1. **audit_query** — find the failed install_app entry. Filter: action=\"install_app\", outcome=\"failure\", app_id=%q, since_minutes=%s.", appID, sinceMin)
	}

	step2 := "2. **journal_search** — pull the correlated app-management + gateway logs spanning the same window. Filter: unit IN (\"app-management\", \"gateway\"), since_minutes=" + sinceMin + ", level≤\"warn\". Cross-reference the timestamp from step 1."
	if appID != "" {
		step2 = fmt.Sprintf("2. **journal_search** — pull the correlated app-management + gateway logs spanning the same window. Filter: unit IN (\"app-management\", \"gateway\"), since_minutes=%s, level≤\"warn\", query=%q. Cross-reference the timestamp from step 1.", sinceMin, appID)
	}

	step3 := "3. **get_system_health** — rule out systemic causes (low disk, memory pressure, services degraded). If overall=\"warn\"|\"critical\", the install failure is likely a symptom, not the root."

	synthesis := "4. **Synthesize** in this order: (a) what error did audit + journal surface? (b) was the host healthy at the time? (c) is the cause app-specific (bad image, port collision, missing env) or systemic (disk full, network gone)? (d) propose one concrete remediation the operator can apply now."

	playbook := strings.Join([]string{step1, step2, step3, synthesis}, "\n\n")

	closing := "Do NOT speculate before running steps 1-3. PowerLab keeps the evidence — use it."

	return &mcp.GetPromptResult{
		Description: "Failed-install diagnostic playbook chaining audit_query → journal_search → get_system_health.",
		Messages: []*mcp.PromptMessage{
			{Role: "user", Content: &mcp.TextContent{Text: intro}},
			{Role: "user", Content: &mcp.TextContent{Text: "## Tool-call recipe\n\n" + playbook}},
			{Role: "user", Content: &mcp.TextContent{Text: closing}},
		},
	}
}
