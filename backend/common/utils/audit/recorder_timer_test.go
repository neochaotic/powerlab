package audit_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/neochaotic/powerlab/backend/common/utils/audit"
)

// Regression test for the timer-stall bug: when fewer than
// BatchSize records arrive within a single MaxLatency window, the
// timer fires once with an empty batch (which is a no-op), and
// THEN the timer goes dead. Subsequent records sit in the channel
// forever waiting for the BatchSize threshold.
//
// Symptom in the wild: POSTs to /v1/audit/frontend-error returned
// 202 but the record never appeared in the JSONL file or the
// /v1/audit/recent ring buffer — until 50 records accumulated, at
// which point all 50 flushed at once.
//
// Failing-test-first per TDD: locks the contract that a single
// Submit followed by a wait of 2× MaxLatency must surface in
// Recent (the in-memory ring buffer is updated by the same
// AppendBatch call that writes the JSONL line).

func TestRecorder_SingleRecordFlushesWithinMaxLatency(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	svc, err := audit.NewService(audit.ServiceOptions{
		Path: path,
		Recorder: audit.RecorderOptions{
			// Tight MaxLatency so the test isn't slow; the bug
			// reproduces at any MaxLatency.
			MaxLatency: 50 * time.Millisecond,
		},
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	rec := audit.Record{Method: "GET", Path: "/single", Status: 200, RemoteIP: "x"}
	rec.FillTimestamps(time.Now())
	svc.Recorder.Submit(rec)

	// Wait 4× MaxLatency to give plenty of room.
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if rows := svc.Store.Recent(audit.RecentOptions{Limit: 5}); len(rows) > 0 {
			return // good — the timer flushed the single record
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("single record never surfaced within 4× MaxLatency — timer-stall bug")
}

func TestRecorder_SecondRecordAfterQuietPeriodAlsoFlushes(t *testing.T) {
	// This is the more pathological case: one record flushes,
	// then a quiet period drains the timer, then a SECOND record
	// arrives. Without the fix, the timer is dead and the second
	// record waits forever.
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	svc, err := audit.NewService(audit.ServiceOptions{
		Path: path,
		Recorder: audit.RecorderOptions{
			MaxLatency: 50 * time.Millisecond,
		},
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	// First record: flushes within MaxLatency
	r1 := audit.Record{Method: "GET", Path: "/r1", Status: 200, RemoteIP: "x"}
	r1.FillTimestamps(time.Now())
	svc.Recorder.Submit(r1)
	time.Sleep(150 * time.Millisecond)

	rows1 := svc.Store.Recent(audit.RecentOptions{Limit: 10})
	if len(rows1) != 1 {
		t.Fatalf("after first submit: got %d, want 1", len(rows1))
	}

	// Quiet period: 3× MaxLatency with no new records. This is
	// where the timer can go dead.
	time.Sleep(200 * time.Millisecond)

	// Second record: must also flush within MaxLatency.
	r2 := audit.Record{Method: "GET", Path: "/r2", Status: 200, RemoteIP: "x"}
	r2.FillTimestamps(time.Now())
	svc.Recorder.Submit(r2)

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if rows := svc.Store.Recent(audit.RecentOptions{Limit: 10}); len(rows) >= 2 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("second record never surfaced after a quiet period — timer went dead after empty-batch tick")
}
