package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/neochaotic/powerlab/backend/powerlab-mcp/journal"
)

// ADR-0046 — read-only tools land first. They dogfood the tool-call
// pattern without risk (no side-effects) and lock in the per-tool
// description tone (action-oriented, side-effect class explicit so
// the model surfaces it to the user).
//
// Both tools here are deliberately thin wrappers over capabilities
// the resource layer already has — they exist for AGENT ergonomics
// (typed schema, fewer rounds) rather than to do new work. An agent
// that prefers reading resources can still drive journal:// or
// system://disk directly; the tools are the friendlier path for
// "find X" / "is Y OK?" question shapes.

// JournalSearchInput is the typed input for the journal_search tool.
// JSON Schema is auto-derived by the SDK from the struct tags.
type JournalSearchInput struct {
	// Unit is the PowerLab service stem (e.g. "core" or "gateway") —
	// the same value journal://units enumerates. canonicalUnit
	// re-applies the powerlab- prefix + .service suffix so an agent
	// passing "powerlab-core.service" or "core" both work.
	Unit string `json:"unit" jsonschema:"PowerLab service name (e.g. 'core' or 'gateway'); see journal://units for the available stems"`

	// Pattern is a literal substring matcher applied to MESSAGE
	// after the journalctl read. Empty = no pattern filter.
	Pattern string `json:"pattern,omitempty" jsonschema:"Optional literal substring filter on MESSAGE; empty matches all entries"`

	// Since is the journalctl --since argument — accepts the same
	// shapes (e.g. "1h" or "yesterday" or an absolute timestamp).
	Since string `json:"since,omitempty" jsonschema:"journalctl --since value (e.g. '1h' or 'yesterday'); empty uses the default recent window"`

	// Lines bounds the response. Defaults to 200; capped at 2000
	// — same bounds the journal:// resource uses.
	Lines int `json:"lines,omitempty" jsonschema:"Max matching lines to return (default 200; max 2000)"`
}

// JournalSearchOutput is the typed output an agent reads — also
// auto-emitted as JSON Schema.
type JournalSearchOutput struct {
	Unit    string          `json:"unit"`
	Pattern string          `json:"pattern,omitempty"`
	Entries []journal.Entry `json:"entries"`
}

// registerJournalSearch exposes the journal_search MCP tool. Side-
// effect class: READ. journalRun is the same Runner the journal://
// resource uses — tests inject a fixture-backed runner.
func registerJournalSearch(s *mcp.Server, journalRun journal.Runner) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "journal_search",
		Description: "READ ONLY — search PowerLab service journals (gateway, core, app-management, user, mcp) by literal substring + time window. Pattern is a literal substring on MESSAGE — NOT a regex, no escaping needed. `since` accepts journalctl shapes ('1h', '30min', 'yesterday', or RFC-3339); empty = a recent default. Returns up to `lines` matching entries (default 200, max 2000). Use when investigating a failed install, an unhealthy service, or any 'why did X happen at time Y'. For raw tail without filtering, prefer journal://{unit}; see journal://units for valid unit stems.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in JournalSearchInput) (*mcp.CallToolResult, JournalSearchOutput, error) {
		if strings.TrimSpace(in.Unit) == "" {
			return nil, JournalSearchOutput{}, errors.New("unit is required (see journal://units)")
		}
		// Reuse journal.Read end-to-end: same canonicalUnit scoping
		// (agent cannot escape to system units), same line cap, same
		// NDJSON parsing.
		q := journal.Query{
			Unit:     in.Unit,
			Lines:    in.Lines,
			Since:    in.Since,
			Priority: "",
		}
		entries, err := journal.Read(ctx, journalRun, q)
		if err != nil {
			return nil, JournalSearchOutput{}, classifyJournalReadError(err)
		}
		// Pattern filter is applied here rather than via journalctl -g
		// because journalctl's grep is regex-only and we want the
		// agent to be able to pass a literal substring without escaping.
		// Strings.Contains is the safest contract for an LLM input.
		if p := strings.TrimSpace(in.Pattern); p != "" {
			matched := entries[:0]
			for _, e := range entries {
				if strings.Contains(e.Message, p) {
					matched = append(matched, e)
				}
			}
			entries = matched
		}
		return nil, JournalSearchOutput{
			Unit:    in.Unit,
			Pattern: in.Pattern,
			Entries: entries,
		}, nil
	})
}

// CheckDiskFreeInput is the typed input for check_disk_free.
type CheckDiskFreeInput struct {
	// Path is any filesystem path; statfs is called on it and the
	// containing filesystem's free space is reported. Defaults to /
	// when empty — the operator's primary disk.
	Path string `json:"path,omitempty" jsonschema:"Filesystem path to check; defaults to / (the box's primary disk)"`
}

