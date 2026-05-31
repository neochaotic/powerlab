package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// prompts/list must advertise compose_authoring with its app_type
// argument; prompts/get with a valid arg returns a non-empty bundle.
func TestComposeAuthoringPrompt_IsAdvertisedAndCallable(t *testing.T) {
	conceptsDir, catalogDir := stageFixtures(t)

	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	registerComposeAuthoringPrompt(srv, conceptsDir, catalogDir)
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	// prompts/list: compose_authoring with app_type arg.
	list, err := cs.ListPrompts(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListPrompts: %v", err)
	}
	var found *mcp.Prompt
	for _, p := range list.Prompts {
		if p.Name == composeAuthoringPromptName {
			found = p
			break
		}
	}
	if found == nil {
		t.Fatalf("compose_authoring not advertised in prompts/list")
	}
	if found.Description == "" {
		t.Errorf("prompt description empty")
	}
	hasAppType := false
	for _, arg := range found.Arguments {
		if arg.Name == "app_type" {
			hasAppType = true
		}
	}
	if !hasAppType {
		t.Errorf("compose_authoring missing 'app_type' argument")
	}

	// prompts/get with app_type="database": bundle includes a postgres
	// example (from the keyword heuristic) AND the conventions doc
	// AND the validator deny-list rules.
	got, err := cs.GetPrompt(t.Context(), &mcp.GetPromptParams{
		Name:      composeAuthoringPromptName,
		Arguments: map[string]string{"app_type": "database"},
	})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	if len(got.Messages) < 3 {
		t.Fatalf("prompt bundle has %d messages; want at least 3 (instructions + conventions + validator)", len(got.Messages))
	}

	bundle := concatPromptText(got.Messages)
	for _, must := range []string{
		"PowerLab compose conventions",
		"composevalidator deny-list",
		"/DATA/PowerLabAppData", // conventions content
		"privileged: true",      // validator content
		"postgres",              // database keyword pick
	} {
		if !strings.Contains(bundle, must) {
			t.Errorf("bundle missing expected substring %q", must)
		}
	}
}

// Empty app_type yields the default trio + a generic question template
// (no app-type-specific phrasing in the final user message).
func TestComposeAuthoringPrompt_EmptyAppTypeYieldsDefaults(t *testing.T) {
	conceptsDir, catalogDir := stageFixtures(t)

	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	registerComposeAuthoringPrompt(srv, conceptsDir, catalogDir)
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	got, err := cs.GetPrompt(t.Context(), &mcp.GetPromptParams{
		Name:      composeAuthoringPromptName,
		Arguments: map[string]string{}, // no app_type
	})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	bundle := concatPromptText(got.Messages)

	// helloworld is in the default trio.
	if !strings.Contains(bundle, "helloworld") {
		t.Errorf("default bundle missing 'helloworld' example")
	}
	// Final message must NOT mention a specific app_type since none
	// was provided.
	final := got.Messages[len(got.Messages)-1]
	tc, ok := final.Content.(*mcp.TextContent)
	if !ok {
		t.Fatalf("final message content is not TextContent: %T", final.Content)
	}
	if strings.Contains(tc.Text, " for a ") {
		t.Errorf("empty app_type yielded parameterized prompt: %q", tc.Text)
	}
}

// Missing conventions file: bundle still produces a valid result with
// a stub message in place of the conventions content.
func TestComposeAuthoringPrompt_MissingConventionsStillBuilds(t *testing.T) {
	conceptsDir := t.TempDir() // empty — no compose-conventions.md
	_, catalogDir := stageFixtures(t)

	got := buildComposeAuthoringResult(conceptsDir, catalogDir, "")
	if len(got.Messages) < 3 {
		t.Fatalf("bundle has %d messages; should still build on missing conventions", len(got.Messages))
	}
	bundle := concatPromptText(got.Messages)
	if !strings.Contains(bundle, "not staged on this host") {
		t.Errorf("missing-conventions stub not surfaced; agent has no hint why conventions are absent")
	}
}

func stageFixtures(t *testing.T) (conceptsDir, catalogDir string) {
	t.Helper()
	conceptsDir = t.TempDir()
	catalogDir = t.TempDir()

	if err := os.WriteFile(
		filepath.Join(conceptsDir, "compose-conventions.md"),
		[]byte("# PowerLab compose conventions\n\nUse /DATA/PowerLabAppData/<app_id>/ paths.\n"),
		0o600,
	); err != nil {
		t.Fatalf("write conventions: %v", err)
	}
	stageCatalogApp := func(id, body string) {
		dir := filepath.Join(catalogDir, "Apps", id)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(body), 0o600); err != nil {
			t.Fatalf("write %s compose: %v", id, err)
		}
	}
	stageCatalogApp("helloworld", "services:\n  helloworld:\n    image: nginx:1.27.3-alpine\n")
	stageCatalogApp("code-server", "services:\n  code-server:\n    image: codercom/code-server:4.20\n")
	stageCatalogApp("nextcloud", "services:\n  nextcloud:\n    image: nextcloud:30\n")
	stageCatalogApp("postgres", "services:\n  postgres:\n    image: postgres:16-alpine\n")
	stageCatalogApp("mariadb", "services:\n  mariadb:\n    image: mariadb:11\n")
	stageCatalogApp("redis", "services:\n  redis:\n    image: redis:7-alpine\n")
	return conceptsDir, catalogDir
}

func concatPromptText(messages []*mcp.PromptMessage) string {
	var sb strings.Builder
	for _, m := range messages {
		if tc, ok := m.Content.(*mcp.TextContent); ok {
			sb.WriteString(tc.Text)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
