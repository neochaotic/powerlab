package journal

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// BuildSystemArgs is pure — it always requests JSON (the parser depends
// on it) and ALWAYS pins the host journal sources to the auth set:
// ssh.service, sshd.service, sudo, su. These selectors are LITERAL,
// fixed in code, never agent-supplied — per ADR-0049 the operator opts
// into the whole tier and the resource code chooses what to expose.
func TestBuildSystemArgs_AlwaysScopesToAuthSources(t *testing.T) {
	got := BuildSystemArgs(SystemQuery{})
	if !slices.Contains(got, "-o") || !slices.Contains(got, "json") {
		t.Fatalf("args %v missing `-o json`", got)
	}
	wantSelectors := []string{
		"_SYSTEMD_UNIT=ssh.service",
		"_SYSTEMD_UNIT=sshd.service",
		"_COMM=sudo",
		"_COMM=su",
	}
	for _, sel := range wantSelectors {
		if !slices.Contains(got, sel) {
			t.Fatalf("args %v missing pinned auth selector %q (ADR-0049 — selectors are fixed in code, never agent-supplied)", got, sel)
		}
	}
}

// With no lines requested, journalctl would dump the entire matching
// journal — a sensitive-tier exfil amplifier if a token is compromised.
// BuildSystemArgs must apply the default cap (ADR-0049: lines default
// 100, ceiling 500; tighter than PowerLab journal's 200/2000 because
// each sensitive entry carries more leakage per record).
func TestBuildSystemArgs_DefaultsToLineCapWhenUnset(t *testing.T) {
	got := BuildSystemArgs(SystemQuery{})
	if !argPair(got, "-n", strconv.Itoa(defaultSystemLines)) {
		t.Fatalf("args %v: with no lines requested, want default -n %d cap (ADR-0049)", got, defaultSystemLines)
	}
}

// Edge case: lines=0 → default 100. Same handling as Query.Lines.
func TestBuildSystemArgs_ZeroLinesUsesDefault(t *testing.T) {
	got := BuildSystemArgs(SystemQuery{Lines: 0})
	if !argPair(got, "-n", strconv.Itoa(defaultSystemLines)) {
		t.Fatalf("args %v: Lines=0 must map to default %d, not 0", got, defaultSystemLines)
	}
}

// Edge case: negative lines → default 100. An agent sending -1 (or any
// negative integer parsed from a query string) must not blow past the
// cap or pass through to journalctl as a real negative.
func TestBuildSystemArgs_NegativeLinesUsesDefault(t *testing.T) {
	got := BuildSystemArgs(SystemQuery{Lines: -1})
	if !argPair(got, "-n", strconv.Itoa(defaultSystemLines)) {
		t.Fatalf("args %v: Lines=-1 must map to default %d, not negative", got, defaultSystemLines)
	}
}

// Edge case: an absurd lines value must be clamped to maxSystemLines.
// Per ADR-0049, the ceiling is 500 (tighter than the PowerLab journal's
// 2000) — a single request must not pull thousands of auth lines.
func TestBuildSystemArgs_ClampsExcessiveLines(t *testing.T) {
	got := BuildSystemArgs(SystemQuery{Lines: 10_000})
	if !argPair(got, "-n", strconv.Itoa(maxSystemLines)) {
		t.Fatalf("args %v: an excessive lines request must clamp to -n %d (ADR-0049 ceiling)", got, maxSystemLines)
	}
}

// Exact boundary: requesting the ceiling itself stays at the ceiling
// (no off-by-one demotion to defaults).
func TestBuildSystemArgs_AtCeilingStays(t *testing.T) {
	got := BuildSystemArgs(SystemQuery{Lines: maxSystemLines})
	if !argPair(got, "-n", strconv.Itoa(maxSystemLines)) {
		t.Fatalf("args %v: Lines=%d must stay at ceiling, not clamp or default", got, maxSystemLines)
	}
}

// Since is forwarded verbatim — journalctl's own --since parser handles
// the format strings ("1 hour ago", "2026-05-31", etc.).
func TestBuildSystemArgs_SinceForwarded(t *testing.T) {
	got := BuildSystemArgs(SystemQuery{Since: "1 hour ago"})
	if !argPair(got, "--since", "1 hour ago") {
		t.Fatalf("args %v missing --since '1 hour ago'", got)
	}
}

// The "failures" variant restricts to PRIORITY warning..error so the
// agent's auth-triage path doesn't have to page through every success
// line. Implemented by setting Failures=true on the query.
func TestBuildSystemArgs_FailuresVariantAppliesPriorityFilter(t *testing.T) {
	got := BuildSystemArgs(SystemQuery{Failures: true})
	if !argPair(got, "-p", "warning..error") {
		t.Fatalf("args %v missing -p warning..error (Failures=true should restrict to warn..err range)", got)
	}
	// The auth variant must NOT have a priority filter (it returns
	// every auth-related entry).
	auth := BuildSystemArgs(SystemQuery{Failures: false})
	if argPair(auth, "-p", "warning..error") {
		t.Fatalf("auth variant args %v should NOT carry -p warning..error", auth)
	}
}

// --no-pager prevents journalctl from invoking less when stdout is a
// terminal — critical for the exec-capture path used by the runner.
func TestBuildSystemArgs_NoPager(t *testing.T) {
	got := BuildSystemArgs(SystemQuery{})
	if !slices.Contains(got, "--no-pager") {
		t.Fatalf("args %v missing --no-pager", got)
	}
}

