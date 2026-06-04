package server

import (
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// prompts/list must advertise onboard_new_powerlab_host with goal +
// experience_level args; prompts/get returns a walkthrough chaining
// list_capabilities → browse_catalog → next steps.
func TestOnboardNewPowerlabHostPrompt_IsAdvertisedAndCallable(t *testing.T) {
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	registerOnboardNewPowerlabHostPrompt(srv)
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	list, err := cs.ListPrompts(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListPrompts: %v", err)
	}
	var found *mcp.Prompt
	for _, p := range list.Prompts {
		if p.Name == onboardNewPowerlabHostPromptName {
			found = p
			break
		}
	}
	if found == nil {
		t.Fatalf("%s not advertised in prompts/list", onboardNewPowerlabHostPromptName)
	}
	if found.Description == "" {
		t.Errorf("prompt description empty")
	}
	wantArgs := map[string]bool{"goal": false, "experience_level": false}
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
		Name:      onboardNewPowerlabHostPromptName,
		Arguments: map[string]string{"goal": "host my media library", "experience_level": "beginner"},
	})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	if len(got.Messages) < 2 {
		t.Fatalf("bundle has %d messages; want ≥ 2", len(got.Messages))
	}
	bundle := concatPromptText(got.Messages)

	for _, must := range []string{
		"list_capabilities",
		"browse_catalog",
		"get_system_health",
		"media library", // goal surfaced
		"beginner",      // experience_level surfaced
	} {
		if !strings.Contains(bundle, must) {
			t.Errorf("bundle missing expected substring %q", must)
		}
	}
}

func TestOnboardNewPowerlabHostPrompt_EmptyArgsYieldGeneric(t *testing.T) {
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	registerOnboardNewPowerlabHostPrompt(srv)
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	got, err := cs.GetPrompt(t.Context(), &mcp.GetPromptParams{
		Name:      onboardNewPowerlabHostPromptName,
		Arguments: map[string]string{},
	})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	bundle := concatPromptText(got.Messages)
	if !strings.Contains(bundle, "list_capabilities") {
		t.Errorf("generic bundle missing core Tool chain reference")
	}
	if strings.Contains(bundle, "goal=\"\"") {
		t.Errorf("generic bundle emitted an empty goal placeholder")
	}
}
