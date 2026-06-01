package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// P1.6 from the 2026-05-31 MCP-only chat-mode retro: an agent that
// wants to PROPOSE a docker-compose.yml for operator review has no
// path today — install_app executes immediately, and there is no
// review-first tool. Operators end up with either a fait-accompli
// install or a chat-mode print of YAML that the agent invented
// without running through the deny-list validator.
//
// generate_artifact is the review path: agent submits draft content
// + kind, the Tool validates (if a validator exists for that kind),
// returns a structured envelope the agent surfaces for operator
// approval. Nothing is persisted; nothing is executed.

func callGenerateArtifact(t *testing.T, args map[string]any) GenerateArtifactOutput {
	t.Helper()
	srv := newMCPServer(BuildInfo{Version: "test"},
		resourcesConfig{procRoot: t.TempDir()},
		fixtureJournalRunner(""))
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "generate_artifact",
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(generate_artifact): %v", err)
	}
	if res.IsError {
		t.Fatalf("generate_artifact errored: %+v", res.Content)
	}
	var out GenerateArtifactOutput
	b, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("decode: %v (raw=%s)", err, string(b))
	}
	return out
}

// Compose YAML that follows PowerLab conventions and passes the
// deny-list validator → artifact is OK; no violations.
func TestGenerateArtifact_CompliantComposeYAMLValidates(t *testing.T) {
	yamlBody := `name: testapp
services:
  web:
    image: nginx:1.27
    restart: unless-stopped
`
	out := callGenerateArtifact(t, map[string]any{
		"kind":    "compose-yaml",
		"title":  "testapp v1",
		"content": yamlBody,
	})
	if !out.Validation.OK {
		t.Fatalf("expected OK validation; got violations=%+v", out.Validation.Violations)
	}
	if out.Kind != "compose-yaml" {
		t.Errorf("kind=%q; want compose-yaml", out.Kind)
	}
	if out.Title != "testapp v1" {
		t.Errorf("title=%q; want roundtrip", out.Title)
	}
	if out.Content != yamlBody {
		t.Errorf("content roundtrip mismatch")
	}
}

// Compose YAML with privileged: true → validator rejects, artifact
// surfaces violations so the agent doesn't propose it without
// acknowledging the deny-list hit.
func TestGenerateArtifact_PrivilegedComposeFlagged(t *testing.T) {
	yamlBody := `name: bad
services:
  evil:
    image: alpine
    privileged: true
`
	out := callGenerateArtifact(t, map[string]any{
		"kind":    "compose-yaml",
		"title":   "definitely not safe",
		"content": yamlBody,
	})
	if out.Validation.OK {
		t.Fatalf("expected validation NOT OK for privileged: true")
	}
	hit := false
	for _, v := range out.Validation.Violations {
		if strings.Contains(strings.ToLower(v.Code+v.Detail), "privileged") {
			hit = true
		}
	}
	if !hit {
		t.Errorf("expected a violation mentioning privileged; got %+v", out.Validation.Violations)
	}
}

// Docker socket bind → another classic deny-list trip.
func TestGenerateArtifact_DockerSocketBindFlagged(t *testing.T) {
	yamlBody := `name: socketbinder
services:
  bad:
    image: alpine
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
`
	out := callGenerateArtifact(t, map[string]any{
		"kind":    "compose-yaml",
		"title":   "docker socket bind",
		"content": yamlBody,
	})
	if out.Validation.OK {
		t.Fatalf("expected validation NOT OK for docker.sock bind")
	}
}

// Shell-script kind has no validator wired today; the artifact still
// roundtrips and validation reports "no validator for this kind"
// rather than silently claiming OK.
func TestGenerateArtifact_ShellScriptKindRoundtripsWithoutValidator(t *testing.T) {
	body := "#!/bin/bash\necho hello\n"
	out := callGenerateArtifact(t, map[string]any{
		"kind":    "shell-script",
		"title":   "hello.sh",
		"content": body,
	})
	if out.Content != body {
		t.Errorf("shell content roundtrip mismatch")
	}
	// No validator for this kind — output should say so clearly so
	// the agent doesn't represent it as "validated".
	if out.Validation.OK {
		t.Errorf("OK=true on unvalidated kind misleads the agent")
	}
	if out.Validation.Note == "" {
		t.Errorf("expected a Note explaining no validator exists for shell-script")
	}
}

// Markdown kind → same passthrough, same explicit "no validator".
func TestGenerateArtifact_MarkdownKindNoValidator(t *testing.T) {
	body := "# Title\n\nSome body."
	out := callGenerateArtifact(t, map[string]any{
		"kind":    "markdown",
		"title":   "doc draft",
		"content": body,
	})
	if out.Content != body {
		t.Errorf("markdown content roundtrip mismatch")
	}
	if out.Validation.OK {
		t.Errorf("OK=true on unvalidated kind misleads the agent")
	}
}

// Empty content → reject before validation runs. The artifact is
// useless without content; the agent should get a clear error so
// it doesn't surface an empty draft to the operator.
func TestGenerateArtifact_EmptyContentRejected(t *testing.T) {
	srv := newMCPServer(BuildInfo{Version: "test"},
		resourcesConfig{procRoot: t.TempDir()},
		fixtureJournalRunner(""))
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "generate_artifact",
		Arguments: map[string]any{
			"kind":    "compose-yaml",
			"title":   "empty",
			"content": "",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError=true for empty content")
	}
}

// Tool MUST appear in tools/list with READ side-effect class — the
// whole point is chat-mode discoverability.
func TestGenerateArtifact_AdvertisedInToolsList(t *testing.T) {
	srv := newMCPServer(BuildInfo{Version: "test"},
		resourcesConfig{procRoot: t.TempDir()},
		fixtureJournalRunner(""))
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	list, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	for _, tool := range list.Tools {
		if tool.Name == "generate_artifact" {
			if !strings.Contains(tool.Description, "READ") {
				t.Errorf("description missing READ side-effect class: %q", tool.Description)
			}
			return
		}
	}
	t.Fatalf("generate_artifact not advertised")
}
