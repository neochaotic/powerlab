package audit_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/neochaotic/powerlab/backend/common/utils/audit"
)

func makeServiceForMW(t *testing.T) *audit.Service {
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

func waitRecorderFlush(t *testing.T) {
	t.Helper()
	// recorder default flush latency is 200ms — wait a bit longer
	// to be safe against scheduler jitter.
	time.Sleep(300 * time.Millisecond)
}

// ─── Echo middleware ────────────────────────────────────────────────────────

func TestEchoMiddleware_CapturesBasicRequest(t *testing.T) {
	svc := makeServiceForMW(t)

	e := echo.New()
	e.Use(audit.Middleware(svc.Recorder, audit.MiddlewareOptions{}))
	e.GET("/x", func(c echo.Context) error { return c.String(200, "ok") })

	srv := httptest.NewServer(e)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/x", nil)
	req.Header.Set("user_id", "42")
	req.Header.Set("user_name", "alisson")
	resp, _ := http.DefaultClient.Do(req)
	_ = resp.Body.Close()

	waitRecorderFlush(t)
	recent := svc.Store.Recent(audit.RecentOptions{Limit: 10})
	if len(recent) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recent))
	}
	r := recent[0]
	if r.Path != "/x" || r.Method != "GET" || r.Status != 200 {
		t.Errorf("captured wrong fields: %+v", r)
	}
	if r.UserID == nil || *r.UserID != 42 {
		t.Errorf("user_id not extracted from header: %+v", r.UserID)
	}
	if r.Username == nil || *r.Username != "alisson" {
		t.Errorf("username not extracted: %+v", r.Username)
	}
}

func TestEchoMiddleware_LoopbackCollapsedToSentinel(t *testing.T) {
	svc := makeServiceForMW(t)
	e := echo.New()
	e.Use(audit.Middleware(svc.Recorder, audit.MiddlewareOptions{}))
	e.GET("/y", func(c echo.Context) error { return c.String(200, "ok") })

	srv := httptest.NewServer(e)
	defer srv.Close()

	_, _ = http.Get(srv.URL + "/y")
	waitRecorderFlush(t)
	recent := svc.Store.Recent(audit.RecentOptions{Limit: 10})
	if len(recent) == 0 {
		t.Fatal("no record captured")
	}
	if recent[0].RemoteIP != audit.LoopbackSentinel {
		t.Errorf("expected loopback sentinel, got %q", recent[0].RemoteIP)
	}
}

func TestEchoMiddleware_StripsTokenQuery(t *testing.T) {
	svc := makeServiceForMW(t)
	e := echo.New()
	e.Use(audit.Middleware(svc.Recorder, audit.MiddlewareOptions{}))
	e.GET("/sse", func(c echo.Context) error { return c.String(200, "ok") })

	srv := httptest.NewServer(e)
	defer srv.Close()

	_, _ = http.Get(srv.URL + "/sse?token=SECRET&channel=audit")
	waitRecorderFlush(t)
	recent := svc.Store.Recent(audit.RecentOptions{Limit: 10})
	if strings.Contains(recent[0].Query, "SECRET") {
		t.Errorf("token leaked to audit: %q", recent[0].Query)
	}
	if !strings.Contains(recent[0].Query, "channel=audit") {
		t.Errorf("other params should survive: %q", recent[0].Query)
	}
}

func TestEchoMiddleware_Skipper(t *testing.T) {
	svc := makeServiceForMW(t)
	e := echo.New()
	e.Use(audit.Middleware(svc.Recorder, audit.MiddlewareOptions{
		Skipper: func(c echo.Context) bool {
			return c.Request().URL.Path == "/ping"
		},
	}))
	e.GET("/ping", func(c echo.Context) error { return c.String(200, "ok") })
	e.GET("/x", func(c echo.Context) error { return c.String(200, "ok") })

	srv := httptest.NewServer(e)
	defer srv.Close()

	_, _ = http.Get(srv.URL + "/ping")
	_, _ = http.Get(srv.URL + "/x")
	waitRecorderFlush(t)
	recent := svc.Store.Recent(audit.RecentOptions{Limit: 10})
	if len(recent) != 1 {
		t.Fatalf("expected 1 (only /x), got %d", len(recent))
	}
	if recent[0].Path != "/x" {
		t.Errorf("wrong path captured: %s", recent[0].Path)
	}
}

// ─── stdlib middleware ───────────────────────────────────────────────────────

func TestHTTPMiddleware_CapturesAndStatusWrap(t *testing.T) {
	svc := makeServiceForMW(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})
	mw := audit.HTTPMiddleware(svc.Recorder, audit.HTTPMiddlewareOptions{})
	wrapped := mw(mux)
	srv := httptest.NewServer(wrapped)
	defer srv.Close()

	req, _ := http.NewRequest("POST", srv.URL+"/api?token=SECRET", nil)
	req.Header.Set("user_id", "7")
	resp, _ := http.DefaultClient.Do(req)
	_ = resp.Body.Close()

	waitRecorderFlush(t)
	recent := svc.Store.Recent(audit.RecentOptions{Limit: 10})
	if len(recent) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recent))
	}
	r := recent[0]
	if r.Method != "POST" {
		t.Errorf("method: %q", r.Method)
	}
	if r.Status != 404 {
		t.Errorf("status not captured: got %d, want 404", r.Status)
	}
	if r.UserID == nil || *r.UserID != 7 {
		t.Errorf("user_id not extracted: %+v", r.UserID)
	}
	if strings.Contains(r.Query, "SECRET") {
		t.Errorf("token leaked: %q", r.Query)
	}
}

