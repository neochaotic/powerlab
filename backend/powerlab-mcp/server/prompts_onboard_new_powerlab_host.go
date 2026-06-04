package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// onboard_new_powerlab_host — guides an agent walking a fresh
// operator through their first install on a PowerLab box. The recipe
// hits capability discovery (what tier of tools is even allowed),
// catalog browsing biased by the operator's stated goal, and a
// pre-install health snapshot. Output is a guided tour, not an
// install command — the agent should ask for confirmation before
// crossing into destructive territory (which is gated server-side
// anyway via EnableDestructiveTools).

const onboardNewPowerlabHostPromptName = "onboard_new_powerlab_host"

func registerOnboardNewPowerlabHostPrompt(s *mcp.Server) {
	s.AddPrompt(
		&mcp.Prompt{
			Name:        onboardNewPowerlabHostPromptName,
			Description: "First-run walkthrough for a PowerLab host: list_capabilities (what's allowed) → browse_catalog (what apps exist, optionally biased by the operator's goal) → get_system_health (is the host ready). Optional goal hint shapes the catalog category emphasis; experience_level tunes how much PowerLab-domain context the agent should foreground. Returns an ordered Tool-call recipe + an interaction-style hint.",
			Arguments: []*mcp.PromptArgument{
				{
					Name:        "goal",
					Description: "Optional. Operator's free-text intent (e.g., 'host my media library', 'self-host a password manager', 'run a Git mirror'). Shapes which catalog categories the agent should emphasise. Empty yields a general-purpose walkthrough.",
					Required:    false,
				},
				{
					Name:        "experience_level",
					Description: "Optional. 'beginner', 'intermediate', or 'expert' — tunes verbosity of PowerLab-specific context (compose conventions, validator deny-list, security tiers). Empty defaults to 'intermediate'.",
					Required:    false,
				},
			},
		},
		func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			goal := ""
			level := ""
			if req.Params.Arguments != nil {
				goal = strings.TrimSpace(req.Params.Arguments["goal"])
				level = strings.TrimSpace(req.Params.Arguments["experience_level"])
			}
			return buildOnboardNewPowerlabHostResult(goal, level), nil
		},
	)
}

func buildOnboardNewPowerlabHostResult(goal, level string) *mcp.GetPromptResult {
	if level == "" {
		level = "intermediate"
	}

	intro := "You are onboarding an operator to a fresh PowerLab host. Three Tools establish the lay of the land before any install attempt: capability tier, available catalog, host readiness. Walk through them; surface results plainly; ask before crossing into anything destructive."
	if goal != "" {
		intro = fmt.Sprintf("You are onboarding an operator to a fresh PowerLab host. They told you their goal: %q. Three Tools establish the lay of the land before any install attempt: capability tier, available catalog (biased by their goal), host readiness. Walk through them; surface results plainly; ask before crossing into anything destructive.", goal)
	}

	step1 := "1. **list_capabilities** — does this host have destructive Tools enabled? Sensitive tier? Report back to the operator in one sentence: which categories of action you CAN take, which you CANNOT. This shapes everything that follows."

	step2 := "2. **browse_catalog** — survey what's installable. Note 3-5 apps that match the operator's goal; ignore the rest. Cite catalog ids so the operator can ask for details."
	if goal != "" {
		step2 = fmt.Sprintf("2. **browse_catalog** — survey what's installable. Filter against the goal (%q): note 3-5 apps that fit; ignore the rest. Cite catalog ids so the operator can ask for details.", goal)
	}

	step3 := "3. **get_system_health** — single pre-install snapshot. If the host is already degraded (low disk, services unhealthy), surface that before suggesting any install — the operator should land the host first."

	guidance := "## Interaction style\n\n- Experience level: " + level + ".\n- Beginner: explain PowerLab-specific concepts (catalog, compose conventions, security tiers) inline as they come up.\n- Intermediate: skip explanations of generic Linux/docker concepts; explain only PowerLab-specific behaviour (validator deny-list, sensitive tier gate).\n- Expert: dense, no inline glossary; assume familiarity with docker-compose, journald, security profiles."

	closing := "Do NOT propose install_app until steps 1-3 are complete and the operator has explicitly picked an app. Even with destructive Tools enabled, the operator should consent before you cross that line."

	playbook := strings.Join([]string{step1, step2, step3}, "\n\n")

	return &mcp.GetPromptResult{
		Description: "First-run walkthrough chaining list_capabilities → browse_catalog → get_system_health with an interaction-style hint.",
		Messages: []*mcp.PromptMessage{
			{Role: "user", Content: &mcp.TextContent{Text: intro}},
			{Role: "user", Content: &mcp.TextContent{Text: "## Tool-call recipe\n\n" + playbook}},
			{Role: "user", Content: &mcp.TextContent{Text: guidance}},
			{Role: "user", Content: &mcp.TextContent{Text: closing}},
		},
	}
}
