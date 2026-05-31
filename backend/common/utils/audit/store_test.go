package audit_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/neochaotic/powerlab/backend/common/utils/audit"
)

// makeStore returns a Store rooted at t.TempDir().
func makeStore(t *testing.T, ringCap int) *audit.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	s, err := audit.NewStore(audit.StoreOptions{
		Path:         path,
		RingCapacity: ringCap,
	})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func sampleRecord(ts time.Time, uid int64, path string) audit.Record {
	r := audit.Record{
		Method:        "GET",
		Path:          path,
		Status:        200,
		LatencyMicros: 1234,
		UserID:        &uid,
		RemoteIP:      "192.168.1.1",
		RequestID:     "req-" + path,
	}
	uname := "alisson"
	r.Username = &uname
	r.FillTimestamps(ts)
	return r
}

func TestStore_AppendBatch_WritesJSONLAndUpdatesRing(t *testing.T) {
	s := makeStore(t, 10)
	now := time.Now()
	batch := []audit.Record{
		sampleRecord(now.Add(-2*time.Second), 1, "/a"),
		sampleRecord(now.Add(-1*time.Second), 1, "/b"),
		sampleRecord(now, 2, "/c"),
	}
	if err := s.AppendBatch(context.Background(), batch); err != nil {
		t.Fatalf("AppendBatch: %v", err)
	}

	// Ring serves newest-first.
	recent := s.Recent(audit.RecentOptions{Limit: 10})
	if len(recent) != 3 {
		t.Fatalf("Recent: got %d, want 3", len(recent))
	}
	if recent[0].Path != "/c" || recent[2].Path != "/a" {
		t.Errorf("expected newest-first order /c,/b,/a — got %s,%s,%s",
			recent[0].Path, recent[1].Path, recent[2].Path)
	}

	// File on disk has 3 JSONL lines.
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(s.Path())))
	if err != nil {
		raw, err = os.ReadFile(s.Path())
		if err != nil {
			t.Fatalf("read jsonl: %v", err)
		}
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines on disk, got %d: %q", len(lines), string(raw))
	}
	var first audit.Record
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("unmarshal first line: %v", err)
	}
	if first.Path != "/a" {
		t.Errorf("disk order should be insertion order — got first %q", first.Path)
	}
	// Wire shape sanity: snake_case keys present.
	if !strings.Contains(lines[0], `"ts_us":`) {
		t.Errorf("line missing snake_case ts_us key: %s", lines[0])
	}
	if !strings.Contains(lines[0], `"latency_us":`) {
		t.Errorf("line missing latency_us key: %s", lines[0])
	}
}

