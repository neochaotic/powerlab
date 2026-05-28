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
//
// Memory profile (2026-05-28 audit, fixed in this file):
// the original implementation `os.ReadFile`'d the whole audit log into
// a single []byte before scanning it, which made a 500 MiB audit log
// drive ~2.8 GiB of RSS in the MCP service during a single audit://
// read. On a Pi 4 / 8 GiB box that's an OOM. The Recent + ByCorrelation
// readers now stream the file via bufio.Scanner and keep only up to
// `limit` records in memory at any time, so RSS during a read scales
// with the requested limit (max 1000 records, ~few hundred KiB),
// independent of the file size.
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

// scannerLineCap is the bufio.Scanner buffer ceiling. Audit records carry
// a frontend-error payload (stack traces) that can be long; raise the
// line ceiling above the 64 KiB default. A single record exceeding 1 MiB
// is broken-by-construction at the writer (the audit middleware itself
// caps stack-trace payload), so a line over 1 MiB ⇒ scanner returns an
// error which we surface; we do NOT silently skip oversized lines (a
// rotation race with a partial >1 MiB line would corrupt a payload-by-
// payload read silently).
const scannerLineCap = 1 << 20

// Recent returns up to limit of the newest records in the JSONL file at
// path (appended chronologically, so newest = last). A missing file
// yields an empty result, not an error — a box that has audited nothing
// yet has no file. limit ≤ 0 applies the default; it is clamped to
// maxLimit.
//
// Streams the file: peak in-memory cost is O(limit) records, not
// O(file_size). Implementation: ring buffer of capacity `limit`, with
// the newest record overwriting the oldest once the ring is full.
func Recent(path string, limit int) ([]Record, error) {
	limit = clampLimit(limit)
	ring := make([]Record, limit)
	head := 0     // next write position
	filled := 0   // count of records seen, capped at limit
	totalSeen := 0

	err := scanRecords(path, func(r Record) bool {
		ring[head] = r
		head = (head + 1) % limit
		if filled < limit {
			filled++
		}
		totalSeen++
		return true
	})
	if err != nil {
		return nil, err
	}

	// Reconstruct the records in oldest→newest order so callers receive
	// the canonical newest-last sequence. When the ring never filled,
	// the in-order slice is just ring[:filled]; once it wrapped, the
	// oldest sits at `head` (where we'd overwrite next).
	if filled < limit {
		return ring[:filled], nil
	}
	out := make([]Record, limit)
	for i := 0; i < limit; i++ {
		out[i] = ring[(head+i)%limit]
	}
	return out, nil
}

// ByCorrelation returns up to limit records whose RequestID matches id —
// the audit trail produced under one correlation id (e.g. everything a
// single tool call triggered). Newest-last order is preserved.
//
// Streams the file and only keeps records that match. Peak memory is
// O(limit) — independent of file size.
func ByCorrelation(path, id string, limit int) ([]Record, error) {
	limit = clampLimit(limit)
	out := make([]Record, 0, limit)

	err := scanRecords(path, func(r Record) bool {
		if r.RequestID != id {
			return true
		}
		if len(out) < limit {
			out = append(out, r)
			return true
		}
		// Ring on the bounded slice: drop the oldest match, append the
		// newest. The shift cost is O(limit) per match — fine because
		// most correlation ids have ≪ limit records associated.
		copy(out, out[1:])
		out[len(out)-1] = r
		return true
	})
	if err != nil {
		return nil, err
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

// scanRecords streams the JSONL file, calling visit for each successfully
// decoded record. Blank and malformed JSON lines are skipped (rotation
// gaps + partial writes) rather than aborting the scan. A missing file
// is not an error — a box that hasn't audited anything yet has no file.
// The scanner buffer is capped at scannerLineCap so a runaway payload
// cannot drive the process into unbounded line-buffering.
func scanRecords(path string, visit func(Record) bool) error {
	// #nosec G304 -- path is cfg.AuditDir (trusted) + a fixed filename.
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("open audit log: %w", err)
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), scannerLineCap)
	for sc.Scan() {
		line := sc.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var r Record
		if err := json.Unmarshal(line, &r); err != nil {
			continue
		}
		if !visit(r) {
			return nil
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("scan audit log: %w", err)
	}
	return nil
}
