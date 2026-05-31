package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ADR-0048 — compose_authoring is an MCP Prompt primitive. When an
// agent (or the user via clients that surface prompts) invokes it,
// the server returns a bundle of curated context ready to ground a
// "design me a PowerLab-idiomatic docker-compose" reasoning task.
//
// The bundle contains:
//  1. system message: the canonical PowerLab compose conventions
//     (sourced from docs/concepts/compose-conventions.md)
//  2. user message: 3 representative catalog YAMLs (deterministic pick
//     by app_type when supplied; stable default trio when empty)
//  3. user message: the composevalidator deny-list — what NOT to write
//  4. user message: a parameterized question template
//
// Token cost ~3-5 KB; far cheaper than the agent doing N discovery
// round-trips through catalog://app/<id> on its own.

const composeAuthoringPromptName = "compose_authoring"

// defaultExamplePicks is the trio shown when app_type is empty or
// doesn't match any heuristic. Picked to span the design space:
// stateless service, stateful service, complex multi-container app.
var defaultExamplePicks = []string{"helloworld", "code-server", "nextcloud"}

// appTypeKeyword → preferred example ids. Heuristic, not exhaustive
// — when no keyword matches we fall back to defaultExamplePicks.
var examplePicksByKeyword = map[string][]string{
	"database":   {"postgres", "mariadb", "redis"},
	"db":         {"postgres", "mariadb", "redis"},
	"media":      {"jellyfin", "plex", "navidrome"},
	"web":        {"nginx", "caddy", "code-server"},
	"static":     {"nginx", "caddy", "memos"},
	"chat":       {"chatbot-ui", "rocket-chat", "matrix"},
	"ai":         {"chatbot-ui", "ollama", "open-webui"},
	"download":   {"qbittorrent", "sonarr", "radarr"},
	"dashboard":  {"homepage", "homarr", "dashy"},
	"monitoring": {"grafana", "uptime-kuma", "prometheus"},
	"finance":    {"actualbudget", "firefly-iii", "memos"},
	"git":        {"gitea", "gitlab-ce", "forgejo"},
	"dev":        {"code-server", "gitea", "drawio"},
}

func registerComposeAuthoringPrompt(s *mcp.Server, conceptsDir, catalogDir string) {
	s.AddPrompt(
		&mcp.Prompt{
			Name:        composeAuthoringPromptName,
			Description: "Bundles the PowerLab compose conventions + 3 representative catalog YAMLs + the composevalidator deny-list, ready to ground an agent designing a new PowerLab docker-compose file. One invocation replaces N discovery round-trips. Optional app_type argument hints which catalog examples to pick (e.g., 'database', 'media', 'ai'); empty defaults to a representative trio.",
			Arguments: []*mcp.PromptArgument{
				{
					Name:        "app_type",
					Description: "Optional hint for which catalog examples to bundle (e.g., 'database', 'media', 'ai', 'dashboard'). Empty value yields a representative default trio.",
					Required:    false,
				},
			},
		},
		func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			appType := ""
			if req.Params.Arguments != nil {
				appType = strings.TrimSpace(req.Params.Arguments["app_type"])
			}
			return buildComposeAuthoringResult(conceptsDir, catalogDir, appType), nil
		},
	)
}

// buildComposeAuthoringResult assembles the curated message bundle.
// Pure function over the configured directories — easy to test.
func buildComposeAuthoringResult(conceptsDir, catalogDir, appType string) *mcp.GetPromptResult {
	conventions := readConventions(conceptsDir)
	validatorRules := composeValidatorRulesPrompt()
	examples := loadExamples(catalogDir, appType)

	messages := []*mcp.PromptMessage{
		{
			Role: "user",
			Content: &mcp.TextContent{Text: "You are designing a docker-compose.yml for a new PowerLab app. PowerLab has specific conventions you must follow — and a deny-list of patterns the install validator REJECTS. The next messages give you the canonical conventions, three real PowerLab apps as patterns to learn from, and the exact rules the validator enforces. Reason from this context before drafting the YAML."},
		},
		{
			Role:    "user",
			Content: &mcp.TextContent{Text: "## PowerLab compose conventions (canonical)\n\n" + conventions},
		},
	}

	for _, ex := range examples {
		messages = append(messages, &mcp.PromptMessage{
			Role:    "user",
			Content: &mcp.TextContent{Text: fmt.Sprintf("## Example: %s/docker-compose.yml\n\n```yaml\n%s\n```", ex.id, ex.body)},
		})
	}

	messages = append(messages, &mcp.PromptMessage{
		Role:    "user",
		Content: &mcp.TextContent{Text: "## composevalidator deny-list — install_app REJECTS YAMLs matching any of these\n\n" + validatorRules},
	})

	prompt := "Now: ask the user for any unclear specifics (image, ports, env vars), then propose a PowerLab-idiomatic docker-compose.yml. Cite which catalog example(s) you drew from."
	if appType != "" {
		prompt = fmt.Sprintf("Now: ask the user for any unclear specifics (image, ports, env vars), then propose a PowerLab-idiomatic docker-compose.yml for a %s. Cite which catalog example(s) you drew from.", appType)
	}
	messages = append(messages, &mcp.PromptMessage{
		Role:    "user",
		Content: &mcp.TextContent{Text: prompt},
	})

	return &mcp.GetPromptResult{
		Description: "PowerLab compose authoring bundle — conventions + examples + validator rules.",
		Messages:    messages,
	}
}

