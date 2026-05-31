package audit

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// StoreOptions configures the JSONL writer + ring buffer per ADR-0035.
//
//   - Path: file to write JSONL lines into. Opened with O_APPEND so
//     multiple writers (e.g. gateway + powerlab-mcp per ADR-0033's
//     per-service middleware mandate) can share the same file without
//     trampling each other — see ADR-0035 amendment, issue #632.
//   - MaxSizeMB: retained for compatibility with the previous
//     lumberjack-rotated layout but currently unused; external
//     rotation (logrotate / systemd) is the new contract. See issue
//     #634 follow-up for in-process rotation if it ever matters.
//   - MaxBackups: same compatibility-only status as MaxSizeMB.
//   - MaxAgeDays: same compatibility-only status as MaxSizeMB.
//   - RingCapacity: in-memory record count for the hot UI path
//     (default 1000). The ring serves /v1/audit/recent without disk
//     IO for the most recent entries.
type StoreOptions struct {
	Path         string
	MaxSizeMB    int
	MaxBackups   int
	MaxAgeDays   int
	RingCapacity int
}

// Store owns the JSONL writer and the in-memory ring buffer that
// backs the UI's audit pane. Per ADR-0035 (amended in #632), the
// JSONL file is the canonical persistent record (greppable from
// SSH) and the ring is a hot-path cache to keep /v1/audit/recent
// sub-millisecond.
//
// Concurrency model:
//   - Append: each AppendBatch call serialises the records into a
//     single buffer and issues ONE Write to a file descriptor opened
//     with O_APPEND. The kernel atomically positions every O_APPEND
//     Write at end-of-file for separate FDs; below PIPE_BUF (4096 B
//     on Linux) the write itself is atomic with respect to other
//     writers. Each Record marshals to a few hundred bytes, so a
//     per-call batch of <=8 records stays well under the ceiling.
//     For larger batches the marshalled buffer can exceed PIPE_BUF
//     and we lose the per-Write atomicity guarantee — see the
//     splitOversizedBatch helper which falls back to per-record
//     writes in that case.
//   - Reads (Recent/Stats): take the ring's mu read-lock; multiple
//     readers are concurrent. File-tail reads for older history
//     open their own os.File handles and do not block the writer.
//
// Multi-writer safety (issue #632): two Store instances opened on
// the same Path are safe because both rely on the kernel's
// O_APPEND atomicity, not on any in-process mutex. This replaces
// the lumberjack-backed implementation that held an internal
// (per-instance) lock + a non-O_APPEND FD — fine within one
// process, broken across two.
type Store struct {
	opts StoreOptions

	mu   sync.RWMutex
	ring []Record // newest-last; len ≤ opts.RingCapacity

	// fileMu serialises writes within this Store instance. Two
	// instances on the same path still co-exist via O_APPEND, but
	// within a single instance the recorder + tests can call
	// AppendBatch concurrently; one Write per call keeps things
	// linear for the kernel and for the ring update that follows.
	fileMu sync.Mutex
	f      *os.File
}

// maxAtomicWrite is the largest single Write we trust the kernel to
// land atomically against other O_APPEND writers. Linux's PIPE_BUF
// is 4096 B; macOS BSD-derived stacks honour the same lower bound.
// We cap a touch under PIPE_BUF to leave headroom for newline +
// future field additions.
const maxAtomicWrite = 4000

