package journal

import (
	"context"
	"slices"
	"strconv"
	"testing"
	"time"
)

// BuildArgs is pure — it must always request JSON (the parser depends on
// it) and, with no unit, scope to PowerLab units only (an agent must not
// be able to read arbitrary system journals through this resource).
func TestBuildArgs_DefaultsToPowerlabWildcard(t *testing.T) {
	got := BuildArgs(Query{})
	if !slices.Contains(got, "json") {
		t.Fatalf("args %v missing `-o json`", got)
	}
	if !argPair(got, "-u", "powerlab-*.service") {
		t.Fatalf("args %v: default unit scope must be powerlab-*.service", got)
	}
}

// The unit param is agent-supplied, so it's canonicalised to a PowerLab
// unit no matter how it arrives — "core", "powerlab-core", or
// "powerlab-core.service" all resolve to the same scoped unit, and a
// non-PowerLab name like "sshd" still gets the powerlab- prefix (no
// escaping to other services' logs).
func TestBuildArgs_UnitIsCanonicalisedAndScoped(t *testing.T) {
	for _, in := range []string{"core", "powerlab-core", "powerlab-core.service"} {
		got := BuildArgs(Query{Unit: in})
		if !argPair(got, "-u", "powerlab-core.service") {
			t.Fatalf("unit %q → args %v; want -u powerlab-core.service", in, got)
		}
	}
	got := BuildArgs(Query{Unit: "sshd"})
	if !argPair(got, "-u", "powerlab-sshd.service") {
		t.Fatalf("non-powerlab unit must stay scoped: %v", got)
	}
}

func TestBuildArgs_LinesSincePriority(t *testing.T) {
	got := BuildArgs(Query{Lines: 50, Since: "1h", Priority: "err"})
	if !argPair(got, "-n", "50") || !argPair(got, "--since", "1h") || !argPair(got, "-p", "err") {
		t.Fatalf("args %v missing one of -n 50 / --since 1h / -p err", got)
	}
}

// With no lines requested, journalctl would otherwise dump the entire
// journal since boot — a token/memory blowout that violates the
// "query, not dump" principle. BuildArgs must apply a default cap.
func TestBuildArgs_DefaultsToLineCapWhenUnset(t *testing.T) {
	got := BuildArgs(Query{}) // Lines unset
	if !argPair(got, "-n", strconv.Itoa(defaultLines)) {
		t.Fatalf("args %v: with no lines requested, want a default -n %d cap", got, defaultLines)
	}
}

// An absurd lines value must be clamped so a single request can't pull
// the whole journal into memory / the agent's context.
func TestBuildArgs_ClampsExcessiveLines(t *testing.T) {
	got := BuildArgs(Query{Lines: 10_000_000})
	if !argPair(got, "-n", strconv.Itoa(maxLines)) {
		t.Fatalf("args %v: an excessive lines request must clamp to -n %d", got, maxLines)
	}
}

func TestParse_NDJSON(t *testing.T) {
	const micros = int64(1716854400123456)
	line := `{"__REALTIME_TIMESTAMP":"1716854400123456","_SYSTEMD_UNIT":"powerlab-core.service","PRIORITY":"6","MESSAGE":"started"}`
	blank := "\n"
	bad := "not json\n"
	body := line + "\n" + blank + bad +
		`{"__REALTIME_TIMESTAMP":"1716854401000000","_SYSTEMD_UNIT":"powerlab-gateway.service","PRIORITY":"3","MESSAGE":"boom"}` + "\n"

	entries, err := Parse([]byte(body))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// blank + malformed lines are skipped, the two valid lines survive.
	if len(entries) != 2 {
		t.Fatalf("got %d entries; want 2 (blank/malformed must be skipped)", len(entries))
	}
	want := time.UnixMicro(micros).UTC().Format(time.RFC3339Nano)
	if entries[0].Time != want {
		t.Fatalf("Time = %q; want %q (from __REALTIME_TIMESTAMP micros)", entries[0].Time, want)
	}
	if entries[0].Unit != "powerlab-core.service" || entries[0].Priority != 6 || entries[0].Message != "started" {
		t.Fatalf("entry[0] = %+v; want unit/priority/message core/6/started", entries[0])
	}
	if entries[1].Priority != 3 {
		t.Fatalf("entry[1].Priority = %d; want 3", entries[1].Priority)
	}
}

// An entry with no PRIORITY field must default to info (6), not 0 —
// 0 is "emergency", and inflating every unprioritised line to emergency
// would wreck an agent's triage.
func TestParse_MissingPriorityDefaultsToInfo(t *testing.T) {
	line := `{"__REALTIME_TIMESTAMP":"1716854400000000","_SYSTEMD_UNIT":"powerlab-core.service","MESSAGE":"no priority field"}`
	entries, err := Parse([]byte(line + "\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries; want 1", len(entries))
	}
	if entries[0].Priority != 6 {
		t.Fatalf("missing PRIORITY → %d; want 6 (info), not 0 (emergency)", entries[0].Priority)
	}
}

// Read wires BuildArgs → the injected runner → Parse, so it's testable
// without a real journalctl. It must pass the built args to the runner
// and return the parsed entries.
func TestRead_UsesRunnerAndParses(t *testing.T) {
	var gotArgs []string
	run := func(_ context.Context, args []string) ([]byte, error) {
		gotArgs = args
		return []byte(`{"__REALTIME_TIMESTAMP":"1716854400000000","_SYSTEMD_UNIT":"powerlab-core.service","PRIORITY":"6","MESSAGE":"hi"}` + "\n"), nil
	}

	entries, err := Read(context.Background(), run, Query{Unit: "core", Lines: 10})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(entries) != 1 || entries[0].Message != "hi" {
		t.Fatalf("entries = %+v; want one entry 'hi'", entries)
	}
	if !argPair(gotArgs, "-u", "powerlab-core.service") || !argPair(gotArgs, "-n", "10") {
		t.Fatalf("runner got args %v; want the BuildArgs output", gotArgs)
	}
}

// argPair reports whether args contains flag immediately followed by val.
func argPair(args []string, flag, val string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == val {
			return true
		}
	}
	return false
}
