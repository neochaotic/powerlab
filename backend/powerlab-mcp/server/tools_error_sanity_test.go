package server

import (
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Error-sanity regression suite. These tests pin the contract that
// agent-visible Tool outputs (errors, Notes, Summaries) NEVER carry
// language-internal leak strings:
//
//   - "exit status N"           — subprocess wrapper noise
//   - "panic:"                  — Go runtime panic surface
//   - "goroutine"               — runtime stack frame language
//   - "/usr/local/go/" etc.     — absolute Go install paths from wraps
//
// The existing journal_search classifier (tools_readonly.go) covers
// the subprocess case; these tests lock the same discipline for the
// catalog/discovery surface. New Tools should add a row here when
// they materialise an error path.

// browse_catalog with a non-existent catalogDir must produce a Note
// the agent can render to the operator without leaking absolute
// filesystem error chains.
func TestBrowseCatalog_MissingCatalogDirErrorIsAgentSafe(t *testing.T) {
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	// Deliberately point at a path that cannot exist; buildCatalogManifest
	// will fail on the first os.ReadDir.
	registerBrowseCatalog(srv, "/nonexistent/powerlab/catalog/path/for/test")
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	got, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "browse_catalog",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	// Materialise the response as plain text — the Note field is
	// where the agent reads "what went wrong" prose.
	var rendered strings.Builder
	for _, c := range got.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			rendered.WriteString(tc.Text)
		}
	}
	body := rendered.String()

	for _, leak := range agentForbiddenLeakStrings() {
		if strings.Contains(body, leak) {
			t.Errorf("browse_catalog error path leaks %q to the agent surface; got: %s", leak, body)
		}
	}
}

// agentForbiddenLeakStrings is the canonical list of substrings that
// must never appear in any Tool's agent-visible response. Add to this
// list rather than per-test ad-hoc — the contract should be central.
func agentForbiddenLeakStrings() []string {
	return []string{
		"exit status",          // subprocess wrap (journal_search regression class)
		"panic:",               // Go runtime panic
		"goroutine",            // Go runtime stack frame language
		"/usr/local/go/",       // Go install path from a wrapped chain
		"runtime.gopanic",      // panic recovery internals
		"runtime/debug.Stack",  // stack-trace dump
	}
}
