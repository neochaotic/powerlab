package main

import (
	"bytes"
	"strings"
	"testing"
)

// Journal subcommand tests. The handler reads `journalctl -o json`
// output (mocked via io.Reader in tests; real journalctl in
// production) line-by-line, parses each via parseJournalEntry, and
// formats to stdout per the --json flag.
//
// Coverage target per ADR-0027 + memory feedback_release_coverage_gate:
// >=95% line coverage, every behaviour locked.

func TestRunJournal_ParsesAndFormatsEachLine(t *testing.T) {
	input := []byte(`{"_SYSTEMD_UNIT":"powerlab-core.service","__REALTIME_TIMESTAMP":"1715619600000000","PRIORITY":"3","MESSAGE":"first line"}
{"_SYSTEMD_UNIT":"powerlab-core.service","__REALTIME_TIMESTAMP":"1715619601000000","PRIORITY":"6","MESSAGE":"second line"}
`)
	out := &bytes.Buffer{}
	opts := JournalOpts{Color: false, JSON: false}
	if err := runJournal(bytes.NewReader(input), out, opts); err != nil {
		t.Fatalf("runJournal: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "ERROR") || !strings.Contains(text, "first line") {
		t.Errorf("expected ERROR + 'first line' in output, got: %q", text)
	}
	if !strings.Contains(text, "INFO") || !strings.Contains(text, "second line") {
		t.Errorf("expected INFO + 'second line' in output, got: %q", text)
	}
	// Exactly 2 newlines (one per line + trailing \n on each).
	if strings.Count(text, "\n") != 2 {
		t.Errorf("expected 2 newlines, got %d: %q", strings.Count(text, "\n"), text)
	}
}

func TestRunJournal_JSONMode_EmitsNdjson(t *testing.T) {
	input := []byte(`{"_SYSTEMD_UNIT":"powerlab-core.service","__REALTIME_TIMESTAMP":"1715619600000000","PRIORITY":"3","MESSAGE":"errored"}
`)
	out := &bytes.Buffer{}
	if err := runJournal(bytes.NewReader(input), out, JournalOpts{JSON: true}); err != nil {
		t.Fatalf("runJournal: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, `"level":"ERROR"`) || !strings.Contains(text, `"message":"errored"`) {
		t.Errorf("expected JSON shape with level + message, got: %q", text)
	}
}

func TestRunJournal_SkipsMalformedLines(t *testing.T) {
	input := []byte(`{"_SYSTEMD_UNIT":"powerlab-core.service","__REALTIME_TIMESTAMP":"1715619600000000","PRIORITY":"6","MESSAGE":"good 1"}
this is not json
{"_SYSTEMD_UNIT":"powerlab-core.service","__REALTIME_TIMESTAMP":"1715619601000000","PRIORITY":"6","MESSAGE":"good 2"}
`)
	out := &bytes.Buffer{}
	if err := runJournal(bytes.NewReader(input), out, JournalOpts{}); err != nil {
		t.Fatalf("runJournal: %v", err)
	}
	text := out.String()
	// Both good lines should be emitted. Malformed line is silently
	// dropped (journalctl shouldn't emit malformed records; if it
	// does, we'd rather print every parseable line than abort).
	if !strings.Contains(text, "good 1") || !strings.Contains(text, "good 2") {
		t.Errorf("expected both good lines in output, got: %q", text)
	}
}

func TestRunJournal_SkipsBlankLines(t *testing.T) {
	input := []byte(`{"_SYSTEMD_UNIT":"powerlab-core.service","__REALTIME_TIMESTAMP":"1715619600000000","PRIORITY":"6","MESSAGE":"only line"}

`)
	out := &bytes.Buffer{}
	if err := runJournal(bytes.NewReader(input), out, JournalOpts{}); err != nil {
		t.Fatalf("runJournal: %v", err)
	}
	if strings.Count(out.String(), "\n") != 1 {
		t.Errorf("expected exactly 1 output line, got: %q", out.String())
	}
}

func TestRunJournal_EmptyInput(t *testing.T) {
	out := &bytes.Buffer{}
	if err := runJournal(bytes.NewReader(nil), out, JournalOpts{}); err != nil {
		t.Fatalf("empty input should not error: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("empty input should produce no output, got: %q", out.String())
	}
}

func TestRunJournal_ColorMode_EmitsANSI(t *testing.T) {
	input := []byte(`{"_SYSTEMD_UNIT":"powerlab-core.service","__REALTIME_TIMESTAMP":"1715619600000000","PRIORITY":"3","MESSAGE":"red error"}
`)
	out := &bytes.Buffer{}
	if err := runJournal(bytes.NewReader(input), out, JournalOpts{Color: true}); err != nil {
		t.Fatalf("runJournal: %v", err)
	}
	if !strings.Contains(out.String(), "\x1b[31m") {
		t.Errorf("color mode should emit red ANSI for ERROR, got: %q", out.String())
	}
}

func TestBuildJournalCmd_DefaultsToFollow(t *testing.T) {
	// Building the journalctl command line is pure — the actual exec
	// is integration-tested elsewhere. Verifies argument shape so a
	// refactor doesn't silently change behaviour.
	args := buildJournalArgs(JournalArgs{Follow: true})
	if !contains(args, "-f") {
		t.Errorf("--follow should add -f flag, got: %v", args)
	}
	if !contains(args, "-o") || !contains(args, "json") {
		t.Errorf("expected -o json flag pair, got: %v", args)
	}
}

func TestBuildJournalCmd_ServiceFilter(t *testing.T) {
	args := buildJournalArgs(JournalArgs{Service: "core"})
	if !contains(args, "-u") {
		t.Errorf("--service should add -u flag, got: %v", args)
	}
	if !contains(args, "powerlab-core.service") {
		t.Errorf("--service core should resolve to powerlab-core.service, got: %v", args)
	}
}

func TestBuildJournalCmd_DefaultMatchesAllPowerlabUnits(t *testing.T) {
	args := buildJournalArgs(JournalArgs{})
	// With no service, default to a wildcard that catches all
	// powerlab-* units.
	if !contains(args, "powerlab-*.service") && !contains(args, "_SYSTEMD_UNIT=powerlab") {
		t.Errorf("default should match all powerlab-* units, got: %v", args)
	}
}

func TestBuildJournalCmd_LinesLimit(t *testing.T) {
	args := buildJournalArgs(JournalArgs{Lines: 50})
	if !contains(args, "-n") || !contains(args, "50") {
		t.Errorf("--lines should add -n N, got: %v", args)
	}
}

// contains reports whether `needle` appears in the args slice.
func contains(args []string, needle string) bool {
	for _, a := range args {
		if a == needle {
			return true
		}
	}
	return false
}
