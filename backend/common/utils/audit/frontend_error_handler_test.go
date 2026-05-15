package audit_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neochaotic/powerlab/backend/common/utils/audit"
)

// Test the POST /v1/audit/frontend-error endpoint that the SvelteKit
// shell calls from window.onerror / unhandledrejection. JWT auth is
// wrapped at the gateway level (HTTPJWT middleware) — this handler
// trusts the caller is authenticated, same contract as the rest of
// the stdlib audit handlers.

func mkFrontendErrSvc(t *testing.T) *audit.Service {
	t.Helper()
	svc, err := audit.NewService(audit.ServiceOptions{
		Path: filepath.Join(t.TempDir(), "audit.jsonl"),
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

func TestFrontendErrorHandler_AcceptsValidBodyAndRecordsRecord(t *testing.T) {
	svc := mkFrontendErrSvc(t)
	h := audit.FrontendErrorHTTPHandler(svc.Recorder)

	body := map[string]any{
		"message": "TypeError: Cannot read properties of undefined (reading 'foo')",
		"stack":   "at /apps/+page.svelte:42:7\nat onMount",
		"url":     "/apps",
		"ua":      "Mozilla/5.0 ...",
		"viewport": map[string]any{
			"w": 1920,
			"h": 1080,
		},
	}
	buf, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/audit/frontend-error", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("user_id", "42")
	req.Header.Set("user_name", "alice")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status: got %d, want 202; body=%s", rec.Code, rec.Body.String())
	}

	// Submit is async — give the recorder a tick to flush.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(svc.Store.Recent(audit.RecentOptions{Limit: 10})) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	recs := svc.Store.Recent(audit.RecentOptions{Limit: 10})
	if len(recs) != 1 {
		t.Fatalf("recorded count: got %d, want 1", len(recs))
	}
	r := recs[0]
	if r.Kind != "ui_error" {
		t.Errorf("Kind: got %q, want ui_error", r.Kind)
	}
	if r.UserID == nil || *r.UserID != 42 {
		t.Errorf("UserID: got %v, want 42", r.UserID)
	}
	if r.Username == nil || *r.Username != "alice" {
		t.Errorf("Username: got %v, want alice", r.Username)
	}
	if msg, _ := r.Payload["message"].(string); !strings.Contains(msg, "TypeError") {
		t.Errorf("payload.message lost: %+v", r.Payload)
	}
}

func TestFrontendErrorHandler_RejectsEmptyMessage(t *testing.T) {
	svc := mkFrontendErrSvc(t)
	h := audit.FrontendErrorHTTPHandler(svc.Recorder)

	body := map[string]any{"message": "", "url": "/x"}
	buf, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/audit/frontend-error", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty message: got %d, want 400", rec.Code)
	}
}

func TestFrontendErrorHandler_RejectsNonPOST(t *testing.T) {
	svc := mkFrontendErrSvc(t)
	h := audit.FrontendErrorHTTPHandler(svc.Recorder)

	req := httptest.NewRequest(http.MethodGet, "/v1/audit/frontend-error", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET: got %d, want 405", rec.Code)
	}
}

func TestFrontendErrorHandler_RejectsOversizedBody(t *testing.T) {
	svc := mkFrontendErrSvc(t)
	h := audit.FrontendErrorHTTPHandler(svc.Recorder)

	// 17 KiB payload — over the 16 KiB limit. A genuine browser
	// stack trace fits in 4–8 KiB comfortably; anything bigger is
	// either a runaway loop or an exfiltration attempt.
	big := strings.Repeat("x", 17*1024)
	body := map[string]any{"message": "boom", "stack": big, "url": "/x"}
	buf, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/audit/frontend-error", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized: got %d, want 413; body=%s", rec.Code, rec.Body.String())
	}
}

func TestFrontendErrorHandler_RejectsMalformedJSON(t *testing.T) {
	svc := mkFrontendErrSvc(t)
	h := audit.FrontendErrorHTTPHandler(svc.Recorder)

	req := httptest.NewRequest(http.MethodPost, "/v1/audit/frontend-error",
		bytes.NewReader([]byte(`{"message": "boom"`))) // unterminated
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed: got %d, want 400", rec.Code)
	}
}
