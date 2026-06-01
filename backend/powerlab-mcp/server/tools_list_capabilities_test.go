package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// P2.8 from the 2026-05-31 MCP-only chat-mode retro: agents wasted
// turns trying gated tools (install_app, journal://system/auth)
// without knowing whether the operator had opted into those tiers.
// list_capabilities is the meta-tool that reports which tiers are
// active on THIS server so the agent can plan without a
// trial-and-error round-trip.

func callListCapabilities(t *testing.T, rc resourcesConfig) ListCapabilitiesOutput {
	t.Helper()
	srv := newMCPServer(BuildInfo{Version: "test"}, rc, fixtureJournalRunner(""))
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "list_capabilities",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool(list_capabilities): %v", err)
	}
	if res.IsError {
		t.Fatalf("list_capabilities errored: %+v", res.Content)
	}
	var out ListCapabilitiesOutput
	b, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out
}

// Default (no opt-ins): destructive disabled, sensitive disabled.
// Agent learns it cannot install/uninstall/restart and cannot read
// the host-auth journal — saves a trial-and-error call to each.
func TestListCapabilities_DefaultLocksDown(t *testing.T) {
	rc := resourcesConfig{procRoot: t.TempDir()}
	out := callListCapabilities(t, rc)
	if out.DestructiveToolsEnabled {
		t.Fatalf("default: DestructiveToolsEnabled=true; want false")
	}
	if out.SensitiveTierEnabled {
		t.Fatalf("default: SensitiveTierEnabled=true; want false")
	}
	if len(out.DestructiveTools) != 0 {
		t.Fatalf("default: DestructiveTools=%v; want empty list", out.DestructiveTools)
	}
}

// EnableDestructiveTools=true → tool names appear so the agent can
// reason about which actions are reachable.
func TestListCapabilities_DestructiveTrueListsTools(t *testing.T) {
	rc := resourcesConfig{procRoot: t.TempDir(), enableDestructiveTools: true}
	out := callListCapabilities(t, rc)
	if !out.DestructiveToolsEnabled {
		t.Fatalf("DestructiveToolsEnabled=false with opt-in; want true")
	}
	got := strings.Join(out.DestructiveTools, ",")
	for _, want := range []string{"install_app", "uninstall_app", "restart_app"} {
		if !strings.Contains(got, want) {
			t.Errorf("destructive tools list missing %q; got %q", want, got)
		}
	}
}

// EnableSensitiveTier=true → the auth journal URIs appear so the
// agent knows it's allowed to read them.
func TestListCapabilities_SensitiveTrueListsResources(t *testing.T) {
	rc := resourcesConfig{procRoot: t.TempDir(), enableSensitiveTier: true}
	out := callListCapabilities(t, rc)
	if !out.SensitiveTierEnabled {
		t.Fatalf("SensitiveTierEnabled=false with opt-in; want true")
	}
	got := strings.Join(out.SensitiveResources, ",")
	for _, want := range []string{"journal://system/auth", "journal://system/failures"} {
		if !strings.Contains(got, want) {
			t.Errorf("sensitive resources list missing %q; got %q", want, got)
		}
	}
}

// Summary string must reflect both tiers in plain text so even an
// agent that doesn't parse the structured fields can surface the
// state to the operator.
func TestListCapabilities_SummaryReflectsState(t *testing.T) {
	rc := resourcesConfig{procRoot: t.TempDir(), enableDestructiveTools: true, enableSensitiveTier: false}
	out := callListCapabilities(t, rc)
	s := strings.ToLower(out.Summary)
	if !strings.Contains(s, "destructive") {
		t.Errorf("summary %q must mention 'destructive' state", out.Summary)
	}
	if !strings.Contains(s, "sensitive") {
		t.Errorf("summary %q must mention 'sensitive' state", out.Summary)
	}
}

// Tool MUST appear in tools/list with READ side-effect class — like
// every other discovery tool, it's read-only by definition.
func TestListCapabilities_AdvertisedInToolsList(t *testing.T) {
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
		if tool.Name == "list_capabilities" {
			if !strings.Contains(tool.Description, "READ") {
				t.Errorf("description missing READ side-effect class")
			}
			return
		}
	}
	t.Fatalf("list_capabilities not advertised in tools/list")
}
