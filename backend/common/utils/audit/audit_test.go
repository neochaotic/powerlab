package audit_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/neochaotic/powerlab/backend/common/utils/audit"
)

// helper: int64 pointer
func i64p(v int64) *int64 { return &v }
func sp(v string) *string { return &v }

func sampleRecord(ts time.Time, user int64) audit.Record {
	return audit.Record{
		TsUnixMicros:  ts.UnixMicro(),
		Method:        "GET",
		Path:          "/v2/app_management/compose",
		Status:        200,
		LatencyMicros: 1234,
		UserID:        i64p(user),
		Username:      sp("alice"),
		RemoteIP:      "192.168.1.10",
	}
}

// TestOpenDB_CreatesSchema — fresh path produces a working DB with the
// expected schema. Asserts the table exists and is empty.
func TestOpenDB_CreatesSchema(t *testing.T) {
	db, err := audit.OpenDB(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	n, err := db.Count(context.Background())
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 0 {
		t.Errorf("fresh DB row count = %d, want 0", n)
	}
}

// TestOpenDB_Idempotent — re-opening an existing DB does not error
// and preserves data.
func TestOpenDB_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.db")
	db1, err := audit.OpenDB(path)
	if err != nil {
		t.Fatalf("first OpenDB: %v", err)
	}
	if err := db1.Insert(context.Background(), sampleRecord(time.Now(), 1)); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	db1.Close()

	db2, err := audit.OpenDB(path)
	if err != nil {
		t.Fatalf("re-OpenDB: %v", err)
	}
	defer db2.Close()
	n, err := db2.Count(context.Background())
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 1 {
		t.Errorf("after re-open row count = %d, want 1 (data lost?)", n)
	}
}

// TestRecorder_DrainsToDB — Submit() enqueues records that the writer
// goroutine flushes within a reasonable deadline. The hot path
// (Submit) must be < 50µs per call so the middleware overhead is
// invisible.
func TestRecorder_DrainsToDB(t *testing.T) {
	db, err := audit.OpenDB(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rec := audit.NewRecorder(db, audit.RecorderOptions{
		Capacity:   64,
		BatchSize:  10,
		MaxLatency: 50 * time.Millisecond,
	})
	defer rec.Close()

	for i := 0; i < 5; i++ {
		rec.Submit(sampleRecord(time.Now(), int64(i+1)))
	}

	// Wait up to 1s for the writer to flush.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		n, _ := db.Count(context.Background())
		if n == 5 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	n, _ := db.Count(context.Background())
	t.Errorf("after 1s, DB has %d records, want 5", n)
}

// TestRecorder_DropOldestOnPressure — when the channel is full and
// the writer cannot drain fast enough, Submit() drops the oldest
// queued record and counts the drop. The hot path NEVER blocks.
//
// We simulate this by stopping the writer via a tiny capacity + a
// batch larger than capacity, then flooding submissions.
func TestRecorder_DropOldestOnPressure(t *testing.T) {
	db, err := audit.OpenDB(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rec := audit.NewRecorder(db, audit.RecorderOptions{
		Capacity:   2, // intentionally tiny
		BatchSize:  100,
		MaxLatency: time.Hour, // never auto-flush — keep channel full
	})
	defer rec.Close()

	// Pause the writer by holding the test goroutine.
	// 10 submissions into a 2-slot channel forces 8 drops.
	for i := 0; i < 10; i++ {
		rec.Submit(sampleRecord(time.Now(), int64(i+1)))
	}

	dropped := rec.Dropped()
	if dropped < 1 {
		t.Errorf("Dropped() = %d, expected backpressure (>= 1) with channel cap 2 and 10 submits", dropped)
	}
}

// TestPruneByAge — rows older than the cutoff are removed; newer
// rows are kept. Drives the hourly retention goroutine.
func TestPruneByAge(t *testing.T) {
	db, err := audit.OpenDB(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Now()
	// 5 records: 2 old (< cutoff), 3 new (after cutoff).
	for _, age := range []time.Duration{10 * time.Hour, 5 * time.Hour, time.Minute, time.Second, 0} {
		if err := db.Insert(context.Background(), sampleRecord(now.Add(-age), 1)); err != nil {
			t.Fatal(err)
		}
	}

	// Cutoff at 1 hour ago — first 2 records should go.
	removed, err := db.PruneByAge(context.Background(), now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("PruneByAge: %v", err)
	}
	if removed != 2 {
		t.Errorf("PruneByAge removed %d rows, want 2", removed)
	}

	n, _ := db.Count(context.Background())
	if n != 3 {
		t.Errorf("after prune row count = %d, want 3", n)
	}
}

// TestPruneToMaxRows — when the table grows past the row cap, the
// oldest rows are removed until we're at or below the cap. Used by
// the size-based retention check.
func TestPruneToMaxRows(t *testing.T) {
	db, err := audit.OpenDB(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for i := 0; i < 100; i++ {
		if err := db.Insert(context.Background(), sampleRecord(time.Now().Add(time.Duration(i)*time.Second), 1)); err != nil {
			t.Fatal(err)
		}
	}

	removed, err := db.PruneToMaxRows(context.Background(), 30)
	if err != nil {
		t.Fatalf("PruneToMaxRows: %v", err)
	}
	if removed != 70 {
		t.Errorf("PruneToMaxRows removed %d rows, want 70", removed)
	}

	n, _ := db.Count(context.Background())
	if n != 30 {
		t.Errorf("after prune row count = %d, want 30", n)
	}
}

// TestLoopbackSentinel — when RemoteIP is the loopback literal, the
// stored value is "loopback" not "127.0.0.1" / "::1" (ADR-0033 PII).
// The middleware decides this; the DB just stores what it's given.
// This test pins the canonical value so middleware authors can rely
// on the sentinel.
func TestLoopbackSentinel(t *testing.T) {
	if audit.LoopbackSentinel != "loopback" {
		t.Errorf("LoopbackSentinel = %q, want \"loopback\"", audit.LoopbackSentinel)
	}
}
