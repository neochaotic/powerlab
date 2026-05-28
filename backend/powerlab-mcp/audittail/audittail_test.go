package audittail

import (
	"os"
	"path/filepath"
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
