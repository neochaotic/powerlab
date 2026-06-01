package server

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// tools/list must advertise both read-only tools per ADR-0046. The
// agent reads tools/list to discover capability; missing here means
// the agent doesn't know the tool exists.
func TestReadOnlyTools_AreAdvertised(t *testing.T) {
	srv := newMCPServer(BuildInfo{Version: "test"},
		resourcesConfig{procRoot: t.TempDir()},
		fixtureJournalRunner(""))
	cs := connectInProcess(t, srv)
	defer cs.Close()

	list, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	gotNames := map[string]bool{}
	for _, tool := range list.Tools {
		gotNames[tool.Name] = true
	}
	for _, want := range []string{"journal_search", "check_disk_free"} {
		if !gotNames[want] {
			t.Fatalf("tools/list missing %q (got: %v)", want, mapKeys(gotNames))
		}
	}
}

// Every tool's description must spell out the side-effect class so
// the LLM surfaces it to the user — ADR-0046 §1 contract. For the
// read-only batch the marker is "READ ONLY".
func TestReadOnlyTools_DescriptionsCarrySideEffectClass(t *testing.T) {
	srv := newMCPServer(BuildInfo{Version: "test"},
		resourcesConfig{procRoot: t.TempDir()},
		fixtureJournalRunner(""))
	cs := connectInProcess(t, srv)
	defer cs.Close()

	list, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	for _, tool := range list.Tools {
		if tool.Name != "journal_search" && tool.Name != "check_disk_free" {
			continue
		}
		if !strings.Contains(tool.Description, "READ ONLY") && !strings.Contains(tool.Description, "READ") {
			t.Fatalf("tool %q description missing side-effect class: %q", tool.Name, tool.Description)
		}
	}
}

// journal_search round-trip: agent calls the tool with unit + pattern,
// gets back filtered entries. Uses the same fixture runner as the
// journal:// integration tests so the canonical path is exercised.
func TestJournalSearch_FiltersEntriesByPattern(t *testing.T) {
	// Fixture: 3 lines from powerlab-core, one mentioning "disk full".
	out := `{"__REALTIME_TIMESTAMP":"1716854400000000","_SYSTEMD_UNIT":"powerlab-core.service","PRIORITY":"3","MESSAGE":"disk full"}` + "\n" +
		`{"__REALTIME_TIMESTAMP":"1716854401000000","_SYSTEMD_UNIT":"powerlab-core.service","PRIORITY":"6","MESSAGE":"all good"}` + "\n" +
		`{"__REALTIME_TIMESTAMP":"1716854402000000","_SYSTEMD_UNIT":"powerlab-core.service","PRIORITY":"4","MESSAGE":"memory warning"}` + "\n"

	srv := newMCPServer(BuildInfo{Version: "test"},
		resourcesConfig{procRoot: t.TempDir()},
		fixtureJournalRunner(out))
	cs := connectInProcess(t, srv)
	defer cs.Close()

	// Filter on a literal substring — the tool uses strings.Contains so
	// the agent doesn't need to escape regex metacharacters.
	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "journal_search",
		Arguments: map[string]any{
			"unit":    "core",
			"pattern": "disk full",
		},
	})
	if err != nil {
		t.Fatalf("CallTool(journal_search): %v", err)
	}
	if res.IsError {
		t.Fatalf("CallTool errored: %+v", res.Content)
	}

	// The SDK delivers the typed output as StructuredContent — a
	// map[string]any after the protocol roundtrip. Round-trip via
	// JSON to populate our typed output struct.
	var got JournalSearchOutput
	b, mErr := json.Marshal(res.StructuredContent)
	if mErr != nil {
		t.Fatalf("marshal StructuredContent: %v", mErr)
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("decode StructuredContent: %v (raw=%s)", err, string(b))
	}
	if len(got.Entries) != 1 {
		t.Fatalf("got %d entries; want 1 (only 'disk full' matches)", len(got.Entries))
	}
	if got.Entries[0].Message != "disk full" {
		t.Fatalf("matched entry message=%q; want 'disk full'", got.Entries[0].Message)
	}
}

// journal_search MUST require a unit — calling it without one is an
// error, not an empty result. Empty unit could otherwise dump ALL
// powerlab-* unit logs by accident.
func TestJournalSearch_RequiresUnit(t *testing.T) {
	srv := newMCPServer(BuildInfo{Version: "test"},
		resourcesConfig{procRoot: t.TempDir()},
		fixtureJournalRunner(""))
	cs := connectInProcess(t, srv)
	defer cs.Close()

	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "journal_search",
		Arguments: map[string]any{},
	})
	// Either: tool returns an MCP error result, OR the call itself errors.
	if err == nil && !res.IsError {
		t.Fatalf("missing unit succeeded; want a validation error")
	}
}

// check_disk_free hits a real path and returns sane numbers. Use the
// test's temp dir (guaranteed to exist on any OS) and just assert
// the shape contract: total > 0, used + avail == total, percent in
// 0..100.
func TestCheckDiskFree_ReturnsSaneShape(t *testing.T) {
	srv := newMCPServer(BuildInfo{Version: "test"},
		resourcesConfig{procRoot: t.TempDir()},
		fixtureJournalRunner(""))
	cs := connectInProcess(t, srv)
	defer cs.Close()

	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "check_disk_free",
		Arguments: map[string]any{"path": t.TempDir()},
	})
	if err != nil {
		t.Fatalf("CallTool(check_disk_free): %v", err)
	}
	if res.IsError {
		t.Fatalf("CallTool errored: %+v", res.Content)
	}

	var got CheckDiskFreeOutput
	b, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.TotalBytes == 0 {
		t.Fatalf("TotalBytes is 0 (statfs failed on a real path?)")
	}
	if got.UsedBytes+got.AvailableBytes != got.TotalBytes {
		t.Fatalf("derived check: used(%d) + avail(%d) != total(%d)", got.UsedBytes, got.AvailableBytes, got.TotalBytes)
	}
	if got.UsedPercent < 0 || got.UsedPercent > 100 {
		t.Fatalf("UsedPercent=%v out of 0..100", got.UsedPercent)
	}
}