// NewStore opens (creates) the JSONL file with O_APPEND and returns
// a Store ready for Append. The ring starts empty — historical
// records are not hydrated on boot (a deliberate trade-off: keeps
// boot fast; the file remains the source of truth for grep /
// observability).
func NewStore(opts StoreOptions) (*Store, error) {
	if opts.Path == "" {
		return nil, errors.New("audit: store path is required")
	}
	if opts.MaxSizeMB <= 0 {
		opts.MaxSizeMB = 10
	}
	if opts.MaxBackups <= 0 {
		opts.MaxBackups = 60
	}
	if opts.MaxAgeDays <= 0 {
		opts.MaxAgeDays = 30
	}
	if opts.RingCapacity <= 0 {
		opts.RingCapacity = 1000
	}

	// Parent directory must exist before OpenFile. 0o750 — audit
	// records carry PII (user ids, IPs); the directory is no more
	// permissive than the file itself (0o600 on OpenFile below).
	if err := os.MkdirAll(filepath.Dir(opts.Path), 0o750); err != nil {
		return nil, fmt.Errorf("audit: mkdir %s: %w", filepath.Dir(opts.Path), err)
	}

	// O_APPEND is the load-bearing flag here. ADR-0033 mandates
	// per-service audit middleware which means every service holds
	// its own FD on the same file; without O_APPEND each FD has an
	// independent offset and they trample each other (#632).
	// 0o600: audit records carry PII (user ids, IPs) — never
	// world-readable.
	f, err := os.OpenFile(opts.Path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600) // #nosec G304 -- caller-validated audit path (typically /var/log/powerlab/audit.jsonl)
	if err != nil {
		return nil, fmt.Errorf("audit: open %s: %w", opts.Path, err)
	}

	return &Store{
		opts: opts,
		ring: make([]Record, 0, opts.RingCapacity),
		f:    f,
	}, nil
}

// AppendBatch writes the records as JSONL lines to the file and
// appends them to the ring (newest at the end; oldest dropped when
// over capacity).
//
// Below maxAtomicWrite the whole batch ships in one Write so the
// kernel's O_APPEND atomicity covers it. Larger batches fall back
// to one Write per record — slightly more syscalls but still safe
// against concurrent writers from other processes.
func (s *Store) AppendBatch(_ context.Context, batch []Record) error {
	if len(batch) == 0 {
		return nil
	}
	// Encode all lines first so a JSON-marshal error doesn't leave
	// a half-written line on disk.
	encoded := make([][]byte, len(batch))
	total := 0
	for i, rec := range batch {
		b, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("audit: marshal: %w", err)
		}
		// Append the newline now so the per-record fallback can
		// hand a single ready buffer to Write without extra copies.
		line := append(b, '\n')
		encoded[i] = line
		total += len(line)
	}

	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	if s.f == nil {
		return errors.New("audit: store closed")
	}

	if total <= maxAtomicWrite {
		// Fast path: one Write, kernel-atomic for separate FDs.
		buf := make([]byte, 0, total)
		for _, line := range encoded {
			buf = append(buf, line...)
		}
		if _, err := s.f.Write(buf); err != nil {
			return fmt.Errorf("audit: write jsonl: %w", err)
		}
	} else {
		// Oversized batch: per-record Writes. Each individual line
		// is well under PIPE_BUF so each Write is still atomic vs
		// other writers, but lines from this batch may interleave
		// with another writer's lines on disk (which is fine — the
		// JSONL contract is line-oriented).
		for _, line := range encoded {
			if _, err := s.f.Write(line); err != nil {
				return fmt.Errorf("audit: write jsonl: %w", err)
			}
		}
	}

	// Update ring under write-lock.
	s.mu.Lock()
	for _, rec := range batch {
		if len(s.ring) >= s.opts.RingCapacity {
			// drop oldest (front)
			copy(s.ring, s.ring[1:])
			s.ring[len(s.ring)-1] = rec
		} else {
			s.ring = append(s.ring, rec)
		}
	}
	s.mu.Unlock()
	return nil
}

// Close flushes pending writes and closes the underlying file. Safe
// to call multiple times — subsequent calls are no-ops.
func (s *Store) Close() error {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	if s.f == nil {
		return nil
	}
	err := s.f.Close()
	s.f = nil
	return err
}

// Path returns the current JSONL file path. Exposed for /v1/audit/stats.
func (s *Store) Path() string { return s.opts.Path }

// RecentOptions filters the records returned by Recent.
//
//   - Limit caps the number of returned records (default 100, max 1000).
//   - UserID, if non-nil, restricts to records with that user_id.
//   - SinceUnixMicros, if > 0, returns only records with
//     ts_unix_us > SinceUnixMicros (used as a cursor for pagination).
type RecentOptions struct {
	Limit           int
	UserID          *int64
	SinceUnixMicros int64
}

