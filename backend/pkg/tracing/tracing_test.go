package tracing_test

import (
	"context"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/neochaotic/powerlab/backend/pkg/logging"
	"github.com/neochaotic/powerlab/backend/pkg/tracing"
)

// --------------------------------------------------------------------
// NewID — format & uniqueness
// --------------------------------------------------------------------

func TestNewID_Length32(t *testing.T) {
	id := tracing.NewID()
	if len(id) != 32 {
		t.Errorf("len(NewID): want 32, got %d (%q)", len(id), id)
	}
}

func TestNewID_HexEncoded(t *testing.T) {
	id := tracing.NewID()
	if _, err := hex.DecodeString(id); err != nil {
		t.Errorf("NewID should be valid hex; got %q (err: %v)", id, err)
	}
}

func TestNewID_Unique(t *testing.T) {
	const n = 1000
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		id := tracing.NewID()
		if _, exists := seen[id]; exists {
			t.Fatalf("collision after %d iterations: %q", i, id)
		}
		seen[id] = struct{}{}
	}
}

// --------------------------------------------------------------------
// Context helpers
// --------------------------------------------------------------------

func TestFromContext_Empty_ReturnsEmpty(t *testing.T) {
	if got := tracing.FromContext(context.Background()); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestFromContext_Nil_ReturnsEmpty(t *testing.T) {
	if got := tracing.FromContext(nil); got != "" {
		t.Errorf("nil context should return \"\"; got %q", got)
	}
}

func TestWithID_FromContext_RoundTrip(t *testing.T) {
	id := "test-id-abc"
	ctx := tracing.WithID(context.Background(), id)
	if got := tracing.FromContext(ctx); got != id {
		t.Errorf("round-trip: want %q, got %q", id, got)
	}
}

func TestWithID_UsesLoggingKey(t *testing.T) {
	// Sanity check: pkg/tracing and pkg/logging must agree on the key
	// so logging's auto-injection sees what tracing puts in context.
	id := "shared-key-test"
	ctx := tracing.WithID(context.Background(), id)
	if got, _ := ctx.Value(logging.CorrelationIDKey{}).(string); got != id {
		t.Errorf("logging.CorrelationIDKey lookup: want %q, got %q", id, got)
	}
}

// --------------------------------------------------------------------
// Middleware — incoming request handling
// --------------------------------------------------------------------

func TestMiddleware_GeneratesIDWhenAbsent(t *testing.T) {
	var seen string
	h := tracing.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = tracing.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if seen == "" {
		t.Error("middleware should have generated a correlation ID")
	}
	if len(seen) != 32 {
		t.Errorf("generated ID should be 32 hex chars, got %d (%q)", len(seen), seen)
	}
	if got := rec.Header().Get(tracing.HeaderName); got != seen {
		t.Errorf("response header echo mismatch: want %q, got %q", seen, got)
	}
}

func TestMiddleware_AcceptsUpstreamID(t *testing.T) {
	var seen string
	h := tracing.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = tracing.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(tracing.HeaderName, "upstream-trace-id")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if seen != "upstream-trace-id" {
		t.Errorf("middleware should respect upstream X-Request-Id; got %q", seen)
	}
	if got := rec.Header().Get(tracing.HeaderName); got != "upstream-trace-id" {
		t.Errorf("response should echo upstream ID; got %q", got)
	}
}

func TestMiddleware_DoesNotOverwriteExistingContextID(t *testing.T) {
	// Edge case: if an inner middleware ran tracing.WithID before the
	// HTTP middleware (test scaffold scenarios), the request's header
	// still wins — that's the wire-truth. Document by test.
	h := tracing.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := tracing.FromContext(r.Context())
		if got != "from-header" {
			t.Errorf("header should win; got %q", got)
		}
	}))

	req := httptest.NewRequest("GET", "/", nil).WithContext(
		tracing.WithID(context.Background(), "from-prior-context"),
	)
	req.Header.Set(tracing.HeaderName, "from-header")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
}

// --------------------------------------------------------------------
// InjectHeader — outbound request handling
// --------------------------------------------------------------------

func TestInjectHeader_SetsHeaderWhenContextHasID(t *testing.T) {
	ctx := tracing.WithID(context.Background(), "out-1234")
	req, _ := http.NewRequest("GET", "http://example.com/", nil)

	tracing.InjectHeader(req, ctx)

	if got := req.Header.Get(tracing.HeaderName); got != "out-1234" {
		t.Errorf("header: want %q, got %q", "out-1234", got)
	}
}

func TestInjectHeader_NoOpWhenContextEmpty(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://example.com/", nil)
	tracing.InjectHeader(req, context.Background())

	if got := req.Header.Get(tracing.HeaderName); got != "" {
		t.Errorf("header should be unset; got %q", got)
	}
}

func TestInjectHeader_OverwritesExistingHeader(t *testing.T) {
	// Edge case: if the request was constructed with a stale ID, the
	// context's ID should win.
	ctx := tracing.WithID(context.Background(), "ctx-id")
	req, _ := http.NewRequest("GET", "http://example.com/", nil)
	req.Header.Set(tracing.HeaderName, "stale-id")

	tracing.InjectHeader(req, ctx)

	if got := req.Header.Get(tracing.HeaderName); got != "ctx-id" {
		t.Errorf("ctx ID should overwrite stale; got %q", got)
	}
}

// --------------------------------------------------------------------
// End-to-end: middleware + InjectHeader propagate ID
// --------------------------------------------------------------------

func TestEndToEnd_MiddlewareThenInjectHeader(t *testing.T) {
	// Service A: receives a request, gets an ID, then makes an
	// outbound call to Service B. Verify B's request carries A's ID.
	var serviceAID string

	serviceA := tracing.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serviceAID = tracing.FromContext(r.Context())

		// Now Service A makes an outbound call.
		outbound, _ := http.NewRequestWithContext(r.Context(), "GET", "http://service-b/", nil)
		tracing.InjectHeader(outbound, r.Context())

		// Inspect the outbound request directly (no actual network).
		if got := outbound.Header.Get(tracing.HeaderName); got != serviceAID {
			t.Errorf("outbound header should match service-A ID: want %q, got %q", serviceAID, got)
		}

		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	serviceA.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if serviceAID == "" {
		t.Fatal("service A never recorded its ID")
	}
	if !strings.EqualFold(rec.Header().Get(tracing.HeaderName), serviceAID) {
		t.Errorf("response echo: want %q, got %q", serviceAID, rec.Header().Get(tracing.HeaderName))
	}
}
