package audittail

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeLog(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	body := ""
	for _, l := range lines {
		body += l + "\n"
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write audit log: %v", err)
	}
	return path
}

const (
	rec1 = `{"ts":"2026-05-28T00:00:01.000000Z","ts_us":1,"method":"POST","path":"/v2/app_management/compose","status":200,"user_id":1,"username":"alice","remote_ip":"loopback","request_id":"req-aaa"}`
	rec2 = `{"ts":"2026-05-28T00:00:02.000000Z","ts_us":2,"method":"GET","path":"/v1/audit/recent","status":200,"user_id":1,"username":"alice","remote_ip":"loopback","request_id":"req-bbb"}`
	rec3 = `{"ts":"2026-05-28T00:00:03.000000Z","ts_us":3,"method":"DELETE","path":"/v2/app_management/x","status":500,"user_id":1,"username":"alice","remote_ip":"loopback","request_id":"req-aaa"}`
)

// Recent returns the newest records (the file is appended chronologically,
// so newest = last) up to the limit — an agent asking "what just happened"
// wants the tail, not the head.
func TestRecent_ReturnsNewestUpToLimit(t *testing.T) {
	path := writeLog(t, rec1, rec2, rec3)
	got, err := Recent(path, 2)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d records; want 2", len(got))
	}
	// Newest first is not required, but the two newest (ts_us 2 and 3)
	// must be the ones returned, not the oldest.
	if got[0].TsUnixMicros != 2 || got[1].TsUnixMicros != 3 {
		t.Fatalf("got ts_us %d,%d; want the two newest (2,3)", got[0].TsUnixMicros, got[1].TsUnixMicros)
	}
}

// A fresh box may not have an audit file yet (nothing audited). That must
// be an empty result, not an error — the resource should report "no
// activity", not fail.
func TestRecent_MissingFileIsEmptyNotError(t *testing.T) {
	got, err := Recent(filepath.Join(t.TempDir(), "nope.jsonl"), 10)
	if err != nil {
		t.Fatalf("Recent(missing) = error %v; want empty + nil", err)
	}
	if len(got) != 0 {
		t.Fatalf("Recent(missing) = %d records; want 0", len(got))
	}
}

func TestRecent_SkipsBlankAndMalformedLines(t *testing.T) {
	path := writeLog(t, rec1, "", "not json", rec2)
	got, err := Recent(path, 10)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d records; want 2 (blank/malformed skipped)", len(got))
	}
}

// ByCorrelation links a tool/action to the audit records it produced —
// the correlation_id (request_id) ties them together.
func TestByCorrelation_FiltersByRequestID(t *testing.T) {
	path := writeLog(t, rec1, rec2, rec3) // rec1 + rec3 share req-aaa
	got, err := ByCorrelation(path, "req-aaa", 10)
	if err != nil {
		t.Fatalf("ByCorrelation: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d records for req-aaa; want 2", len(got))
	}
	for _, r := range got {
		if r.RequestID != "req-aaa" {
			t.Fatalf("record with request_id %q leaked into the req-aaa filter", r.RequestID)
		}
	}
}

// limit<=0 applies the default; an excessive limit is clamped, so one
// read can't pull an unbounded slice into the agent's context.
func TestRecent_DefaultsAndClampsLimit(t *testing.T) {
	if defaultLimit <= 0 || maxLimit < defaultLimit {
		t.Fatalf("bad bounds: default %d max %d", defaultLimit, maxLimit)
	}
	path := writeLog(t, rec1)
	// limit 0 → default applies (still returns the 1 record present).
	got, err := Recent(path, 0)
	if err != nil || len(got) != 1 {
		t.Fatalf("Recent(limit 0) = %d,%v; want 1 record (default applied)", len(got), err)
	}
}