// readConventions reads docs/concepts/compose-conventions.md. Missing
// file → returns a short stub explaining the situation, so the prompt
// still produces a useful bundle on a dev box without the install
// staged.
func readConventions(conceptsDir string) string {
	path := filepath.Join(conceptsDir, "compose-conventions.md")
	// #nosec G304 -- conceptsDir is operator-configured; filename is literal.
	body, err := os.ReadFile(path)
	if err != nil {
		return "(compose-conventions.md not staged on this host — the canonical conventions document was not bundled with the install. Reason from generic docker-compose best-practice; the validator rules in the next section are still enforced.)"
	}
	return string(body)
}

// composeValidatorRulesPrompt encodes the deny-list rules
// composevalidator (ADR-0046 §4) enforces, in the agent-facing voice.
// Kept as a function returning a literal so the prompt bundle stays
// self-contained — no dependency on the composevalidator package's
// internals (which would pull validator-private types into this
// prompt's surface).
func composeValidatorRulesPrompt() string {
	return strings.TrimSpace(`
Any of the following patterns cause install_app to REJECT the YAML before forwarding to app-management. NEVER include these in your design:

- ` + "`privileged: true`" + ` — full container escape.
- ` + "`network_mode: host`" + `, ` + "`network_mode: container:*`" + ` — host namespace bypass.
- ` + "`pid: host`" + `, ` + "`ipc: host`" + `, ` + "`uts: host`" + `, ` + "`userns_mode: host`" + ` — same.
- ` + "`cap_add:`" + ` containing ` + "`SYS_ADMIN`, `ALL`, `NET_ADMIN`, `SYS_PTRACE`" + `, ` + "`SYS_MODULE`, `SYS_RAWIO`, `SYS_BOOT`, `SYS_TIME`, `MAC_ADMIN`, `MAC_OVERRIDE`, `DAC_READ_SEARCH`" + ` — container escape via specific syscalls.
- Volume binds to ` + "`/var/run/docker.sock`" + ` or any docker-socket variant — gives the container full host control.
- Volume binds with host source in ` + "`/proc`, `/sys`, `/etc`, `/root`, `/var/lib`, `/var/log`, `/dev`, `/boot`" + `, or library/binary directories — sensitive host path exposure.
- Raw ` + "`devices:`" + ` entries pointing at unrestricted device files (specific named devices like /dev/dri/renderD128 for transcoding are OK; the deny-list rejects unrestricted patterns).

If your design genuinely needs one of these (rare, specific hardware passthrough), the operator must install manually outside install_app. The MCP install path is for apps that fit the validator's safety envelope.
`)
}

type catalogExample struct {
	id   string
	body string
}

// loadExamples picks 3 catalog apps based on app_type heuristic.
// Returns whatever exists from the picked set; if fewer than 3 of
// the picks have catalog files, falls through to defaultExamplePicks.
// Always returns at most 3 examples to keep the prompt bundle bounded.
func loadExamples(catalogDir, appType string) []catalogExample {
	picks := pickExamples(appType)

	out := []catalogExample{}
	for _, id := range picks {
		path := filepath.Join(catalogDir, "Apps", id, "docker-compose.yml")
		// #nosec G304 -- catalogDir is operator-configured; id comes
		// from the curated pickExamples list, not user input.
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		out = append(out, catalogExample{id: id, body: string(body)})
		if len(out) >= 3 {
			break
		}
	}

	// If the keyword-matched picks didn't yield enough, top up from
	// the default trio (skip already-included ids).
	if len(out) < 3 {
		seen := map[string]bool{}
		for _, e := range out {
			seen[e.id] = true
		}
		for _, id := range defaultExamplePicks {
			if seen[id] {
				continue
			}
			path := filepath.Join(catalogDir, "Apps", id, "docker-compose.yml")
			// #nosec G304 -- same justification as above.
			body, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			out = append(out, catalogExample{id: id, body: string(body)})
			if len(out) >= 3 {
				break
			}
		}
	}

	// Last-resort fallback: on a dev box without the catalog staged,
	// pick the first 3 apps that DO exist so the prompt is never
	// entirely empty.
	if len(out) == 0 {
		out = fallbackAnyThreeApps(catalogDir)
	}

	return out
}

func pickExamples(appType string) []string {
	if appType == "" {
		return defaultExamplePicks
	}
	lower := strings.ToLower(appType)
	for keyword, picks := range examplePicksByKeyword {
		if strings.Contains(lower, keyword) {
			return picks
		}
	}
	return defaultExamplePicks
}

func fallbackAnyThreeApps(catalogDir string) []catalogExample {
	appsRoot := filepath.Join(catalogDir, "Apps")
	entries, err := os.ReadDir(appsRoot)
	if err != nil {
		return []catalogExample{}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	out := []catalogExample{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(appsRoot, e.Name(), "docker-compose.yml")
		// #nosec G304 -- catalogDir is operator-configured.
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		out = append(out, catalogExample{id: e.Name(), body: string(body)})
		if len(out) >= 3 {
			break
		}
	}
	return out
}
