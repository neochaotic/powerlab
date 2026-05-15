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

	"gopkg.in/natefinch/lumberjack.v2"
)

// StoreOptions configures the JSONL writer + ring buffer per ADR-0035.
//
//   - Path: file to write JSONL lines into. Rotated by lumberjack
//     into Path.1.gz, Path.2.gz, ... when MaxSizeMB is exceeded.
//   - MaxSizeMB: rotation threshold per file (default 10 MB).
//   - MaxBackups: number of rotated files to keep (default 60).
//   - MaxAgeDays: maximum age of rotated files (default 30).
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

// Store owns the JSONL writer (lumberjack) and the in-memory ring
// buffer that backs the UI's audit pane. Per ADR-0035, the JSONL
// file is the canonical persistent record (greppable from SSH); the
// ring is a hot-path cache to keep /v1/audit/recent sub-millisecond.
//
// Concurrency model:
//   - Append: writer goroutine holds the mu write-lock just long
//     enough to update the ring; the lumberjack write is done outside
//     the lock because lumberjack has its own internal synchronisation.
//   - Reads (Recent/Stats): take the mu read-lock; multiple readers
//     are concurrent. File-tail reads for older history open their
//     own os.File handles and do not block the writer.
type Store struct {
	opts StoreOptions

	mu   sync.RWMutex
	ring []Record // newest-last; len ≤ opts.RingCapacity
	w    *lumberjack.Logger
}

// NewStore opens (creates) the JSONL file and returns a Store ready
// for Append. The ring starts empty — historical records are not
// hydrated on boot (a deliberate trade-off: keeps boot fast; the
// file remains the source of truth for grep / observability).
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

	// Make sure parent dir exists. lumberjack creates the file
	// lazily on first write but expects the directory to be there.
	if err := os.MkdirAll(filepath.Dir(opts.Path), 0o755); err != nil {
		return nil, fmt.Errorf("audit: mkdir %s: %w", filepath.Dir(opts.Path), err)
	}

	w := &lumberjack.Logger{
		Filename:   opts.Path,
		MaxSize:    opts.MaxSizeMB,
		MaxBackups: opts.MaxBackups,
		MaxAge:     opts.MaxAgeDays,
		Compress:   true,
	}

	return &Store{
		opts: opts,
		ring: make([]Record, 0, opts.RingCapacity),
		w:    w,
	}, nil
}

// AppendBatch writes the records as JSONL lines to the file and
// appends them to the ring (newest at the end; oldest dropped when
// over capacity). One file write per call — lumberjack itself wraps
// a single os.File so multiple records in one Write reduce syscalls.
func (s *Store) AppendBatch(_ context.Context, batch []Record) error {
	if len(batch) == 0 {
		return nil
	}
	// Encode all lines first so a JSON-marshal error doesn't leave
	// a half-written line on disk.
	var buf strings.Builder
	for _, rec := range batch {
		b, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("audit: marshal: %w", err)
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	if _, err := s.w.Write([]byte(buf.String())); err != nil {
		return fmt.Errorf("audit: write jsonl: %w", err)
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
	if s.w == nil {
		return nil
	}
	err := s.w.Close()
	s.w = nil
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
	f, err := os.Open(path)
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