// SECURITY LOCK — SystemEntry must NEVER expose _PID, _CMDLINE, or
// _AUDIT_SESSION. The WHOLE POINT of this resource family is the wire
// shape: `_CMDLINE` would surface `sudo --password=hunter2` argvs;
// `_PID` is operator noise; `_AUDIT_SESSION` is kernel internal noise.
// Same defensive lock as backend/core/model/processes_test.go's
// TestProcessSummary_NeverLeaksCmdline (ADR-0049 calls it out by name).
//
// If a future refactor adds Cmdline/PID/AuditSession to SystemEntry,
// this test fails LOUD before the leak ships.
func TestSystemEntry_NeverLeaksForbiddenFields(t *testing.T) {
	forbidden := []string{
		"cmdline", "args", "argv", "commandline", "command_line",
		"pid", "_pid",
		"audit_session", "_audit_session", "auditsession",
		"selinux", "_selinux_context", "selinuxcontext",
	}

	rt := reflect.TypeOf(SystemEntry{})
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		jsonTag := strings.Split(f.Tag.Get("json"), ",")[0]
		for _, bad := range forbidden {
			if strings.EqualFold(f.Name, bad) {
				t.Errorf("SystemEntry has forbidden field %q — ADR-0049 wire shape promise is {ts, unit, hostname, message} ONLY (no _PID, no _CMDLINE, no _AUDIT_SESSION)", f.Name)
			}
			if strings.EqualFold(jsonTag, bad) {
				t.Errorf("SystemEntry field %q has forbidden JSON tag %q (would leak via wire)", f.Name, jsonTag)
			}
		}
	}

	// Belt + braces: marshal an entry with all fields populated, the
	// forbidden tokens MUST NOT appear in the wire output.
	out, err := json.Marshal(SystemEntry{
		Time:     "2026-05-31T12:34:56Z",
		Unit:     "ssh.service",
		Hostname: "powerlab-host",
		Message:  "Failed password for invalid user root from 198.51.100.42",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	low := strings.ToLower(string(out))
	for _, bad := range forbidden {
		if strings.Contains(low, bad) {
			t.Errorf("SystemEntry JSON contains forbidden token %q: %s", bad, string(out))
		}
	}
}

// SystemEntry's wire keys are the agent's parse contract — locked.
func TestSystemEntry_StableWireKeys(t *testing.T) {
	out, err := json.Marshal(SystemEntry{Time: "t", Unit: "u", Hostname: "h", Message: "m"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{`"ts"`, `"unit"`, `"hostname"`, `"message"`} {
		if !strings.Contains(string(out), key) {
			t.Errorf("SystemEntry JSON missing wire key %s: %s", key, string(out))
		}
	}
}

// ReadSystem wires BuildSystemArgs → injected runner → ParseSystem so
// it's testable without journalctl. Confirms the auth-variant pipeline
// works end-to-end against a fixture runner.
func TestReadSystem_UsesRunnerAndParses(t *testing.T) {
	var gotArgs []string
	run := func(_ context.Context, args []string) ([]byte, error) {
		gotArgs = args
		return []byte(`{"__REALTIME_TIMESTAMP":"1716854400000000","_SYSTEMD_UNIT":"ssh.service","_HOSTNAME":"box","MESSAGE":"sshd accepted"}` + "\n"), nil
	}

	entries, err := ReadSystem(context.Background(), run, SystemQuery{Lines: 10})
	if err != nil {
		t.Fatalf("ReadSystem: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %+v; want 1", entries)
	}
	if entries[0].Unit != "ssh.service" || entries[0].Hostname != "box" || entries[0].Message != "sshd accepted" {
		t.Fatalf("entry[0] = %+v; want unit=ssh.service hostname=box message=sshd accepted", entries[0])
	}
	if !argPair(gotArgs, "-n", "10") {
		t.Fatalf("runner got args %v; want -n 10 forwarded from query", gotArgs)
	}
}

// ParseSystem must skip blank + malformed lines (same defensive shape
// as Parse) rather than abort — log rotation gaps and non-text MESSAGE
// records must not poison the read.
func TestParseSystem_SkipsBlankAndMalformed(t *testing.T) {
	body := `{"__REALTIME_TIMESTAMP":"1716854400000000","_SYSTEMD_UNIT":"ssh.service","_HOSTNAME":"box","MESSAGE":"a"}` + "\n" +
		"\n" +
		"not json\n" +
		`{"__REALTIME_TIMESTAMP":"1716854401000000","_SYSTEMD_UNIT":"sshd.service","_HOSTNAME":"box","MESSAGE":"b"}` + "\n"

	entries, err := ParseSystem([]byte(body))
	if err != nil {
		t.Fatalf("ParseSystem: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d; want 2 (blank/bad lines must be skipped)", len(entries))
	}
}

// When _SYSTEMD_UNIT is missing (which happens for journal records
// produced via _COMM only — sudo logs typically arrive that way), the
// unit field falls back to _COMM so the agent still has SOMETHING
// useful in the unit column.
func TestParseSystem_UnitFallsBackToComm(t *testing.T) {
	body := `{"__REALTIME_TIMESTAMP":"1716854400000000","_COMM":"sudo","_HOSTNAME":"box","MESSAGE":"alice : TTY=pts/0"}` + "\n"
	entries, err := ParseSystem([]byte(body))
	if err != nil {
		t.Fatalf("ParseSystem: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d; want 1", len(entries))
	}
	if entries[0].Unit != "sudo" {
		t.Fatalf("entries[0].Unit = %q; want fallback to _COMM 'sudo'", entries[0].Unit)
	}
}
