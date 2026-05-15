package logs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestJournalLineToSSE_SeverityMapping(t *testing.T) {
	cases := []struct {
		name     string
		priority string
		want     string
	}{
		// Per systemd journal severity levels (RFC 5424). Sprint 21
		// Phase 4 buckets into 4 colors: ERROR / WARN / INFO / DEBUG.
		{"emerg (0) → error", "0", "error"},
		{"alert (1) → error", "1", "error"},
		{"crit (2) → error", "2", "error"},
		{"err (3) → error", "3", "error"},
		{"warn (4) → warn", "4", "warn"},
		{"notice (5) → info", "5", "info"},
		{"info (6) → info", "6", "info"},
		{"debug (7) → debug", "7", "debug"},
		{"missing → info (default)", "", "info"},
		{"unparseable → info (default)", "abc", "info"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapJournalPriority(tc.priority)
			if got != tc.want {
				t.Errorf("priority=%q: got %s, want %s", tc.priority, got, tc.want)
			}
		})
	}
}

func TestJournalLineToSSE_ParseAndEmit(t *testing.T) {
	// A typical journalctl JSON line. We don't care about every field;
	// we extract MESSAGE + PRIORITY + __REALTIME_TIMESTAMP and emit a
	// compact SSE data envelope so the UI doesn't re-parse.
	line := []byte(`{"PRIORITY":"3","MESSAGE":"oh no","__REALTIME_TIMESTAMP":"1715800000000000","_SYSTEMD_UNIT":"powerlab-gateway.service"}`)
	out, ok := journalLineToSSE(line)
	if !ok {
		t.Fatalf("expected ok=true for valid JSON line")
	}
	s := string(out)
	if !strings.HasPrefix(s, "data: ") {
		t.Errorf("expected SSE data: prefix, got %q", s)
	}
	if !strings.HasSuffix(s, "\n\n") {
		t.Errorf("expected SSE blank-line terminator, got %q", s)
	}
	if !strings.Contains(s, `"severity":"error"`) {
		t.Errorf("expected severity=error in SSE payload, got %q", s)
	}
	if !strings.Contains(s, `"message":"oh no"`) {
		t.Errorf("expected message field in SSE payload, got %q", s)
	}
}

func TestJournalLineToSSE_RejectsInvalidJSON(t *testing.T) {
	// journalctl occasionally emits non-JSON lines (e.g. its own
	// "-- Reboot --" markers). Skip these silently.
	_, ok := journalLineToSSE([]byte(`-- Reboot --`))
	if ok {
		t.Errorf("expected ok=false for non-JSON line")
	}
}

func TestStreamJournaldHTTPHandler_RejectsInvalidServiceName(t *testing.T) {
	cases := []struct {
		name    string
		service string
		want    int
	}{
		// Path-traversal of the form `../etc` gets normalised by net/url
		// before our handler sees it (so the segment `..` never arrives
		// literally), but a literal `..` token CAN arrive when the client
		// percent-encodes it. Test the realistic post-decode form.
		{"path-segment dotdot", "..", http.StatusBadRequest},
		{"uppercase", "Gateway", http.StatusBadRequest},
		{"semicolon injection", "gateway;ls", http.StatusBadRequest},
		{"backtick injection", "gateway`id`", http.StatusBadRequest},
		{"empty", "", http.StatusBadRequest},
		{"too long", strings.Repeat("a", 65), http.StatusBadRequest},
		{"starts with digit", "1gateway", http.StatusBadRequest},
		{"valid lowercase", "gateway", -1}, // -1 = expect anything except 400
		{"valid with hyphen", "app-management", -1},
		{"valid with digit", "gateway2", -1},
	}
	h := StreamJournaldHTTPHandler("powerlab-")
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Build the request with a placeholder path, then assign
			// URL.Path directly so net/url doesn't normalise dotdot
			// segments out of our test data. The handler reads
			// r.URL.Path, so this is the realistic post-decode shape.
			req := httptest.NewRequest(http.MethodGet, "/v1/logs/services/x/stream", nil)
			req.URL.Path = "/v1/logs/services/" + tc.service + "/stream"
			ctx, cancel := context.WithTimeout(req.Context(), 100*time.Millisecond)
			defer cancel()
			req = req.WithContext(ctx)
			w := httptest.NewRecorder()
			h(w, req)
			if tc.want == http.StatusBadRequest && w.Code != http.StatusBadRequest {
				t.Errorf("service=%q: got %d, want %d", tc.service, w.Code, tc.want)
			}
			if tc.want == -1 && w.Code == http.StatusBadRequest {
				t.Errorf("service=%q: got 400, expected accepted (any other code)", tc.service)
			}
		})
	}
}

func TestStreamJournaldHTTPHandler_RejectsNonGET(t *testing.T) {
	h := StreamJournaldHTTPHandler("powerlab-")
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(m, "/v1/logs/services/gateway/stream", nil)
		w := httptest.NewRecorder()
		h(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("method %s: got %d, want 405", m, w.Code)
		}
	}
}

func TestStreamJournaldHTTPHandler_SetsSSEHeaders(t *testing.T) {
	// The handler must set the canonical SSE headers BEFORE the first
	// flush. Without them, browsers buffer the response instead of
	// streaming. Sprint 18 (#384) regression class.
	h := StreamJournaldHTTPHandler("powerlab-")
	req := httptest.NewRequest(http.MethodGet, "/v1/logs/services/gateway/stream", nil)
	ctx, cancel := context.WithTimeout(req.Context(), 50*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	h(w, req)
	hdr := w.Result().Header
	if got := hdr.Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Errorf("Content-Type: got %q, want text/event-stream", got)
	}
	if got := hdr.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control: got %q, want no-cache", got)
	}
	if got := hdr.Get("X-Accel-Buffering"); got != "no" {
		// X-Accel-Buffering=no defeats nginx/proxy buffering. Without
		// this, reverse-proxy deployments don't see the stream.
		t.Errorf("X-Accel-Buffering: got %q, want no", got)
	}
}