func TestStore_RingDropsOldestWhenOverCapacity(t *testing.T) {
	s := makeStore(t, 3)
	now := time.Now()
	for i := 0; i < 5; i++ {
		err := s.AppendBatch(context.Background(), []audit.Record{
			sampleRecord(now.Add(time.Duration(i)*time.Second), int64(i+1), "/p"),
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	recent := s.Recent(audit.RecentOptions{Limit: 10})
	if len(recent) != 3 {
		t.Fatalf("ring should hold last 3, got %d", len(recent))
	}
	// Newest first: 5, 4, 3
	if *recent[0].UserID != 5 || *recent[2].UserID != 3 {
		t.Errorf("expected newest user_ids 5..3, got %d..%d", *recent[0].UserID, *recent[2].UserID)
	}
}

func TestStore_Recent_UserIDFilter(t *testing.T) {
	s := makeStore(t, 10)
	now := time.Now()
	_ = s.AppendBatch(context.Background(), []audit.Record{
		sampleRecord(now.Add(-2*time.Second), 1, "/a"),
		sampleRecord(now.Add(-1*time.Second), 2, "/b"),
		sampleRecord(now, 1, "/c"),
	})

	uid := int64(1)
	recent := s.Recent(audit.RecentOptions{UserID: &uid, Limit: 10})
	if len(recent) != 2 {
		t.Fatalf("user filter: got %d, want 2", len(recent))
	}
	for _, r := range recent {
		if *r.UserID != 1 {
			t.Errorf("filter leaked uid %d", *r.UserID)
		}
	}
}

func TestStore_Recent_SinceCursor(t *testing.T) {
	s := makeStore(t, 10)
	now := time.Now()
	_ = s.AppendBatch(context.Background(), []audit.Record{
		sampleRecord(now.Add(-2*time.Second), 1, "/old"),
		sampleRecord(now.Add(-1*time.Second), 1, "/mid"),
		sampleRecord(now, 1, "/new"),
	})

	cursor := now.Add(-1500 * time.Millisecond).UnixMicro()
	recent := s.Recent(audit.RecentOptions{SinceUnixMicros: cursor, Limit: 10})
	if len(recent) != 2 {
		t.Fatalf("since cursor: got %d, want 2", len(recent))
	}
}

func TestStore_Recent_LimitClampedAt1000(t *testing.T) {
	s := makeStore(t, 2000)
	now := time.Now()
	batch := make([]audit.Record, 1500)
	for i := range batch {
		batch[i] = sampleRecord(now.Add(time.Duration(i)*time.Millisecond), 1, "/p")
	}
	if err := s.AppendBatch(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
	recent := s.Recent(audit.RecentOptions{Limit: 5000})
	if len(recent) != 1000 {
		t.Errorf("limit should clamp to 1000, got %d", len(recent))
	}
}

func TestStore_Stats_RowCountFileSizePath(t *testing.T) {
	s := makeStore(t, 10)
	now := time.Now()
	_ = s.AppendBatch(context.Background(), []audit.Record{
		sampleRecord(now.Add(-time.Second), 1, "/a"),
		sampleRecord(now, 2, "/b"),
	})

	stats, err := s.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.RowCount != 2 {
		t.Errorf("RowCount: got %d, want 2", stats.RowCount)
	}
	if stats.FileSizeBytes <= 0 {
		t.Errorf("FileSizeBytes should be > 0, got %d", stats.FileSizeBytes)
	}
	if stats.Path != s.Path() {
		t.Errorf("Path mismatch: %q vs %q", stats.Path, s.Path())
	}
	if stats.OldestUnixUs == 0 || stats.NewestUnixUs == 0 {
		t.Errorf("timestamps should be set with non-empty ring")
	}
	if stats.OldestUnixUs > stats.NewestUnixUs {
		t.Errorf("oldest > newest: %d > %d", stats.OldestUnixUs, stats.NewestUnixUs)
	}
}

func TestStore_EmptyStats(t *testing.T) {
	s := makeStore(t, 10)
	stats, err := s.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.RowCount != 0 {
		t.Errorf("expected empty ring, got count %d", stats.RowCount)
	}
	// No file yet — size should be 0.
	if stats.FileSizeBytes != 0 {
		t.Errorf("empty store should have size 0, got %d", stats.FileSizeBytes)
	}
}

func TestStore_Close_Idempotent(t *testing.T) {
	s := makeStore(t, 1)
	if err := s.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestStore_NewStore_RequiresPath(t *testing.T) {
	if _, err := audit.NewStore(audit.StoreOptions{}); err == nil {
		t.Errorf("NewStore should reject empty path")
	}
}

// REGRESSION — issue #632. ADR-0033 mandates per-service audit
// middleware; ADR-0035 promises the JSONL store is multi-writer
// safe. With ADR-0047 powerlab-mcp became the second writer to
// /var/log/powerlab/audit.jsonl alongside the gateway.
//
// Two separate Store instances writing to the same file MUST NOT
// trample each other. The kernel guarantees atomic appends below
// PIPE_BUF (4096 B on Linux) for separate file descriptors opened
// with O_APPEND — our JSONL records are well under that ceiling
// (a Record marshals to a few hundred bytes), so AppendBatch with
// one Record per call must land every line.
//
// Pre-fix (lumberjack, O_WRONLY|O_CREATE, no O_APPEND): the two
// FDs each hold an independent offset and overwrite each other.
// This test produced ~half the expected lines.
//
// Post-fix (direct os.OpenFile + O_APPEND): every Write hits the
// kernel's atomic append path; both writers' lines interleave but
// none are lost.
func TestStore_MultiWriter_AtomicAppend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")

	const writers = 4
	const recordsPerWriter = 250
	const totalWant = writers * recordsPerWriter

	stores := make([]*audit.Store, writers)
	for i := 0; i < writers; i++ {
		s, err := audit.NewStore(audit.StoreOptions{Path: path})
		if err != nil {
			t.Fatalf("NewStore #%d: %v", i, err)
		}
		stores[i] = s
		defer func(s *audit.Store) { _ = s.Close() }(s)
	}

	now := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(idx int, s *audit.Store) {
			defer wg.Done()
			for n := 0; n < recordsPerWriter; n++ {
				rec := sampleRecord(now.Add(time.Duration(n)*time.Microsecond), int64(idx+1), "/concurrent")
				if err := s.AppendBatch(context.Background(), []audit.Record{rec}); err != nil {
					t.Errorf("writer %d append %d: %v", idx, n, err)
					return
				}
			}
		}(i, stores[i])
	}
	wg.Wait()

	// Close all stores so any buffered state flushes (we don't
	// buffer above the kernel boundary, but Close is the contract).
	for _, s := range stores {
		if err := s.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	}

	body, err := os.ReadFile(path) // #nosec G304 -- path is t.TempDir()-derived test fixture
	if err != nil {
		t.Fatalf("read audit.jsonl: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != totalWant {
		t.Fatalf("got %d JSONL lines; want %d — concurrent writes were lost or torn", len(lines), totalWant)
	}
	// Every line must be a parseable Record. A torn write surfaces
	// here even if line count happens to match (e.g. an interleaved
	// short write that still ended in '\n').
	seen := make(map[string]int, totalWant)
	for i, line := range lines {
		var rec audit.Record
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("line %d not parseable JSON: %v\nline=%q", i, err, line)
		}
		if rec.UserID == nil {
			t.Fatalf("line %d missing user_id: %q", i, line)
		}
		seen[strings.TrimSpace(line)]++
	}
	// Each writer used a distinct UserID — count per-writer.
	perWriter := make(map[int64]int)
	for i, line := range lines {
		var rec audit.Record
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("re-parse line %d: %v", i, err)
		}
		perWriter[*rec.UserID]++
	}
	for uid := int64(1); uid <= int64(writers); uid++ {
		if got := perWriter[uid]; got != recordsPerWriter {
			t.Errorf("writer uid=%d wrote %d lines; want %d", uid, got, recordsPerWriter)
		}
	}
}