// CheckDiskFreeOutput carries the answer in operator-friendly units
// alongside the raw byte counts the agent might compose with.
type CheckDiskFreeOutput struct {
	Path           string  `json:"path"`
	TotalBytes     uint64  `json:"total_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	UsedPercent    float64 `json:"used_percent"`
}

// registerCheckDiskFree exposes the check_disk_free MCP tool. Side-
// effect class: READ.
func registerCheckDiskFree(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "check_disk_free",
		Description: "READ ONLY — quick free-space check for ONE filesystem path (defaults to / — the primary disk). Common targets on PowerLab: `/` (root), `/var/lib/docker` (container layers), `/var/log` (logs), `/DATA` (default app data root). Returns total/available/used bytes + used percent. Cheap — call this BEFORE `install_app` (host needs ≥5% free to avoid eviction) or when an operator asks 'is X full'. Treat used_percent ≥ 95 as critical, ≥ 85 as warn. For a full per-mount survey with SMART metadata, prefer system://disk.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in CheckDiskFreeInput) (*mcp.CallToolResult, CheckDiskFreeOutput, error) {
		path := strings.TrimSpace(in.Path)
		if path == "" {
			path = "/"
		}
		// statfs requires the path to exist; surface a friendly error
		// on a typo rather than a Go syscall error string.
		if _, err := os.Stat(path); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil, CheckDiskFreeOutput{}, fmt.Errorf("path %q does not exist", path)
			}
			return nil, CheckDiskFreeOutput{}, fmt.Errorf("stat %q: %w", path, err)
		}
		var st syscall.Statfs_t
		if err := syscall.Statfs(path, &st); err != nil {
			return nil, CheckDiskFreeOutput{}, fmt.Errorf("statfs %q: %w", path, err)
		}
		total := uint64(st.Blocks) * uint64(st.Bsize)
		avail := uint64(st.Bavail) * uint64(st.Bsize)
		if avail > total { // degraded zfs case — clamp
			avail = total
		}
		used := total - avail
		var pct float64
		if total > 0 {
			pct = float64(used) / float64(total) * 100
			pct = float64(int(pct*10+0.5)) / 10
		}
		return nil, CheckDiskFreeOutput{
			Path:           path,
			TotalBytes:     total,
			AvailableBytes: avail,
			UsedBytes:      used,
			UsedPercent:    pct,
		}, nil
	})
}

// registerReadOnlyTools keeps registration-site discipline: every
// tool we ship is wired in one place + every change here shows up
// in `tools/list` automatically. Future destructive tools (ADR-0046
// batch 3) gate on cfg.EnableDestructiveTools — registered separately.
func registerReadOnlyTools(s *mcp.Server, journalRun journal.Runner) {
	registerJournalSearch(s, journalRun)
	registerCheckDiskFree(s)
}

// classifyJournalReadError turns the raw journal.Read error into a
// message the agent can surface to an operator without leaking
// subprocess noise. The most common failure shape on Mac dev boxes
// and minimal containers is `exit status 1` from a non-existent or
// non-running journalctl — opaque to anyone reading just the error
// text. We trade that for a structured hint that names the cause and
// suggests where to look. The MCP SDK delivers this as the tool's
// IsError=true content, so the agent sees clean prose instead of
// `journal read: run journalctl: exit status 1`.
//
// Other errors (timeout, journalctl present but ACL-denied) fall
// through to a generic but still leak-free shape.
func classifyJournalReadError(err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "exit status"),
		strings.Contains(msg, "executable file not found"),
		strings.Contains(msg, "no such file or directory"):
		return errors.New("journald unavailable on this host (systemd-journald not running, or journalctl not in PATH — common on macOS dev boxes and non-systemd containers)")
	case errors.Is(err, context.DeadlineExceeded):
		return errors.New("journal read timed out — host journal may be very large; narrow the query with a smaller --lines or --since window")
	}
	return errors.New("journal read failed: " + sanitizeJournalErr(msg))
}

// sanitizeJournalErr strips known noisy prefixes from journal.Read's
// wrapped error so the surfaced text reads as one sentence rather than
// a wrapped-error chain. Keep this list small — better to leave the
// occasional unknown shape than to fully suppress useful detail.
func sanitizeJournalErr(s string) string {
	for _, prefix := range []string{"run journalctl: ", "journal read: "} {
		s = strings.TrimPrefix(s, prefix)
	}
	return s
}
