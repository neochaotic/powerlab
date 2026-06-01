package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/neochaotic/powerlab/backend/powerlab-mcp/journal"
)

// THE most important gate test for ADR-0049: when EnableSensitiveTier
// is false, neither journal://system/auth NOR journal://system/failures
// may appear in resources/list — an agent that doesn't see them has no
// URI to address and the surface effectively doesn't exist. Same
// pattern as TestDestructiveTools_NotAdvertisedWhenFlagFalse.
func TestSensitiveJournal_NotAdvertisedWhenFlagFalse(t *testing.T) {
	srv := newMCPServer(BuildInfo{Version: "test"},
		resourcesConfig{procRoot: t.TempDir(), enableSensitiveTier: false},
		fixtureJournalRunner(""))
	cs := connectInProcess(t, srv)

	list, err := cs.ListResources(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	for _, r := range list.Resources {
		if r.URI == "journal://system/auth" || r.URI == "journal://system/failures" {
			t.Fatalf("sensitive resource %q advertised when EnableSensitiveTier=false (gate broken — ADR-0049 says NOT registered)", r.URI)
		}
	}
}

// And reading those URIs when the gate is off MUST return an error /
// not-found from the MCP layer — the agent's ReadResource call must
// not silently succeed against a URI the server doesn't expose.
func TestSensitiveJournal_ReadFailsWhenFlagFalse(t *testing.T) {
	srv := newMCPServer(BuildInfo{Version: "test"},
		resourcesConfig{procRoot: t.TempDir(), enableSensitiveTier: false},
		fixtureJournalRunner(""))
	cs := connectInProcess(t, srv)

	for _, uri := range []string{"journal://system/auth", "journal://system/failures"} {
		_, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: uri})
		if err == nil {
			t.Fatalf("ReadResource(%s) succeeded with EnableSensitiveTier=false; want error (resource not registered)", uri)
		}
	}
}

// With the flag true: both resources appear AND read end-to-end,
// returning the parsed wire shape {ts, unit, hostname, message}.
func TestSensitiveJournal_AdvertisedAndReadableWhenFlagTrue(t *testing.T) {
	// Fixture: one ssh.service line, one sudo line — exercises both
	// _SYSTEMD_UNIT and the _COMM fallback path.
	out := `{"__REALTIME_TIMESTAMP":"1716854400000000","_SYSTEMD_UNIT":"ssh.service","_HOSTNAME":"box","MESSAGE":"Failed password for invalid user root from 198.51.100.42"}` + "\n" +
		`{"__REALTIME_TIMESTAMP":"1716854401000000","_COMM":"sudo","_HOSTNAME":"box","MESSAGE":"alice : TTY=pts/0 ; PWD=/home/alice ; USER=root ; COMMAND=/bin/ls"}` + "\n"

	srv := newMCPServer(BuildInfo{Version: "test"},
		resourcesConfig{procRoot: t.TempDir(), enableSensitiveTier: true},
		fixtureJournalRunner(out))
	cs := connectInProcess(t, srv)

	// Both URIs advertised.
	list, err := cs.ListResources(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	advertised := map[string]bool{}
	for _, r := range list.Resources {
		advertised[r.URI] = true
	}
	for _, want := range []string{"journal://system/auth", "journal://system/failures"} {
		if !advertised[want] {
			t.Errorf("EnableSensitiveTier=true: resource %s missing from resources/list", want)
		}
	}

	// Both read AND parse to []SystemEntry with the canonical wire
	// keys. The wire-shape test in journal/ pins the field names;
	// this asserts the resource surface delivers them too.
	for _, uri := range []string{"journal://system/auth", "journal://system/failures"} {
		res, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: uri})
		if err != nil {
			t.Fatalf("ReadResource(%s): %v", uri, err)
		}
		if len(res.Contents) == 0 {
			t.Fatalf("ReadResource(%s) returned no contents", uri)
		}
		// Defensive: the raw payload must NEVER contain the forbidden
		// tokens, even after MCP wrapping. Belt + braces.
		low := strings.ToLower(res.Contents[0].Text)
		for _, bad := range []string{"cmdline", "_pid", "audit_session"} {
			if strings.Contains(low, bad) {
				t.Errorf("ReadResource(%s) payload leaked forbidden token %q: %s", uri, bad, res.Contents[0].Text)
			}
		}
		var entries []journal.SystemEntry
		if err := json.Unmarshal([]byte(res.Contents[0].Text), &entries); err != nil {
			t.Fatalf("ReadResource(%s) payload not []SystemEntry: %v (%q)", uri, err, res.Contents[0].Text)
		}
		if len(entries) != 2 {
			t.Fatalf("ReadResource(%s) = %d entries; want 2", uri, len(entries))
		}
		if entries[0].Unit != "ssh.service" || entries[1].Unit != "sudo" {
			t.Fatalf("ReadResource(%s) units = %q,%q; want ssh.service,sudo (sudo via _COMM fallback)", uri, entries[0].Unit, entries[1].Unit)
		}
	}
}

