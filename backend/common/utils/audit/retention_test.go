package audit_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/neochaotic/powerlab/backend/common/utils/audit"
)

// makeDBWith inserts n records at evenly-spaced timestamps ending
// at now, with the oldest at `oldestAge` ago. Used to set up prune
// scenarios.
func makeDBWith(t *testing.T, n int, oldestAge time.Duration) *audit.DB {
	t.Helper()
	db, err := audit.OpenDB(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	step := oldestAge / time.Duration(n)
	for i := 0; i < n; i++ {
		ts := now.Add(-oldestAge + time.Duration(i)*step)
		if err := db.Insert(context.Background(), sampleRecord(ts, int64(i+1))); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

// TestRetention_RunOnce_AgeOnly — MaxAge prunes old rows; recent
// rows survive. With MaxRows=0 (disabled), row-cap is a no-op.
func TestRetention_RunOnce_AgeOnly(t *testing.T) {
	db := makeDBWith(t, 10, 48*time.Hour) // 10 records spanning -48h..now
	defer db.Close()

	rr := audit.NewRetentionRunner(db, audit.RetentionOptions{
		MaxAge:            12 * time.Hour, // keep last 12h only
		Interval:          time.Hour,
		SkipWALCheckpoint: true,
	})
	defer rr.Close()

	agePruned, rowsPruned, err := rr.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if agePruned == 0 {
		t.Errorf("agePruned = 0, expected several rows older than 12h")
	}
	if rowsPruned != 0 {
		t.Errorf("rowsPruned = %d, expected 0 (MaxRows disabled)", rowsPruned)
	}

	// Remaining rows must all be within the kept window.
	n, _ := db.Count(context.Background())
	if n == 0 {
		t.Errorf("after prune all rows gone — expected some within 12h")
	}
}

// TestRetention_RunOnce_RowCapOnly — MaxRows alone trims oldest until
// at or below cap. MaxAge huge so age-prune is a no-op.
func TestRetention_RunOnce_RowCapOnly(t *testing.T) {
	db := makeDBWith(t, 50, time.Hour) // 50 rows in the last hour
	defer db.Close()

	rr := audit.NewRetentionRunner(db, audit.RetentionOptions{
		MaxAge:            10 * 365 * 24 * time.Hour, // basically never
		MaxRows:           20,
		Interval:          time.Hour,
		SkipWALCheckpoint: true,
	})
	defer rr.Close()

	agePruned, rowsPruned, err := rr.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if agePruned != 0 {
		t.Errorf("agePruned = %d, expected 0 (MaxAge too long)", agePruned)
	}
	if rowsPruned != 30 {
		t.Errorf("rowsPruned = %d, expected 30 (50 → 20)", rowsPruned)
	}

	n, _ := db.Count(context.Background())
	if n != 20 {
		t.Errorf("post-prune count = %d, want 20", n)
	}
}

// TestRetention_RunOnce_NoOpWhenNothingToPrune — a fresh DB or one
// fully within the keep window must complete cleanly with zero
// pruned. No errors, no side effects.
func TestRetention_RunOnce_NoOpWhenNothingToPrune(t *testing.T) {
	db := makeDBWith(t, 5, time.Minute) // 5 records, all < 1min old
	defer db.Close()

	rr := audit.NewRetentionRunner(db, audit.RetentionOptions{
		MaxAge:            24 * time.Hour,
		MaxRows:           1000,
		Interval:          time.Hour,
		SkipWALCheckpoint: true,
	})
	defer rr.Close()

	agePruned, rowsPruned, err := rr.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if agePruned != 0 || rowsPruned != 0 {
		t.Errorf("expected 0/0 pruned, got %d/%d", agePruned, rowsPruned)
	}

	n, _ := db.Count(context.Background())
	if n != 5 {
		t.Errorf("count drifted: %d, want 5", n)
	}
}

// TestRetention_Goroutine_FiresOnInterval — with a sub-second
// interval, the loop should call RunOnce automatically and prune
// rows. Proves the ticker is alive and the goroutine doesn't deadlock.
func TestRetention_Goroutine_FiresOnInterval(t *testing.T) {
	db := makeDBWith(t, 10, 48*time.Hour)
	defer db.Close()

	rr := audit.NewRetentionRunner(db, audit.RetentionOptions{
		MaxAge:            12 * time.Hour,
		Interval:          50 * time.Millisecond,
		SkipWALCheckpoint: true,
	})
	defer rr.Close()

	// Wait up to 1s for the goroutine to fire at least one prune.
	// We can't read agePruned from the loop — we observe the side
	// effect via row count instead.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		n, _ := db.Count(context.Background())
		if n < 10 {
			return // success: goroutine pruned at least one row
		}
		time.Sleep(20 * time.Millisecond)
	}
	n, _ := db.Count(context.Background())
	t.Errorf("after 1s of 50ms ticks, count = %d (still 10) — goroutine not pruning?", n)
}

// TestRetention_CloseIsIdempotent — calling Close multiple times
// must not panic or deadlock. Mirrors Recorder's contract.
func TestRetention_CloseIsIdempotent(t *testing.T) {
	db, err := audit.OpenDB(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rr := audit.NewRetentionRunner(db, audit.RetentionOptions{
		MaxAge:            time.Hour,
		Interval:          time.Hour,
		SkipWALCheckpoint: true,
	})

	// First close should block until the goroutine exits.
	rr.Close()
	// Second close should return immediately, no panic, no hang.
	done := make(chan struct{})
	go func() {
		rr.Close()
		close(done)
	}()
	select {
	case <-done:
		// ok
	case <-time.After(500 * time.Millisecond):
		t.Fatal("second Close() hung — not idempotent")
	}
}

// TestRetention_DefaultsApplied — zero-valued options get sane
// defaults. Locks the ADR-0033 contract so the public API can be
// called with `RetentionOptions{}` and behave correctly.
func TestRetention_DefaultsApplied(t *testing.T) {
	db, err := audit.OpenDB(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Pass zero-valued options. Goroutine must start and Close
	// must succeed without hanging — proves defaults() filled in
	// a non-zero Interval (zero would make NewTicker panic).
	rr := audit.NewRetentionRunner(db, audit.RetentionOptions{})
	done := make(chan struct{})
	go func() {
		rr.Close()
		close(done)
	}()
	select {
	case <-done:
		// ok
	case <-time.After(500 * time.Millisecond):
		t.Fatal("NewRetentionRunner+Close with zero opts hung — defaults not applied")
	}
}
