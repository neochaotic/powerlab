// system.go — host-level auth journal reader for the sensitive
// sysadmin tier (ADR-0049).
//
// Unlike journal.go (which scopes every read to powerlab-*.service),
// this surface deliberately reads the HOST journal — ssh.service,
// sshd.service, sudo, su. The selectors are LITERAL strings fixed in
// this file; the agent supplies only the bounded `lines`/`since` and
// the variant choice (auth vs failures). There is no agent path to
// "request a different unit"; the whole tier-vs-no-tier decision lives
// in mcp.conf's EnableSensitiveTier knob (default false; not registered
// when off — same gate shape as EnableDestructiveTools per ADR-0046).
//
// Wire shape: {ts, unit, hostname, message}. _PID, _CMDLINE, and
// _AUDIT_SESSION are deliberately omitted. _CMDLINE in particular is
// the same security promise as backend/core/model::ProcessSummary's
// name-only rule — argv routinely carries secrets (passwords as flags,
// signed URLs, JWT-bearing env expansions). The reflect+JSON test in
// system_test.go::TestSystemEntry_NeverLeaksForbiddenFields locks the
// shape so a future refactor adding any of those fields fails LOUD
// before the leak ships.
//
// As with journal.go, journalctl is exec'd as a literal argv (no
// shell interpolation, no fmt.Sprintf into a command string), so the
// fixed selectors here are not an injection surface.
package journal

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

// Bounded read window for the sensitive tier — tighter than the
// PowerLab-units journal (200 default / 2000 max). Per ADR-0049 each
// sensitive entry carries more leakage per record, so the per-request
// blast radius if a JWT is compromised stays small.
const (
	defaultSystemLines = 100
	maxSystemLines     = 500
)

// authSelectors are the FIXED journalctl filters that scope reads to
// the auth-relevant subset of the host journal. Literal, never agent-
// supplied — see package doc.
//
// `_SYSTEMD_UNIT=ssh.service` covers Debian/Ubuntu; `=sshd.service`
// covers Fedora/Arch/CentOS. The `+` between the two field groups is
// CRITICAL: journalctl ORs matchers WITHIN the same field but ANDs
// ACROSS different fields. Without the `+`, the filter becomes
//   (_SYSTEMD_UNIT=ssh.service OR _SYSTEMD_UNIT=sshd.service) AND
//   (_COMM=sudo OR _COMM=su)
// which is impossible (sudo records never carry _SYSTEMD_UNIT=ssh),
// so the resource returned [] on every host until 2026-06-01.
// `+` reverses the join to OR between the groups. See
// system_test.go::TestBuildSystemArgs_HasOrSeparatorBetweenFieldGroups
// for the regression lock.
var authSelectors = []string{
	"_SYSTEMD_UNIT=ssh.service",
	"_SYSTEMD_UNIT=sshd.service",
	"+",
	"_COMM=sudo",
	"_COMM=su",
}

// SystemQuery is the agent-facing filter for a sensitive-tier read.
// Note there is NO Unit field — the unit set is fixed in code (the
// whole point of the security model).
type SystemQuery struct {
	// Lines bounds how many records this read returns. 0 / negative
	// → defaultSystemLines (100); above maxSystemLines (500) clamps
	// to the ceiling.
	Lines int

	// Since is forwarded verbatim to journalctl's --since flag.
	// journalctl's own parser accepts "1 hour ago", "2026-05-31",
	// "2026-05-31 14:00:00", etc.
	Since string

	// Failures, when true, adds `-p err..warning` so the read
	// excludes successful logins / no-op sudos. Backs the
	// journal://system/failures resource variant. Range MUST be
	// written low-to-high (journalctl reads `-p LOW..HIGH` numerically;
	// see BuildSystemArgs + issue #639 for the bug class).
	Failures bool
}