// The failures URI must add `-p err..warning` to the journalctl
// argv — that's the WHOLE point of the variant. Confirmed by
// capturing args via the runner. The literal MUST be err..warning
// (3..4, low-to-high), not the reversed warning..error spelling that
// shipped originally and surfaced as `exit status 1` from journalctl
// — see issue #639 and journal/system_test.go::
// TestBuildSystemArgs_FailuresPriorityRangeIsValid.
func TestSensitiveJournal_FailuresAddsPriorityFilter(t *testing.T) {
	var seenArgs [][]string
	runner := func(_ context.Context, args []string) ([]byte, error) {
		// Defensive copy so the assertion sees the args at call time.
		cp := append([]string(nil), args...)
		seenArgs = append(seenArgs, cp)
		return []byte{}, nil
	}

	srv := newMCPServer(BuildInfo{Version: "test"},
		resourcesConfig{procRoot: t.TempDir(), enableSensitiveTier: true},
		runner)
	cs := connectInProcess(t, srv)

	if _, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: "journal://system/auth"}); err != nil {
		t.Fatalf("ReadResource auth: %v", err)
	}
	if _, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: "journal://system/failures"}); err != nil {
		t.Fatalf("ReadResource failures: %v", err)
	}

	if len(seenArgs) != 2 {
		t.Fatalf("got %d runner calls; want 2 (auth + failures)", len(seenArgs))
	}
	if containsPair(seenArgs[0], "-p", "err..warning") {
		t.Errorf("auth call args %v should NOT carry a -p priority filter", seenArgs[0])
	}
	if !containsPair(seenArgs[1], "-p", "err..warning") {
		t.Errorf("failures call args %v must carry -p err..warning (#639: reversed warning..error breaks journalctl)", seenArgs[1])
	}
}

// The lines query param must round-trip from URI → BuildSystemArgs.
// Tests at the resource layer: the agent's URL becomes the runner's
// argv. Above-ceiling values must clamp (defence at the resource edge
// in case anything skipped journal's clamp).
func TestSensitiveJournal_LinesQueryRoundTrips(t *testing.T) {
	var seenArgs []string
	runner := func(_ context.Context, args []string) ([]byte, error) {
		seenArgs = args
		return []byte{}, nil
	}

	srv := newMCPServer(BuildInfo{Version: "test"},
		resourcesConfig{procRoot: t.TempDir(), enableSensitiveTier: true},
		runner)
	cs := connectInProcess(t, srv)

	cases := []struct {
		uri      string
		wantN    string
		describe string
	}{
		{"journal://system/auth?lines=50", "50", "in-range value passes through"},
		{"journal://system/auth?lines=10000", "500", "above-ceiling clamps to 500"},
		{"journal://system/auth?lines=-1", "100", "negative defaults to 100"},
		{"journal://system/auth?lines=0", "100", "zero defaults to 100"},
		{"journal://system/auth", "100", "missing param defaults to 100"},
	}
	for _, tc := range cases {
		t.Run(tc.describe, func(t *testing.T) {
			if _, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: tc.uri}); err != nil {
				t.Fatalf("ReadResource(%s): %v", tc.uri, err)
			}
			if !containsPair(seenArgs, "-n", tc.wantN) {
				t.Fatalf("uri=%s args=%v; want -n %s", tc.uri, seenArgs, tc.wantN)
			}
		})
	}
}

// containsPair reports whether args holds flag immediately followed by val.
func containsPair(args []string, flag, val string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == val {
			return true
		}
	}
	return false
}