// Recent returns the most recent records matching opts, newest first.
// Reads exclusively from the ring buffer — historical queries beyond
// the ring fall through to RecentWithHistory.
func (s *Store) Recent(opts RecentOptions) []Record {
	if opts.Limit <= 0 {
		opts.Limit = 100
	}
	if opts.Limit > 1000 {
		opts.Limit = 1000
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Record, 0, opts.Limit)
	// Walk ring back-to-front so newest entries come first.
	for i := len(s.ring) - 1; i >= 0 && len(out) < opts.Limit; i-- {
		rec := s.ring[i]
		if opts.UserID != nil {
			if rec.UserID == nil || *rec.UserID != *opts.UserID {
				continue
			}
		}
		if opts.SinceUnixMicros > 0 && rec.TsUnixMicros <= opts.SinceUnixMicros {
			continue
		}
		out = append(out, rec)
	}
	return out
}

// Stats describes the JSONL store's current state for the UI footer.
type Stats struct {
	RowCount      int64  `json:"row_count"`
	OldestUnixUs  int64  `json:"oldest_unix_us"`
	NewestUnixUs  int64  `json:"newest_unix_us"`
	FileSizeBytes int64  `json:"file_size_bytes"`
	Path          string `json:"path"`
}

// StatsResult is returned by Stats() — the row count is the ring's
// occupancy (fast). File size is read from the OS; missing file is
// reported as 0 bytes (not an error: the file is lazy-created).
func (s *Store) Stats(_ context.Context) (Stats, error) {
	s.mu.RLock()
	ringCount := len(s.ring)
	var oldest, newest int64
	if ringCount > 0 {
		oldest = s.ring[0].TsUnixMicros
		newest = s.ring[ringCount-1].TsUnixMicros
	}
	s.mu.RUnlock()

	var size int64
	if fi, err := os.Stat(s.opts.Path); err == nil {
		size = fi.Size()
	}
	// If the ring is full, the actual on-disk count is higher than
	// the ring's occupancy. The ring's count is "what's queryable
	// hot" — operators wanting the wire total grep the file.
	return Stats{
		RowCount:      int64(ringCount),
		OldestUnixUs:  oldest,
		NewestUnixUs:  newest,
		FileSizeBytes: size,
		Path:          s.opts.Path,
	}, nil
}

// recentFromFile is a (slow) fallback for queries whose `since`
// cursor predates the ring. It scans the current file and the most
// recent .gz backup looking for matches. Caller is expected to drop
// to this path only when Recent() returns fewer than Limit hits.
//
// Exported for the future MCP observability service to reuse;
// internally we don't call this on the hot path. Operators wanting
// deep history grep the JSONL files directly with jq.
func (s *Store) recentFromFile(opts RecentOptions) ([]Record, error) {
	dir, base := filepath.Split(s.opts.Path)
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	type fent struct {
		name    string
		modTime time.Time
		isGz    bool
	}
	candidates := []fent{}
	for _, f := range files {
		name := f.Name()
		if name == base {
			info, _ := f.Info()
			candidates = append(candidates, fent{name, info.ModTime(), false})
			continue
		}
		if strings.HasPrefix(name, base+".") && strings.HasSuffix(name, ".gz") {
			info, _ := f.Info()
			candidates = append(candidates, fent{name, info.ModTime(), true})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].modTime.After(candidates[j].modTime)
	})

	out := []Record{}
	for _, c := range candidates {
		if len(out) >= opts.Limit {
			break
		}
		path := filepath.Join(dir, c.name)
		records, err := readJSONLFile(path, c.isGz)
		if err != nil {
			continue // best-effort: skip unreadable files
		}
		// Walk records back-to-front (newest first within a file).
		for i := len(records) - 1; i >= 0 && len(out) < opts.Limit; i-- {
			rec := records[i]
			if opts.UserID != nil {
				if rec.UserID == nil || *rec.UserID != *opts.UserID {
					continue
				}
			}
			if opts.SinceUnixMicros > 0 && rec.TsUnixMicros <= opts.SinceUnixMicros {
				continue
			}
			out = append(out, rec)
		}
	}
	return out, nil
}

// readJSONLFile decodes a JSONL file (optionally gzipped) into a
// slice of Records. Bad lines are skipped (incremental recovery).
func readJSONLFile(path string, gzipped bool) ([]Record, error) {
	f, err := os.Open(path) // #nosec G304 -- caller-validated audit path
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var r io.Reader = f
	if gzipped {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		r = gz
	}
	out := []Record{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		var rec Record
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			continue // skip truncated/bad line
		}
		out = append(out, rec)
	}
	return out, sc.Err()
}