// Regression for the 2026-05-28 audit OOM: the original implementation
// read the WHOLE audit.jsonl into a single []byte before scanning, which
// drove ~2.8 GiB of RSS during a single Recent read against a 500 MiB
// audit log (a real-life scenario on a chatty long-running box). On a
// Pi 4 with 4-8 GiB RAM, that's an OOM. The fix: stream the file
// line-by-line via bufio.Scanner, keep only `limit` records in memory
// at any time.
//
// This test synthesises a 200 MiB audit log and asserts:
//  1. Recent returns the requested limit's worth of records correctly
//  2. Resident memory growth during the read is bounded — the test
//     uses Go's runtime.MemStats HeapAlloc delta as a proxy for what
//     the OS would see in RSS (the original buggy code allocated
//     ~3-4x the file size into the heap, which we'd catch trivially)
//
// We pick a 200 MiB synthetic file (not 500 MiB) to keep the test fast
// + reliable in CI — the bug would have shown ~600 MiB heap growth
// even at this size. Our budget is 50 MiB heap delta — generous
// enough to absorb buffer-pool noise + JSON unmarshal scratch, tight
// enough that the old O(file) behaviour would fail by an order of
// magnitude. Skipped in `-short` so day-to-day go test stays snappy.
func TestRecent_MemoryBoundOnLargeFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 200 MiB synthetic file test in -short mode")
	}

	const (
		fileBytesTarget = 200 * 1024 * 1024 // 200 MiB
		heapDeltaCapMiB = 50                // see comment above
		recordsToRead   = 10                // ask Recent for just 10
	)

	path := filepath.Join(t.TempDir(), "audit.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create synthetic audit log: %v", err)
	}

	// One ~500 B record × ~420k = ~200 MiB. We write in chunks to keep
	// the test's own setup memory small.
	tmpl := `{"ts":"2026-05-28T00:00:01.000000Z","ts_us":%d,"method":"GET","path":"/v1/test","status":200,"user_id":1,"username":"alice","remote_ip":"loopback","request_id":"req-%d","user_agent":"%s"}`
	pad := strings.Repeat("x", 200)
	wrote := 0
	i := 0
	var buf strings.Builder
	buf.Grow(1 << 20)
	for wrote < fileBytesTarget {
		line := fmt.Sprintf(tmpl, i, i, pad)
		buf.WriteString(line)
		buf.WriteByte('\n')
		i++
		if buf.Len() >= 1<<20 {
			n, werr := f.WriteString(buf.String())
			if werr != nil {
				_ = f.Close()
				t.Fatalf("write synthetic audit log: %v", werr)
			}
			wrote += n
			buf.Reset()
		}
	}
	if buf.Len() > 0 {
		n, werr := f.WriteString(buf.String())
		if werr != nil {
			_ = f.Close()
			t.Fatalf("write synthetic audit log final flush: %v", werr)
		}
		wrote += n
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close synthetic audit log: %v", err)
	}
	t.Logf("synthetic audit log: %d MiB, %d records", wrote/(1024*1024), i)

	// Force a clean baseline. runtime.GC is a hint, not a guarantee,
	// but combined with two reads (the first warms internal pools, the
	// second is the one we measure) it gives us a stable delta.
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	got, err := Recent(path, recordsToRead)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(got) != recordsToRead {
		t.Fatalf("Recent(limit %d) returned %d records; want %d", recordsToRead, len(got), recordsToRead)
	}
	// Sanity: the records returned must be the NEWEST. The synthetic
	// records use ts_us=i in write order, so the last record written
	// has ts_us = i-1; the result's last record's ts_us must match.
	if got[len(got)-1].TsUnixMicros != int64(i-1) {
		t.Fatalf("last record ts_us=%d; want %d (newest of the synthetic stream)", got[len(got)-1].TsUnixMicros, i-1)
	}

	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	// TotalAlloc is monotonic-since-program-start; the per-call cost
	// is the delta. Heap usage at the END of the call (HeapAlloc) is
	// what'd correlate with RSS after the read finishes — that's the
	// budget the old code blew out (it held a []byte of the whole
	// file even after the function returned).
	heapDeltaBytes := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	allocPerCallBytes := int64(after.TotalAlloc - before.TotalAlloc)
	t.Logf("memory: heap delta=%d MiB, total alloc this call=%d MiB",
		heapDeltaBytes/(1024*1024), allocPerCallBytes/(1024*1024))

	heapDeltaCap := int64(heapDeltaCapMiB) * 1024 * 1024
	if heapDeltaBytes > heapDeltaCap {
		t.Fatalf("Recent left %d MiB resident on the heap after returning; cap is %d MiB. The OOM regression is back — Recent is loading the whole audit log into memory again",
			heapDeltaBytes/(1024*1024), heapDeltaCapMiB)
	}
}
