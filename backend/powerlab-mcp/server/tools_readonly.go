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
		Description: "Search PowerLab service journals by pattern + time range. READ ONLY — does not modify any state. Returns matching entries up to the line limit. For raw tail access without a search filter, prefer journal://{unit}. See journal://units for available unit names.",
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
			return nil, JournalSearchOutput{}, fmt.Errorf("journal read: %w", err)
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
		Description: "Quick free-space check for one filesystem path (defaults to / — the primary disk). READ ONLY. Returns total/available/used bytes + used percent. For a per-mount survey with SMART metadata, prefer system://disk.",
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
