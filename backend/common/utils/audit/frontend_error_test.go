package audit_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neochaotic/powerlab/backend/common/utils/audit"
)

// Frontend errors are captured by the SvelteKit shell via window.onerror /
// unhandledrejection and POSTed to /v1/audit/frontend-error. They land in
// the SAME JSONL file as HTTP audit records (one storage layer, one
// retention policy, one ring buffer) but carry a Kind discriminator so
// readers can distinguish them.
//
// Backward compatibility: existing records have no `kind` field; the JSON
// tag is `omitempty` so the absence reads as Kind == "" == implicit
// "request". A new record with Kind = "ui_error" carries a Payload map
// with the browser-supplied fields (message, stack, url, user_agent,
// viewport).

func TestRecord_UIErrorKind_RoundTripsViaJSON(t *testing.T) {
	uid := int64(42)
	uname := "alice"
	rec := audit.Record{
		Kind:     "ui_error",
		Method:   "POST",
		Path:     "/v1/audit/frontend-error",
		Status:   202,
		UserID:   &uid,
		Username: &uname,
		RemoteIP: "192.168.1.5",
		Payload: map[string]any{
			"message": "TypeError: Cannot read properties of undefined (reading 'foo')",
			"stack":   "TypeError: Cannot read properties of undefined\n  at /apps/+page:42",
			"url":     "/apps",
			"ua":      "Mozilla/5.0",
			"viewport": map[string]any{
				"w": float64(1920),
				"h": float64(1080),
			},
		},
	}
	rec.FillTimestamps(time.Now())

	b, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"kind":"ui_error"`) {
		t.Errorf("marshalled record missing kind=ui_error: %s", b)
	}
	if !strings.Contains(string(b), `"payload":`) {
		t.Errorf("marshalled record missing payload field: %s", b)
	}

	var back audit.Record
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Kind != "ui_error" {
		t.Errorf("Kind: got %q, want ui_error", back.Kind)
	}
	if msg, _ := back.Payload["message"].(string); !strings.Contains(msg, "TypeError") {
		t.Errorf("payload.message round-trip lost: %v", back.Payload)
	}
}

func TestRecord_EmptyKind_OmitsFieldInJSON(t *testing.T) {
	// HTTP-request records (the existing schema) must continue to
	// marshal WITHOUT a "kind" field so existing JSONL files and
	// downstream consumers stay byte-identical.
	uid := int64(1)
	rec := audit.Record{
		Method:        "GET",
		Path:          "/v1/audit/recent",
		Status:        200,
		LatencyMicros: 1234,
		UserID:        &uid,
		RemoteIP:      "loopback",
	}
	rec.FillTimestamps(time.Now())

	b, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), `"kind"`) {
		t.Errorf("legacy HTTP record must not emit kind field, got: %s", b)
	}
	if strings.Contains(string(b), `"payload"`) {
		t.Errorf("legacy HTTP record must not emit payload field, got: %s", b)
	}
}

func TestStore_AppendBatch_RoundTripsUIErrorThroughJSONLFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	s, err := audit.NewStore(audit.StoreOptions{Path: path, RingCapacity: 10})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	uid := int64(7)
	rec := audit.Record{
		Kind:     "ui_error",
		Method:   "POST",
		Path:     "/v1/audit/frontend-error",
		Status:   202,
		UserID:   &uid,
		RemoteIP: "192.168.1.5",
		Payload: map[string]any{
			"message": "boom",
			"url":     "/settings",
		},
	}
	rec.FillTimestamps(time.Now())

	if err := s.AppendBatch(context.Background(), []audit.Record{rec}); err != nil {
		t.Fatalf("AppendBatch: %v", err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read jsonl: %v", err)
	}
	if !strings.Contains(string(body), `"kind":"ui_error"`) {
		t.Errorf("jsonl line missing kind: %s", body)
	}

	recent := s.Recent(audit.RecentOptions{Limit: 10})
	if len(recent) != 1 {
		t.Fatalf("recent count: got %d, want 1", len(recent))
	}
	if recent[0].Kind != "ui_error" {
		t.Errorf("ring record Kind: got %q, want ui_error", recent[0].Kind)
	}
	if msg, _ := recent[0].Payload["message"].(string); msg != "boom" {
		t.Errorf("ring record payload lost: %+v", recent[0].Payload)
	}
}
