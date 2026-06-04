package server

import (
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// prompts/list must advertise troubleshoot_install_failure with its
// two optional arguments; prompts/get returns a structured diagnostic
// playbook that names the specific Tools the agent should chain
// (audit_query → journal_search → get_system_health).
func TestTroubleshootInstallFailurePrompt_IsAdvertisedAndCallable(t *testing.T) {
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	registerTroubleshootInstallFailurePrompt(srv)
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	list, err := cs.ListPrompts(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListPrompts: %v", err)
	}
	var found *mcp.Prompt
	for _, p := range list.Prompts {
		if p.Name == troubleshootInstallFailurePromptName {
			found = p
			break
		}
	}
	if found == nil {
		t.Fatalf("%s not advertised in prompts/list", troubleshootInstallFailurePromptName)
	}
	if found.Description == "" {
		t.Errorf("prompt description empty")
	}
	wantArgs := map[string]bool{"app_id": false, "since_minutes": false}
	for _, arg := range found.Arguments {
		if _, want := wantArgs[arg.Name]; want {
			wantArgs[arg.Name] = true
		}
	}
	for name, seen := range wantArgs {
		if !seen {
			t.Errorf("missing optional argument %q", name)
		}
	}

	got, err := cs.GetPrompt(t.Context(), &mcp.GetPromptParams{
		Name:      troubleshootInstallFailurePromptName,
		Arguments: map[string]string{"app_id": "jellyfin", "since_minutes": "30"},
	})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	if len(got.Messages) < 2 {
		t.Fatalf("bundle has %d messages; want ≥ 2 (instructions + playbook)", len(got.Messages))
	}
	bundle := concatPromptText(got.Messages)

	// The prompt must explicitly cite the Tool chain — without the
	// names, an agent without Tool affinity for chaining doesn't know
	// what to call. This is the whole point of the Prompt: encode the
	// observability recipe.
	for _, must := range []string{
		"audit_query",
		"journal_search",
		"get_system_health",
		"jellyfin",      // app_id parameter surfaced
		"30",            // since_minutes parameter surfaced
		"install_app",   // diagnostic context: what action failed
	} {
		if !strings.Contains(bundle, must) {
			t.Errorf("bundle missing expected substring %q", must)
		}
	}
}

// Empty arguments yield a generic version of the playbook without
// app-specific phrasing. Agents that invoke the prompt cold (no app
// id known yet) still get a useful chain to start from.
func TestTroubleshootInstallFailurePrompt_EmptyArgsYieldGeneric(t *testing.T) {
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	registerTroubleshootInstallFailurePrompt(srv)
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	got, err := cs.GetPrompt(t.Context(), &mcp.GetPromptParams{
		Name:      troubleshootInstallFailurePromptName,
		Arguments: map[string]string{},
	})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	bundle := concatPromptText(got.Messages)
	if !strings.Contains(bundle, "audit_query") {
		t.Errorf("generic bundle missing core Tool chain reference")
	}
	// No app_id supplied → must not pretend to know one.
	if strings.Contains(bundle, "app_id=\"") {
		t.Errorf("generic bundle emitted a parameterized app_id=\\\"…\\\" call without an arg")
	}
}
