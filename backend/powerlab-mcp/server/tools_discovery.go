package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// P0.1 from the 2026-05-31 MCP-only chat-mode test: agents that only
// surface Tools (the prevailing chat-mode client UX) couldn't reach
// PowerLab's canonical authoring guidance — it lived in Prompts
// (compose_authoring) and templated Resources (catalog://app/{id})
// that those clients don't autonomously discover. These three
// "discovery" Tools wrap the same handlers so the canonical content
// is reachable via tools/call without changing the underlying Prompt
// + Resource contracts.
//
// Architectural redundancy is intentional. Each Tool is a thin
// adapter: it builds nothing new, it just exposes existing handlers
// through the surface the agent actually thinks in.
//
// Tools added (READ ONLY):
//   - browse_catalog          → wraps buildCatalogManifest (catalog://index)
//   - get_compose_conventions → wraps readConventions (docs://concepts/compose-conventions)
//   - start_compose_authoring → wraps buildComposeAuthoringResult (compose_authoring Prompt)

// ---------- browse_catalog ----------

type browseCatalogInput struct {
	Filter string `json:"filter,omitempty" jsonschema:"optional case-insensitive substring filter on app ID; empty returns every cataloged app"`
}

type browseCatalogApp struct {
	ID  string `json:"id"`
	URI string `json:"uri"`
}

type browseCatalogOutput struct {
	Apps  []browseCatalogApp `json:"apps"`
	Total int                `json:"total"`
	Note  string             `json:"note,omitempty"`
}

func registerBrowseCatalog(s *mcp.Server, catalogDir string) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "browse_catalog",
		Description: "READ ONLY — list PowerLab community-catalog apps available for install or as compose-pattern references. Mirrors catalog://index in tool form so chat-mode agents can discover apps without resource-template navigation. Optional filter narrows by case-insensitive substring on app id. Use catalog://app/<id> (or search_docs source=catalog) to fetch a specific app's docker-compose.yml.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in browseCatalogInput) (*mcp.CallToolResult, browseCatalogOutput, error) {
		raw, err := buildCatalogManifest(catalogDir)
		if err != nil {
			return nil, browseCatalogOutput{Apps: []browseCatalogApp{}, Note: "catalog read failed: " + err.Error()}, nil
		}
		var m catalogManifest
		if jerr := json.Unmarshal(raw, &m); jerr != nil {
			return nil, browseCatalogOutput{Apps: []browseCatalogApp{}, Note: "catalog manifest decode failed: " + jerr.Error()}, nil
		}
		filter := strings.ToLower(strings.TrimSpace(in.Filter))
		out := browseCatalogOutput{Apps: []browseCatalogApp{}}
		for _, e := range m.Apps {
			if filter != "" && !strings.Contains(strings.ToLower(e.ID), filter) {
				continue
			}
			// Resource type and Tool output type are deliberately
			// distinct (the Resource wire shape and Tool wire shape
			// are allowed to diverge over time). Build the Tool
			// shape explicitly even when fields currently match.
			out.Apps = append(out.Apps, browseCatalogApp{ //nolint:staticcheck // S1016 — intentional type decoupling
				ID:  e.ID,
				URI: e.URI,
			})
		}
		out.Total = len(out.Apps)
		if len(m.Apps) == 0 {
			out.Note = "catalog is empty on this host (fresh box, dev environment, or installer didn't stage Apps/)"
		}
		return nil, out, nil
	})
}

// ---------- get_compose_conventions ----------

type getComposeConventionsInput struct{}

type getComposeConventionsOutput struct {
	Markdown string `json:"markdown"`
	URI      string `json:"uri"`
	Note     string `json:"note,omitempty"`
}

func registerGetComposeConventions(s *mcp.Server, conceptsDir string) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_compose_conventions",
		Description: "READ ONLY — returns the canonical PowerLab docker-compose conventions document (the source of truth for x-powerlab metadata, volume paths, port allocation, image trust policy, etc.). Mirrors docs://concepts/compose-conventions so chat-mode agents have a direct fetch path. Use this before drafting any compose YAML for PowerLab to avoid legacy CasaOS-era idioms.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ getComposeConventionsInput) (*mcp.CallToolResult, getComposeConventionsOutput, error) {
		path := filepath.Join(conceptsDir, "compose-conventions.md")
		// #nosec G304 -- conceptsDir is operator-configured; filename literal.
		body, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, getComposeConventionsOutput{
					URI:  docsConceptsPrefix + "compose-conventions",
					Note: fmt.Sprintf("compose-conventions.md not staged on this host (looked at %s); install.sh stages it from docs/concepts/", path),
				}, nil
			}
			return nil, getComposeConventionsOutput{
				URI:  docsConceptsPrefix + "compose-conventions",
				Note: "read failed: " + err.Error(),
			}, nil
		}
		return nil, getComposeConventionsOutput{
			Markdown: string(body),
			URI:      docsConceptsPrefix + "compose-conventions",
		}, nil
	})
}

// ---------- start_compose_authoring ----------

type startComposeAuthoringInput struct {
	AppType string `json:"app_type,omitempty" jsonschema:"optional hint for which catalog examples to bundle (e.g., 'database', 'media', 'ai', 'dashboard'); empty yields a representative default trio"`
}

type startComposeAuthoringOutput struct {
	// Bundle is the flattened markdown the agent should treat as its
	// grounding context for the authoring task: conventions + 3
	// catalog examples + validator deny-list rules.
	Bundle string `json:"bundle"`
	// PromptURI points to the canonical Prompt primitive for clients
	// that DO surface Prompts (a richer multi-message structure
	// available via prompts/get). The Tool form is the chat-mode
	// fallback.
	PromptURI string `json:"prompt_uri"`
}

func registerStartComposeAuthoring(s *mcp.Server, conceptsDir, catalogDir string) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "start_compose_authoring",
		Description: "READ ONLY — produces the curated authoring bundle for drafting a new PowerLab docker-compose.yml: canonical conventions + 3 representative catalog examples + the composevalidator deny-list rules. Optional app_type hint biases the catalog example selection ('database', 'media', 'ai', 'dashboard'). Use this BEFORE drafting any compose YAML — drafting from the source tree or from public docs produces legacy/incorrect idioms. Tool form of the compose_authoring Prompt primitive, for chat-mode agents that don't surface Prompts.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in startComposeAuthoringInput) (*mcp.CallToolResult, startComposeAuthoringOutput, error) {
		appType := strings.TrimSpace(in.AppType)
		res := buildComposeAuthoringResult(conceptsDir, catalogDir, appType)
		var bundle strings.Builder
		for _, m := range res.Messages {
			if tc, ok := m.Content.(*mcp.TextContent); ok {
				bundle.WriteString(tc.Text)
				bundle.WriteString("\n\n")
			}
		}
		return nil, startComposeAuthoringOutput{
			Bundle:    strings.TrimSpace(bundle.String()),
			PromptURI: "prompt://" + composeAuthoringPromptName,
		}, nil
	})
}

// registerDiscoveryTools registers all three P0.1 discovery Tools.
// Wired from server.New alongside the existing Resource + Prompt
// registrations, so an agent sees them in tools/list immediately.
func registerDiscoveryTools(s *mcp.Server, conceptsDir, catalogDir string) {
	registerBrowseCatalog(s, catalogDir)
	registerGetComposeConventions(s, conceptsDir)
	registerStartComposeAuthoring(s, conceptsDir, catalogDir)
}
