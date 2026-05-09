package foundation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/neochaotic/powerlab/backend/pkg/foundation"
	"github.com/neochaotic/powerlab/backend/pkg/logging"
	"github.com/neochaotic/powerlab/backend/pkg/tracing"
)

// captureLogger returns a logger that writes to an in-memory buffer so
// tests can assert on emitted log lines.
func captureLogger(t *testing.T) (logging.Logger, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	l, err := logging.New(logging.Config{
		Level:  "debug",
		Format: "json",
		Writer: &buf,
	})
	if err != nil {
		t.Fatalf("captureLogger: logging.New failed: %v", err)
	}
	return l, &buf
}

// TestWrap_PassesThroughNonPanicHandler is the happy path: a normal
// handler is reached unmodified, status code propagates, body is the
// handler's body. If this fails, the foundation chain is breaking
// requests that should succeed.
func TestWrap_PassesThroughNonPanicHandler(t *testing.T) {
	log, _ := captureLogger(t)
	handler := foundation.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("brewing"))
	}), log)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Errorf("status: want 418, got %d", rec.Code)
	}
	if rec.Body.String() != "brewing" {
		t.Errorf("body: want 'brewing', got %q", rec.Body.String())
	}
}

// TestWrap_PanicReturns500 is the structural close for bug-#64 SIGSEGV.
// A handler that panics MUST yield a 500 response and MUST NOT crash
// the test process. This is the single test that proves the
// composition every PowerLab service uses on its
// http.Server.Handler effectively contains panics.
func TestWrap_PanicReturns500(t *testing.T) {
	log, _ := captureLogger(t)
	handler := foundation.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("simulated nil deref")
	}), log)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)

	// Must not propagate the panic — if Wrap is misconfigured,
	// this call panics and the test process dies.
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: want 500, got %d", rec.Code)
	}
}

// TestWrap_PanicLogIncludesCorrelationID verifies the second half of
// the foundation contract: when a panic is recovered, the emitted log
// line carries the correlation_id from request context. This is the
// thing that makes panic incidents debuggable in production — without
// the correlation_id, panics in a busy service are unattributable.
func TestWrap_PanicLogIncludesCorrelationID(t *testing.T) {
	log, buf := captureLogger(t)
	handler := foundation.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}), log)

	const wantID = "test-correlation-id-42"
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	req.Header.Set("X-Request-Id", wantID)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if !strings.Contains(buf.String(), wantID) {
		t.Errorf("expected log to contain correlation_id %q, got: %s", wantID, buf.String())
	}
	if !strings.Contains(buf.String(), "panic recovered in handler") {
		t.Errorf("expected log to contain 'panic recovered in handler', got: %s", buf.String())
	}
}

// TestWrap_HappyPathPropagatesCorrelationID verifies the tracing layer
// is wired BEFORE the recovery layer (so the recovery handler can read
// the correlation_id from context). Verifies via a handler that reads
// the ID from its context and writes it to the response.
func TestWrap_HappyPathPropagatesCorrelationID(t *testing.T) {
	log, _ := captureLogger(t)
	handler := foundation.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := tracing.FromContext(r.Context())
		_, _ = io.WriteString(w, id)
	}), log)

	const wantID = "abc-123"
	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	req.Header.Set("X-Request-Id", wantID)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Body.String() != wantID {
		t.Errorf("correlation_id: want %q in response, got %q", wantID, rec.Body.String())
	}
}

// TestWrap_PanicBodyIsErrInternalShape verifies the body returned for
// a recovered panic matches the canonical pkg/errors.WriteHTTP shape.
// Clients (UI, CLI) parse this shape — drifting from it breaks every
// downstream consumer of the error envelope.
func TestWrap_PanicBodyIsErrInternalShape(t *testing.T) {
	log, _ := captureLogger(t)
	handler := foundation.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}), log)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	handler.ServeHTTP(rec, req)

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("response body must be JSON: %v (body=%s)", err, rec.Body.String())
	}
	// pkg/errors.WriteHTTP envelope: {code, i18n_key, ...}.
	// correlation_id is injected by the tracing layer above us.
	if _, ok := body["code"]; !ok {
		t.Errorf("response body missing 'code' field: %v", body)
	}
	if _, ok := body["i18n_key"]; !ok {
		t.Errorf("response body missing 'i18n_key' field: %v", body)
	}
	if _, ok := body["correlation_id"]; !ok {
		t.Errorf("response body missing 'correlation_id' field: %v", body)
	}
}

// TestWrap_NilLoggerDoesNotPanic guards against a misconfigured
// service passing a nil logger into Wrap. The Wrap helper MUST tolerate
// this case rather than nil-pointer-deref'ing the first request.
//
// (Defensive: we never want a misconfiguration to cause the bug class
// the foundation is supposed to fix.)
func TestWrap_NilLoggerDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Wrap with nil logger panicked at construction: %v", r)
		}
	}()
	handler := foundation.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(context.Background())
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: want 200, got %d", rec.Code)
	}
}
