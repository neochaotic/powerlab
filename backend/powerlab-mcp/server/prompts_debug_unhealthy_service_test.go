package server

import (
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// prompts/list must advertise debug_unhealthy_service with service +
// since_minutes args; prompts/get returns a playbook chaining
// get_system_health → journal_search → check_disk_free.
func TestDebugUnhealthyServicePrompt_IsAdvertisedAndCallable(t *testing.T) {
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	registerDebugUnhealthyServicePrompt(srv)
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	list, err := cs.ListPrompts(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListPrompts: %v", err)
	}
	var found *mcp.Prompt
	for _, p := range list.Prompts {
		if p.Name == debugUnhealthyServicePromptName {
			found = p
			break
		}
	}
	if found == nil {
		t.Fatalf("%s not advertised in prompts/list", debugUnhealthyServicePromptName)
	}
	if found.Description == "" {
		t.Errorf("prompt description empty")
	}
	wantArgs := map[string]bool{"service": false, "since_minutes": false}
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
		Name:      debugUnhealthyServicePromptName,
		Arguments: map[string]string{"service": "gateway", "since_minutes": "15"},
	})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	if len(got.Messages) < 2 {
		t.Fatalf("bundle has %d messages; want ≥ 2", len(got.Messages))
	}
	bundle := concatPromptText(got.Messages)

	for _, must := range []string{
		"get_system_health",
		"journal_search",
		"check_disk_free",
		"gateway", // service argument surfaced
		"15",      // since_minutes argument surfaced
	} {
		if !strings.Contains(bundle, must) {
			t.Errorf("bundle missing expected substring %q", must)
		}
	}
}

func TestDebugUnhealthyServicePrompt_EmptyArgsYieldGeneric(t *testing.T) {
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	registerDebugUnhealthyServicePrompt(srv)
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	got, err := cs.GetPrompt(t.Context(), &mcp.GetPromptParams{
		Name:      debugUnhealthyServicePromptName,
		Arguments: map[string]string{},
	})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	bundle := concatPromptText(got.Messages)
	if !strings.Contains(bundle, "get_system_health") {
		t.Errorf("generic bundle missing core Tool chain reference")
	}
	if strings.Contains(bundle, "unit=\"\"") {
		t.Errorf("generic bundle emitted an empty unit filter")
	}
}
