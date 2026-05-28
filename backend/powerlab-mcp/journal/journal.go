// Package journal reads the systemd journal via `journalctl -o json`,
// scoped to PowerLab units, for the journal:// MCP resource. The exec is
// injected (a Runner) so the arg-building and parsing are unit-testable
// without journalctl, and so the resource can be exercised with fixtures
// on any OS.
//
// Access is deliberately scoped to PowerLab units (powerlab-*.service):
// the unit parameter is agent-supplied, so it is canonicalised rather
// than passed through, preventing an agent from reading other services'
// journals. Arguments are passed to exec as a literal argv (no shell),
// and each agent-supplied value is the argument to a flag, so there is
// no flag- or shell-injection surface.
package journal

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Entry is one journal record, flattened to the fields the MCP resource
// exposes.
type Entry struct {
	Time     string `json:"time"` // RFC3339Nano, from __REALTIME_TIMESTAMP
	Unit     string `json:"unit"`
	Priority int    `json:"priority"`
	Message  string `json:"message"`
}

// Query is the agent-facing filter for a journal read.
type Query struct {
	Unit     string // bare or full unit; canonicalised to powerlab-<x>.service
	Lines    int    // 0 = journalctl default
	Since    string // passed to --since (e.g. "1h", "2026-05-28 00:00:00")
	Priority string // passed to -p (e.g. "err", "3")
}

// Runner executes journalctl with args and returns its stdout. Production
// wires exec.CommandContext; tests inject a fixture.
type Runner func(ctx context.Context, args []string) ([]byte, error)

// BuildArgs translates a Query into the journalctl argv. Pure. Always
// requests JSON, and always constrains -u to a PowerLab unit.
func BuildArgs(q Query) []string {
	args := []string{"-o", "json", "--no-pager"}
	if q.Unit == "" {
		args = append(args, "-u", "powerlab-*.service")
	} else {
		args = append(args, "-u", canonicalUnit(q.Unit))
	}
	if q.Lines > 0 {
		args = append(args, "-n", strconv.Itoa(q.Lines))
	}
	if q.Since != "" {
		args = append(args, "--since", q.Since)
	}
	if q.Priority != "" {
		args = append(args, "-p", q.Priority)
	}
	return args
}

// canonicalUnit scopes any agent-supplied unit name to a PowerLab unit:
// "core", "powerlab-core", and "powerlab-core.service" all map to
// "powerlab-core.service", and a foreign name keeps the powerlab- prefix.
func canonicalUnit(u string) string {
	u = strings.TrimSuffix(u, ".service")
	u = strings.TrimPrefix(u, "powerlab-")
	return "powerlab-" + u + ".service"
}

// Read runs journalctl through the runner and parses the result.
func Read(ctx context.Context, run Runner, q Query) ([]Entry, error) {
	out, err := run(ctx, BuildArgs(q))
	if err != nil {
		return nil, fmt.Errorf("run journalctl: %w", err)
	}
	return Parse(out)
}

// rawEntry mirrors the journalctl -o json fields we consume. journalctl
// emits every field as a JSON string; lines where MESSAGE is non-text
// (an array of bytes) fail to decode and are skipped by Parse.
type rawEntry struct {
	Realtime string `json:"__REALTIME_TIMESTAMP"`
	Unit     string `json:"_SYSTEMD_UNIT"`
	Priority string `json:"PRIORITY"`
	Message  string `json:"MESSAGE"`
}

// Parse reads `journalctl -o json` NDJSON. Blank lines and lines that
// don't decode are skipped (rotation gaps, non-text MESSAGE) rather than
// aborting the whole read — better to return the parseable records.
func Parse(b []byte) ([]Entry, error) {
	sc := bufio.NewScanner(bytes.NewReader(b))
	// MESSAGE can carry a multi-line stack trace; raise the line ceiling
	// from bufio's 64 KiB default to 1 MiB.
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)

	var entries []Entry
	for sc.Scan() {
		line := sc.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var r rawEntry
		if err := json.Unmarshal(line, &r); err != nil {
			continue // malformed / non-text MESSAGE — skip
		}
		entries = append(entries, Entry{
			Time:     realtimeToRFC3339(r.Realtime),
			Unit:     r.Unit,
			Priority: atoiOr(r.Priority, 0),
			Message:  r.Message,
		})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan journal: %w", err)
	}
	return entries, nil
}

// realtimeToRFC3339 converts the journalctl __REALTIME_TIMESTAMP (unix
// microseconds as a string) to RFC3339Nano UTC. An unparseable value
// yields "" rather than a misleading epoch-zero timestamp.
func realtimeToRFC3339(micros string) string {
	us, err := strconv.ParseInt(micros, 10, 64)
	if err != nil {
		return ""
	}
	return time.UnixMicro(us).UTC().Format(time.RFC3339Nano)
}

func atoiOr(s string, fallback int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}
