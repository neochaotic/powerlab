package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// P0.1 from the 2026-05-31 MCP-only chat-mode test: agents that
// only surface Tools (most chat-mode clients) can't reach the
// canonical Prompts and Resources where PowerLab's authoring
// guidance lives. These Tools are thin wrappers around existing
// Prompt/Resource handlers so the same canonical content is
// reachable via the tools/call surface.

// browse_catalog mirrors catalog://index as a Tool. Agents can
// discover what apps are available without knowing they have to
// read a resources/{uri-template} construct first.
func TestBrowseCatalog_ReturnsCatalogApps(t *testing.T) {
	catalogDir := t.TempDir()
	mkApp := func(id string) {
		appDir := filepath.Join(catalogDir, "Apps", id)
		if err := os.MkdirAll(appDir, 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", id, err)
		}
		mustWrite(t, appDir, "docker-compose.yml", "services:\n  x:\n    image: x:1\n")
	}
	mkApp("vaultwarden")
	mkApp("nextcloud")

	srv := newMCPServer(BuildInfo{Version: "test"},
		resourcesConfig{procRoot: t.TempDir(), catalogDir: catalogDir},
		fixtureJournalRunner(""))
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "browse_catalog",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool(browse_catalog): %v", err)
	}
	if res.IsError {
		t.Fatalf("browse_catalog errored: %+v", res.Content)
	}
	var out browseCatalogOutput
	b, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Apps) != 2 {
		t.Fatalf("got %d apps; want 2 (vaultwarden + nextcloud)", len(out.Apps))
	}
	// Check that the catalog URI is reachable from the response so the
	// agent can chain to catalog://app/<id> for the full YAML.
	for _, app := range out.Apps {
		if app.URI == "" {
			t.Errorf("app %q missing URI — agent has nowhere to chain to", app.ID)
		}
	}
}

func TestBrowseCatalog_FilterNarrowsResult(t *testing.T) {
	catalogDir := t.TempDir()
	for _, id := range []string{"vaultwarden", "nextcloud", "vault"} {
		appDir := filepath.Join(catalogDir, "Apps", id)
		_ = os.MkdirAll(appDir, 0o750)
		mustWrite(t, appDir, "docker-compose.yml", "x")
	}

	srv := newMCPServer(BuildInfo{Version: "test"},
		resourcesConfig{procRoot: t.TempDir(), catalogDir: catalogDir},
		fixtureJournalRunner(""))
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "browse_catalog",
		Arguments: map[string]any{"filter": "vault"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	var out browseCatalogOutput
	b, _ := json.Marshal(res.StructuredContent)
	_ = json.Unmarshal(b, &out)
	if len(out.Apps) != 2 {
		t.Fatalf("filter=vault: got %d; want 2 (vaultwarden + vault, NOT nextcloud)", len(out.Apps))
	}
}

// get_compose_conventions returns the docs/concepts/compose-conventions.md
// content as a Tool result. Agents that don't surface Prompts can
// still ground on the canonical conventions document.
func TestGetComposeConventions_ReturnsCanonicalMarkdown(t *testing.T) {
	conceptsDir := t.TempDir()
	canonical := "# Compose conventions\n\nUse /DATA/PowerLabAppData paths.\nNever bind /var/run/docker.sock.\n"
	mustWrite(t, conceptsDir, "compose-conventions.md", canonical)

	srv := newMCPServer(BuildInfo{Version: "test"},
		resourcesConfig{procRoot: t.TempDir(), conceptsDir: conceptsDir},
		fixtureJournalRunner(""))
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "get_compose_conventions",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("get_compose_conventions errored: %+v", res.Content)
	}
	var out getComposeConventionsOutput
	b, _ := json.Marshal(res.StructuredContent)
	_ = json.Unmarshal(b, &out)
	if out.Markdown != canonical {
		t.Fatalf("returned markdown differs from on-disk source.\ngot:  %q\nwant: %q", out.Markdown, canonical)
	}
}

// Missing concepts dir is not an error — the agent gets a clear note.
func TestGetComposeConventions_MissingFileExplainsItself(t *testing.T) {
	srv := newMCPServer(BuildInfo{Version: "test"},
		resourcesConfig{procRoot: t.TempDir(), conceptsDir: "/nope/missing"},
		fixtureJournalRunner(""))
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "get_compose_conventions",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	var out getComposeConventionsOutput
	b, _ := json.Marshal(res.StructuredContent)
	_ = json.Unmarshal(b, &out)
	if out.Note == "" {
		t.Fatalf("missing dir must produce a Note hint, not silent empty markdown")
	}
}

// start_compose_authoring exposes the compose_authoring Prompt as a
// Tool. Same bundle (conventions + 3 catalog examples + validator
// rules) flattened so an agent that only sees tools/list can reach
// it. The optional app_type argument behaves the same as on the
// Prompt.
func TestStartComposeAuthoring_BundleHasExpectedSections(t *testing.T) {
	conceptsDir := t.TempDir()
	catalogDir := t.TempDir()
	mustWrite(t, conceptsDir, "compose-conventions.md", "## conventions\nuse PowerLabAppData paths\n")
	// At least one catalog app so the bundle has an example to cite.
	appDir := filepath.Join(catalogDir, "Apps", "vaultwarden")
	_ = os.MkdirAll(appDir, 0o750)
	mustWrite(t, appDir, "docker-compose.yml", "services:\n  vaultwarden:\n    image: vaultwarden/server:1\n")

	srv := newMCPServer(BuildInfo{Version: "test"},
		resourcesConfig{procRoot: t.TempDir(), conceptsDir: conceptsDir, catalogDir: catalogDir},
		fixtureJournalRunner(""))
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "start_compose_authoring",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("start_compose_authoring errored: %+v", res.Content)
	}
	var out startComposeAuthoringOutput
	b, _ := json.Marshal(res.StructuredContent)
	_ = json.Unmarshal(b, &out)

	bundle := out.Bundle
	for _, want := range []string{"conventions", "validator", "vaultwarden"} {
		if !strings.Contains(strings.ToLower(bundle), want) {
			t.Errorf("bundle missing %q section. bundle=%q", want, bundle)
		}
	}
}

// All 3 discovery tools must appear in tools/list so the agent can
// see them in chat-mode contexts where it only thinks in Tools.
func TestDiscoveryTools_AdvertisedInToolsList(t *testing.T) {
	srv := newMCPServer(BuildInfo{Version: "test"},
		resourcesConfig{procRoot: t.TempDir()},
		fixtureJournalRunner(""))
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	list, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	got := map[string]bool{}
	for _, tool := range list.Tools {
		got[tool.Name] = true
	}
	for _, want := range []string{"browse_catalog", "get_compose_conventions", "start_compose_authoring"} {
		if !got[want] {
			t.Errorf("tools/list missing %q (agent in chat-mode will never see it)", want)
		}
	}
}

// Context is preserved for future ctx-aware callers but currently
// unused — silence the staticcheck warning.
var _ = context.TODO
