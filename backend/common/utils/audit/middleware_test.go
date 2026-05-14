package audit_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/neochaotic/powerlab/backend/common/utils/audit"
)

// helper: spin up an echo handler wired to the audit middleware,
// fire one request, drain the recorder, and return the persisted
// record (or nil if none was written).
//
// Mirrors the production setup: JWT middleware runs first (we
// simulate it by manually setting user_id/user_name headers before
// invoking the handler), then audit middleware, then the handler.
func runOneRequest(t *testing.T, method, path string, setup func(req *http.Request), opts audit.MiddlewareOptions) (audit.Record, bool) {
	t.Helper()

	db, err := audit.OpenDB(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	rec := audit.NewRecorder(db, audit.RecorderOptions{
		Capacity:   16,
		BatchSize:  1, // flush every record so the test doesn't race the 200ms timer
		MaxLatency: 10 * time.Millisecond,
	})
	t.Cleanup(rec.Close)

	e := echo.New()
	e.Use(audit.Middleware(rec, opts))
	e.Any("/*", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(method, path, nil)
	if setup != nil {
		setup(req)
	}
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req)

	// Wait for the writer to flush.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		n, _ := db.Count(context.Background())
		if n >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	row, err := db.GetMostRecent(context.Background())
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return audit.Record{}, false
		}
		t.Fatalf("GetMostRecent: %v", err)
	}
	return row, true
}

func TestMiddleware_RecordsBasicRequest(t *testing.T) {
	got, ok := runOneRequest(t, http.MethodGet, "/v2/app_management/compose", nil, audit.MiddlewareOptions{})
	if !ok {
		t.Fatal("no record persisted")
	}
	if got.Method != "GET" {
		t.Errorf("Method = %q, want GET", got.Method)
	}
	if got.Path != "/v2/app_management/compose" {
		t.Errorf("Path = %q, want /v2/app_management/compose", got.Path)
	}
	if got.Status != 200 {
		t.Errorf("Status = %d, want 200", got.Status)
	}
	if got.LatencyMicros <= 0 {
		t.Errorf("LatencyMicros = %d, want > 0", got.LatencyMicros)
	}
}

func TestMiddleware_StripsTokenFromQuery(t *testing.T) {
	got, ok := runOneRequest(t, http.MethodGet, "/v2/foo?token=secret.jwt.bar&limit=10", nil, audit.MiddlewareOptions{})
	if !ok {
		t.Fatal("no record persisted")
	}
	if strings.Contains(got.Query, "token") || strings.Contains(got.Query, "secret") {
		t.Errorf("Query leaked token: %q", got.Query)
	}
	if !strings.Contains(got.Query, "limit=10") {
		t.Errorf("Query lost non-token param: %q (want limit=10)", got.Query)
	}
}

func TestMiddleware_CapturesUserFromJWTHeaders(t *testing.T) {
	got, ok := runOneRequest(t, http.MethodPost, "/v1/users/whoami",
		func(req *http.Request) {
			// Simulate what jwt.JWT() ParseTokenFunc does after
			// validating: stamps user_id (and optionally user_name)
			// onto the request headers for downstream handlers.
			req.Header.Set("user_id", "42")
			req.Header.Set("user_name", "alice")
		},
		audit.MiddlewareOptions{})
	if !ok {
		t.Fatal("no record persisted")
	}
	if got.UserID == nil || *got.UserID != 42 {
		t.Errorf("UserID = %v, want *42", got.UserID)
	}
	if got.Username == nil || *got.Username != "alice" {
		t.Errorf("Username = %v, want *alice", got.Username)
	}
}

func TestMiddleware_LoopbackRemoteIP_CollapsedToSentinel(t *testing.T) {
	got, ok := runOneRequest(t, http.MethodGet, "/v1/sys/utilization",
		func(req *http.Request) {
			// httptest.NewRequest produces 192.0.2.1 by default for
			// RemoteAddr; force loopback so the sentinel branch fires.
			req.RemoteAddr = "127.0.0.1:50100"
		},
		audit.MiddlewareOptions{})
	if !ok {
		t.Fatal("no record persisted")
	}
	if got.RemoteIP != audit.LoopbackSentinel {
		t.Errorf("RemoteIP = %q, want %q", got.RemoteIP, audit.LoopbackSentinel)
	}
}

func TestMiddleware_PassesThroughRequestID(t *testing.T) {
	const wantID = "test-correlation-1234"
	got, ok := runOneRequest(t, http.MethodGet, "/v1/sys/utilization",
		func(req *http.Request) { req.Header.Set(echo.HeaderXRequestID, wantID) },
		audit.MiddlewareOptions{})
	if !ok {
		t.Fatal("no record persisted")
	}
	if got.RequestID != wantID {
		t.Errorf("RequestID = %q, want %q", got.RequestID, wantID)
	}
}

func TestMiddleware_SkipperBypassesRecording(t *testing.T) {
	db, err := audit.OpenDB(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rec := audit.NewRecorder(db, audit.RecorderOptions{Capacity: 16, BatchSize: 1, MaxLatency: 10 * time.Millisecond})
	defer rec.Close()

	e := echo.New()
	e.Use(audit.Middleware(rec, audit.MiddlewareOptions{
		Skipper: func(c echo.Context) bool {
			return c.Path() == "/v1/sys/heartbeat" || strings.HasPrefix(c.Request().URL.Path, "/v1/sys/heartbeat")
		},
	}))
	e.GET("/v1/sys/heartbeat", func(c echo.Context) error { return c.NoContent(http.StatusOK) })
	e.GET("/v1/users/info", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	for _, p := range []string{"/v1/sys/heartbeat", "/v1/users/info"} {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
	}

	// Wait briefly for any writes.
	time.Sleep(150 * time.Millisecond)
	n, _ := db.Count(context.Background())
	if n != 1 {
		t.Errorf("recorded %d requests, want 1 (heartbeat must be skipped)", n)
	}
}

// belt-and-suspenders: confirm the parser used for ID arithmetic
// handles negative + zero cases so the JWT header → UserID extraction
// doesn't silently misbehave on edge values. Locks the contract.
func TestMiddleware_UserIDParsing_Edges(t *testing.T) {
	for _, tc := range []struct {
		header string
		want   *int64
	}{
		{"", nil},
		{"0", ptrInt64(0)},
		{"not-a-number", nil},
		{"-1", ptrInt64(-1)},
		{strconv.FormatInt(int64(1<<62), 10), ptrInt64(1 << 62)},
	} {
		got, ok := runOneRequest(t, http.MethodGet, "/v1/foo",
			func(req *http.Request) {
				if tc.header != "" {
					req.Header.Set("user_id", tc.header)
				}
			},
			audit.MiddlewareOptions{})
		if !ok {
			t.Fatalf("[%s] no record persisted", tc.header)
		}
		if (got.UserID == nil) != (tc.want == nil) {
			t.Errorf("[%s] UserID nilness mismatch: got=%v want=%v", tc.header, got.UserID, tc.want)
			continue
		}
		if got.UserID != nil && *got.UserID != *tc.want {
			t.Errorf("[%s] UserID = %d, want %d", tc.header, *got.UserID, *tc.want)
		}
	}
}

func ptrInt64(v int64) *int64 { return &v }