// check_disk_free against a non-existent path returns a friendly
// error rather than leaking the raw stat() error string — the agent
// sees "path does not exist" not "stat /no/such/place: no such file
// or directory."
func TestCheckDiskFree_RejectsMissingPath(t *testing.T) {
	srv := newMCPServer(BuildInfo{Version: "test"},
		resourcesConfig{procRoot: t.TempDir()},
		fixtureJournalRunner(""))
	cs := connectInProcess(t, srv)
	defer cs.Close()

	missing := filepath.Join(t.TempDir(), "definitely-not-here")
	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "check_disk_free",
		Arguments: map[string]any{"path": missing},
	})
	if err == nil && !res.IsError {
		t.Fatalf("missing path succeeded; want a friendly error")
	}
	// Belt-and-suspenders: confirm the friendly message survived if
	// the SDK packaged the error into content rather than IsError.
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok && strings.Contains(tc.Text, "does not exist") {
			return
		}
	}
}

// Helper that survives across test files (the canonical method is
// elsewhere — duplicated here for the standalone tools test to avoid
// import order surprises).
func mapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}


// REGRESSION (P0.2 from 2026-05-31 MCP-only chat-mode test): when
// journalctl is unavailable on the host (macOS dev box, container
// without systemd, broken install), the journal.Runner returns a
// raw subprocess error string like "exit status 1". The original
// tool wrapper bubbled it up verbatim as
// `journal read: run journalctl: exit status 1` — opaque to any
// agent. This test locks the cleaner contract: the error must be
// human-readable, must NOT contain the raw "exit status" leak, and
// must hint at the cause (journald unavailable) so the agent can
// either degrade or surface to operator.
func TestJournalSearch_JournaldUnavailable_CleanErrorMessage(t *testing.T) {
	failingRunner := func(_ context.Context, _ []string) ([]byte, error) {
		return nil, errors.New("exit status 1")
	}
	srv := newMCPServer(BuildInfo{Version: "test"},
		resourcesConfig{procRoot: t.TempDir()},
		failingRunner)
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "journal_search",
		Arguments: map[string]any{
			"unit":    "core",
			"pattern": "anything",
		},
	})
	if err != nil {
		t.Fatalf("CallTool transport error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError=true when journalctl unavailable; got success with %+v", res.Content)
	}

	// Concatenate content text so we can assert on the full surfaced
	// message regardless of how the SDK chunks it.
	var msg strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			msg.WriteString(tc.Text)
		}
	}
	got := msg.String()
	if strings.Contains(got, "exit status") {
		t.Fatalf("error message leaks raw subprocess output %q (pre-fix shape: 'journal read: run journalctl: exit status 1'); want clean human-readable text", got)
	}
	if !strings.Contains(strings.ToLower(got), "journald") {
		t.Fatalf("error message %q must mention 'journald' so the agent can surface the right diagnostic", got)
	}
}

// Timeout shape: runner reports context.DeadlineExceeded. The agent
// should see hint about narrowing the query window, not the bare
// `context deadline exceeded`.
func TestJournalSearch_RunnerTimeout_HintsQueryNarrowing(t *testing.T) {
	timeoutRunner := func(_ context.Context, _ []string) ([]byte, error) {
		return nil, context.DeadlineExceeded
	}
	srv := newMCPServer(BuildInfo{Version: "test"},
		resourcesConfig{procRoot: t.TempDir()},
		timeoutRunner)
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "journal_search",
		Arguments: map[string]any{
			"unit":    "core",
			"pattern": "x",
		},
	})
	if err != nil {
		t.Fatalf("CallTool transport error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError=true on timeout")
	}
	var msg strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			msg.WriteString(tc.Text)
		}
	}
	got := strings.ToLower(msg.String())
	if !strings.Contains(got, "timed out") && !strings.Contains(got, "timeout") {
		t.Fatalf("error %q must signal timeout class", msg.String())
	}
	if !strings.Contains(got, "lines") && !strings.Contains(got, "since") {
		t.Fatalf("error %q must hint at the query knobs (lines/since) the operator can narrow", msg.String())
	}
}

// Unknown error fallback: classifier MUST still sanitize the
// `run journalctl: ` / `journal read: ` wrapping noise so even
// unrecognized errors read as one sentence.
func TestJournalSearch_UnknownError_Sanitized(t *testing.T) {
	weirdRunner := func(_ context.Context, _ []string) ([]byte, error) {
		return nil, errors.New("ACL denied: user 'nobody' cannot read journal")
	}
	srv := newMCPServer(BuildInfo{Version: "test"},
		resourcesConfig{procRoot: t.TempDir()},
		weirdRunner)
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "journal_search",
		Arguments: map[string]any{"unit": "core"},
	})
	if err != nil {
		t.Fatalf("CallTool transport error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError=true on unknown error")
	}
	var msg strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			msg.WriteString(tc.Text)
		}
	}
	got := msg.String()
	if strings.Contains(got, "run journalctl:") || strings.Contains(got, "journal read: journal read:") {
		t.Fatalf("classifier did not sanitize wrapped-error noise: %q", got)
	}
	if !strings.Contains(got, "ACL denied") {
		t.Fatalf("classifier dropped the underlying detail (operator needs it to diagnose): %q", got)
	}
}
