// Package audittail is a read-only tail reader for the PowerLab audit
// JSONL log (ADR-0035), backing the audit:// MCP resources. It does NOT
// use audit.Store (the writer): that opens a lumberjack writer and does
// not hydrate from the file. Here we just read the file and parse the
// exported audit.Record wire type.
//
// There is a single audit file (the gateway is the only service that
// mounts the audit middleware, ADR-0033); "service" is a field on each
// record (Path), not a separate file. So the resources read one path and
// filter in-memory.
package audittail

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/neochaotic/powerlab/backend/common/utils/audit"
)

// Record is the audit wire type, re-exported so callers don't import the
// writer-side package directly.
type Record = audit.Record

// Limit bounds. Audit volume is low (dozens/day per ADR-0035), but a read
// must still be bounded so it can't pull an unbounded slice into the
// agent's context.
const (
	defaultLimit = 100
	maxLimit     = 1000
)

// Recent returns up to limit of the newest records in the JSONL file at
// path (appended chronologically, so newest = last). A missing file
// yields an empty result, not an error — a box that has audited nothing
// yet has no file. limit ≤ 0 applies the default; it is clamped to
// maxLimit.
func Recent(path string, limit int) ([]Record, error) {
	recs, err := readAll(path)
	if err != nil {
		return nil, err
	}
	limit = clampLimit(limit)
	if len(recs) > limit {
		recs = recs[len(recs)-limit:]
	}
	return recs, nil
}

// ByCorrelation returns up to limit records whose RequestID matches id —
// the audit trail produced under one correlation id (e.g. everything a
// single tool call triggered). Newest-last order is preserved.
func ByCorrelation(path, id string, limit int) ([]Record, error) {
	recs, err := readAll(path)
	if err != nil {
		return nil, err
	}
	limit = clampLimit(limit)
	out := make([]Record, 0, limit)
	for _, r := range recs {
		if r.RequestID == id {
			out = append(out, r)
		}
	}
	if len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out, nil
}

func clampLimit(n int) int {
	if n <= 0 {
		return defaultLimit
	}
	if n > maxLimit {
		return maxLimit
	}
	return n
}

// readAll reads and parses every record in the JSONL file. Blank and
// malformed lines are skipped (rotation gaps, partial writes) rather than
// aborting — better to return the parseable records.
func readAll(path string) ([]Record, error) {
	b, err := readFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read audit log: %w", err)
	}

	var recs []Record
	sc := bufio.NewScanner(bytes.NewReader(b))
	// Audit records carry a frontend-error payload (stack traces) that can
	// be long; raise the line ceiling from bufio's 64 KiB default.
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var r Record
		if err := json.Unmarshal(line, &r); err != nil {
			continue
		}
		recs = append(recs, r)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan audit log: %w", err)
	}
	return recs, nil
}

// readFile reads path. path is derived from cfg.AuditDir (a trusted,
// operator-configured directory) joined with a fixed filename, so the
// gosec G304 file-inclusion finding is a false positive, suppressed here.
func readFile(path string) ([]byte, error) {
	// #nosec G304 -- path is cfg.AuditDir (trusted) + a fixed filename.
	return os.ReadFile(path)
}