// SystemEntry is the wire shape an agent reads. Fields are LOCKED by
// system_test.go::TestSystemEntry_StableWireKeys and the negative-
// space test TestSystemEntry_NeverLeaksForbiddenFields.
//
// IMPORTANT: do NOT add a Cmdline / PID / AuditSession field to this
// struct. ADR-0049 calls out the omission explicitly; the security
// promise is what makes the resource family acceptable to ship.
type SystemEntry struct {
	Time     string `json:"ts"`       // RFC3339Nano, from __REALTIME_TIMESTAMP
	Unit     string `json:"unit"`     // _SYSTEMD_UNIT, falling back to _COMM
	Hostname string `json:"hostname"` // _HOSTNAME
	Message  string `json:"message"`  // MESSAGE
}

// BuildSystemArgs translates a SystemQuery into the journalctl argv.
// Pure — exec'd by the runner exactly as returned.
func BuildSystemArgs(q SystemQuery) []string {
	args := []string{"-o", "json", "--no-pager"}
	// Pinned auth selectors come BEFORE -n so the matchers attach to
	// the read context; ordering is for readability, not semantics
	// (journalctl accepts matchers anywhere on the line).
	args = append(args, authSelectors...)

	lines := q.Lines
	if lines <= 0 {
		lines = defaultSystemLines
	}
	if lines > maxSystemLines {
		lines = maxSystemLines
	}
	args = append(args, "-n", strconv.Itoa(lines))

	if q.Since != "" {
		args = append(args, "--since", q.Since)
	}
	if q.Failures {
		// PRIORITY range — err (3) and warning (4) inclusive.
		// journalctl reads `-p LOW..HIGH` using the numeric syslog
		// priorities (emerg=0, alert=1, crit=2, err=3, warning=4,
		// notice=5, info=6, debug=7), so the range MUST be written
		// low-to-high. The historical `warning..error` spelling was
		// REVERSED (4..3) AND used the non-canonical priority name
		// `error` (journalctl wants `err`); on systemd ≥ 245 it
		// errored out with "Failed to parse log level range
		// warning..error" and surfaced as `exit status 1` to the
		// MCP agent (issue #639). Pinned by
		// system_test.go::TestBuildSystemArgs_FailuresPriorityRangeIsValid.
		args = append(args, "-p", "err..warning")
	}
	return args
}

// ReadSystem runs journalctl via the injected runner and parses the
// NDJSON into the bounded wire shape.
func ReadSystem(ctx context.Context, run Runner, q SystemQuery) ([]SystemEntry, error) {
	out, err := run(ctx, BuildSystemArgs(q))
	if err != nil {
		return nil, fmt.Errorf("run journalctl (sensitive tier): %w", err)
	}
	return ParseSystem(out)
}

// rawSystemEntry mirrors the journalctl -o json fields ParseSystem
// consumes. Fields not listed here are never decoded — the wire-shape
// contract is enforced both in SystemEntry and at parse time.
type rawSystemEntry struct {
	Realtime string `json:"__REALTIME_TIMESTAMP"`
	Unit     string `json:"_SYSTEMD_UNIT"`
	Comm     string `json:"_COMM"`
	Hostname string `json:"_HOSTNAME"`
	Message  string `json:"MESSAGE"`
}

// ParseSystem reads `journalctl -o json` NDJSON. Blank lines + records
// that don't decode (non-text MESSAGE, log rotation gaps) are skipped
// rather than aborting the whole read.
func ParseSystem(b []byte) ([]SystemEntry, error) {
	sc := bufio.NewScanner(bytes.NewReader(b))
	// MESSAGE can carry multi-line content; raise from bufio's 64 KiB
	// default to 1 MiB so a single long sshd debug line doesn't
	// truncate the record.
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)

	var entries []SystemEntry
	for sc.Scan() {
		line := sc.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var r rawSystemEntry
		if err := json.Unmarshal(line, &r); err != nil {
			continue // malformed / non-text MESSAGE — skip
		}
		// _COMM-only records (sudo logs typically arrive that way)
		// have no _SYSTEMD_UNIT; fall back so the agent has SOMETHING
		// useful in the unit column.
		unit := r.Unit
		if unit == "" {
			unit = r.Comm
		}
		entries = append(entries, SystemEntry{
			Time:     realtimeToRFC3339(r.Realtime),
			Unit:     unit,
			Hostname: r.Hostname,
			Message:  r.Message,
		})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan system journal: %w", err)
	}
	return entries, nil
}