func TestHTTPMiddleware_ImplicitStatusOK(t *testing.T) {
	svc := makeServiceForMW(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mw := audit.HTTPMiddleware(svc.Recorder, audit.HTTPMiddlewareOptions{})
	srv := httptest.NewServer(mw(mux))
	defer srv.Close()

	_, _ = http.Get(srv.URL + "/")
	waitRecorderFlush(t)
	recent := svc.Store.Recent(audit.RecentOptions{Limit: 1})
	if recent[0].Status != 200 {
		t.Errorf("implicit Write should record 200, got %d", recent[0].Status)
	}
}

func TestHTTPMiddleware_Skipper(t *testing.T) {
	svc := makeServiceForMW(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mw := audit.HTTPMiddleware(svc.Recorder, audit.HTTPMiddlewareOptions{
		Skipper: func(r *http.Request) bool { return r.URL.Path == "/ping" },
	})
	srv := httptest.NewServer(mw(mux))
	defer srv.Close()
	_, _ = http.Get(srv.URL + "/ping")
	_, _ = http.Get(srv.URL + "/api")
	waitRecorderFlush(t)
	recent := svc.Store.Recent(audit.RecentOptions{Limit: 10})
	if len(recent) != 1 || recent[0].Path != "/api" {
		t.Fatalf("skipper failed: %+v", recent)
	}
}

// ─── HTTP handlers (stdlib variant) ──────────────────────────────────────────

func TestRecentHTTPHandler_ReturnsJSON(t *testing.T) {
	svc := makeServiceForMW(t)
	now := time.Now()
	_ = svc.Store.AppendBatch(context.Background(), []audit.Record{
		sampleRecord(now, 1, "/x"),
	})

	h := audit.RecentHTTPHandler(svc.Store)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/v1/audit/recent", nil)
	h(w, r)

	if got := w.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("Content-Type: %q, want JSON", got)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"path":"/x"`) {
		t.Errorf("body missing recorded path: %s", body)
	}
}

func TestStatsHTTPHandler_ReturnsJSON(t *testing.T) {
	svc := makeServiceForMW(t)
	now := time.Now()
	_ = svc.Store.AppendBatch(context.Background(), []audit.Record{
		sampleRecord(now, 1, "/x"),
	})

	h := audit.StatsHTTPHandler(svc.Store)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/v1/audit/stats", nil)
	h(w, r)
	if w.Code != 200 {
		t.Errorf("status: %d", w.Code)
	}
	if got := w.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("Content-Type: %q, want JSON", got)
	}
	if !strings.Contains(w.Body.String(), `"row_count":1`) {
		t.Errorf("body missing row_count=1: %s", w.Body.String())
	}
}

func TestRecentHTTPHandler_MethodNotAllowed(t *testing.T) {
	svc := makeServiceForMW(t)
	h := audit.RecentHTTPHandler(svc.Store)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/audit/recent", nil)
	h(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: %d, want 405", w.Code)
	}
}

// ─── Recorder drop-oldest ────────────────────────────────────────────────────

func TestRecorder_DropOldestUnderBackpressure(t *testing.T) {
	store, _ := audit.NewStore(audit.StoreOptions{
		Path:         filepath.Join(t.TempDir(), "audit.jsonl"),
		RingCapacity: 100,
	})
	defer store.Close()

	// Tight channel → ensure drop path.
	rec := audit.NewRecorder(store, audit.RecorderOptions{
		Capacity:   2,
		BatchSize:  100,
		MaxLatency: time.Hour, // never flush via timer in this test
	})
	defer rec.Close()

	for i := 0; i < 50; i++ {
		var r audit.Record
		r.Path = "/x"
		rec.Submit(r)
	}
	if rec.Dropped() == 0 {
		t.Errorf("expected non-zero drops with capacity 2 + 50 submits, got %d", rec.Dropped())
	}
}

// Regression: the audit middleware wraps the ResponseWriter to
// capture status codes. The wrapper MUST forward Flush() to the
// underlying ResponseWriter, or any SSE / chunked-transfer handler
// downstream silently buffers — the user sees the modal "loading"
// forever because log chunks never reach the browser until the
// stream closes.
//
// User-reported in v0.6.13-stg: "fica carregando muito tempo, depois
// fica em instalando e nao aparece tela de logs" — root cause traced
// to the missing Flusher interface forwarding here.
func TestHTTPMiddleware_ResponseWriterImplementsFlusher(t *testing.T) {
	svc := makeServiceForMW(t)

	var sawFlusher bool
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_, sawFlusher = w.(http.Flusher)
		w.WriteHeader(200)
	})

	mw := audit.HTTPMiddleware(svc.Recorder, audit.HTTPMiddlewareOptions{})
	srv := httptest.NewServer(mw(mux))
	defer srv.Close()

	_, _ = http.Get(srv.URL + "/")
	if !sawFlusher {
		t.Error("BUG: audit middleware response wrapper does NOT implement http.Flusher. SSE handlers downstream cannot flush chunks → modal `loading` forever during install.")
	}
}

func TestHTTPMiddleware_FlushPassesThroughToUnderlying(t *testing.T) {
	svc := makeServiceForMW(t)

	flushed := false
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("data: hello\n\n"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
			flushed = true
		}
	})

	mw := audit.HTTPMiddleware(svc.Recorder, audit.HTTPMiddlewareOptions{})
	srv := httptest.NewServer(mw(mux))
	defer srv.Close()

	_, _ = http.Get(srv.URL + "/")
	if !flushed {
		t.Error("BUG: handler could not Flush through the audit-middleware wrapper")
	}
}
