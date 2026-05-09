package lifecycle_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/neochaotic/powerlab/backend/pkg/lifecycle"
	"github.com/neochaotic/powerlab/backend/pkg/logging"
)

// syncBuffer is a thread-safe bytes.Buffer for tests where the writer
// (logger) and the reader (assertions) can race — specifically the
// SafeGo tests where the panic-recovery log lands from a goroutine.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}
func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}
func (s *syncBuffer) Bytes() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]byte(nil), s.buf.Bytes()...)
}

func capturedLogger(t *testing.T) (logging.Logger, *syncBuffer) {
	t.Helper()
	buf := &syncBuffer{}
	l, err := logging.New(logging.Config{Level: "info", Format: "json", Writer: buf})
	if err != nil {
		t.Fatalf("logging.New: %v", err)
	}
	return l, buf
}

// --------------------------------------------------------------------
// RecoverMiddleware — HTTP panic recovery
// --------------------------------------------------------------------

func TestRecoverMiddleware_CatchesStringPanic(t *testing.T) {
	logger, buf := capturedLogger(t)
	h := lifecycle.RecoverMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("oh no")
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/test-path", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: want 500, got %d", rec.Code)
	}
	if !strings.Contains(buf.String(), "panic") {
		t.Errorf("expected panic to be logged; got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "/test-path") {
		t.Errorf("expected request path in log; got: %s", buf.String())
	}
}

func TestRecoverMiddleware_BodyMatchesErrInternalShape(t *testing.T) {
	logger, _ := capturedLogger(t)
	h := lifecycle.RecoverMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ptr *int
		_ = *ptr // nil-deref — same class as bug #64
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: want 500, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body should be valid JSON, got %s\nerr: %v", rec.Body.String(), err)
	}
	if body["code"] != "common.internal" {
		t.Errorf("body.code: want common.internal, got %v", body["code"])
	}
}

func TestRecoverMiddleware_ProcessKeepsRunning(t *testing.T) {
	// Three sequential requests; only the middle one panics. The other
	// two must complete normally.
	logger, _ := capturedLogger(t)
	calls := 0
	h := lifecycle.RecoverMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 2 {
			panic("intentional")
		}
		w.WriteHeader(http.StatusOK)
	}))

	for i := 1; i <= 3; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
		switch i {
		case 1, 3:
			if rec.Code != http.StatusOK {
				t.Errorf("request %d: want 200, got %d", i, rec.Code)
			}
		case 2:
			if rec.Code != http.StatusInternalServerError {
				t.Errorf("request %d: want 500, got %d", i, rec.Code)
			}
		}
	}
	if calls != 3 {
		t.Errorf("expected 3 handler invocations, got %d", calls)
	}
}

func TestRecoverMiddleware_NonPanicPathUnchanged(t *testing.T) {
	logger, buf := capturedLogger(t)
	h := lifecycle.RecoverMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("normal response"))
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if rec.Code != http.StatusTeapot {
		t.Errorf("status: want 418, got %d", rec.Code)
	}
	if rec.Body.String() != "normal response" {
		t.Errorf("body: want %q, got %q", "normal response", rec.Body.String())
	}
	// No panic means no panic log line.
	if strings.Contains(buf.String(), "panic") {
		t.Errorf("expected no panic log on happy path; got: %s", buf.String())
	}
}

// --------------------------------------------------------------------
// SafeGo — goroutine panic recovery
// --------------------------------------------------------------------

func TestSafeGo_RunsFunction(t *testing.T) {
	logger, _ := capturedLogger(t)
	var ran bool
	var mu sync.Mutex
	done := make(chan struct{})

	lifecycle.SafeGo(context.Background(), logger, func() {
		mu.Lock()
		ran = true
		mu.Unlock()
		close(done)
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("goroutine never completed")
	}

	mu.Lock()
	defer mu.Unlock()
	if !ran {
		t.Error("function did not run")
	}
}

func TestSafeGo_RecoversFromPanic(t *testing.T) {
	logger, buf := capturedLogger(t)
	done := make(chan struct{})

	lifecycle.SafeGo(context.Background(), logger, func() {
		defer close(done)
		panic("explosion")
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("goroutine never completed (panic likely escaped)")
	}

	// Tiny sync window for the deferred logger call to flush — without
	// it, on slow CI, the buffer may not have the line yet.
	time.Sleep(20 * time.Millisecond)

	if !strings.Contains(buf.String(), "panic") {
		t.Errorf("expected panic to be logged; got: %s", buf.String())
	}
}

func TestSafeGo_PropagatesCorrelationIDInPanicLog(t *testing.T) {
	logger, buf := capturedLogger(t)
	ctx := context.WithValue(context.Background(), logging.CorrelationIDKey{}, "req-safe-1")
	done := make(chan struct{})

	lifecycle.SafeGo(ctx, logger, func() {
		defer close(done)
		panic("with corr id")
	})

	<-done
	time.Sleep(20 * time.Millisecond)

	if !strings.Contains(buf.String(), "req-safe-1") {
		t.Errorf("expected correlation_id in panic log; got: %s", buf.String())
	}
}
