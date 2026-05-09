package tracing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/neochaotic/powerlab/backend/pkg/logging"
)

// HeaderName is the HTTP header carrying the correlation ID across
// service boundaries. X-Request-Id is the de-facto standard used by
// HAProxy, Traefik, nginx, Heroku, and most Go middleware libraries.
const HeaderName = "X-Request-Id"

// NewID returns a fresh correlation ID — 32 hex characters from 16
// random bytes. Collision-resistant (2^128 space), URL/header-safe,
// no external dependency.
//
// crypto/rand failure is exceptional (would indicate the OS RNG is
// broken) and would deserve a panic, but a panic at the request
// entry point is exactly the situation pkg/lifecycle was designed to
// recover from. We surface the failure in-band: NewID always returns
// a non-empty string. On the (extremely rare) RNG failure we fall
// back to a fixed sentinel so callers still produce a parseable ID.
// The sentinel is unmistakable in logs and grep alerts.
func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand.Read should never fail on healthy systems.
		// Returning a sentinel rather than panicking keeps request
		// entry deterministic.
		return "00000000000000000000000000000000"
	}
	return hex.EncodeToString(b[:])
}

// FromContext returns the correlation ID stored in ctx, or empty
// string if none. Safe to call with a nil context (returns "").
func FromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(logging.CorrelationIDKey{}).(string); ok {
		return id
	}
	return ""
}

// WithID returns a derived context carrying the correlation ID.
//
// Uses logging.CorrelationIDKey{} so pkg/logging's auto-injection sees
// the same value pkg/tracing wrote.
func WithID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, logging.CorrelationIDKey{}, id)
}

// Middleware reads the correlation ID from the inbound request's
// X-Request-Id header (or generates one if absent), stores it in the
// request's context, and echoes it back on the response so the client
// can quote it from a toast or error UI.
//
// Wrap as the outermost layer of the chain — even panic-recovery
// (lifecycle.RecoverMiddleware) benefits from having the ID available
// in the request context when it logs the stack trace.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(HeaderName)
		if id == "" {
			id = NewID()
		}

		// Echo back so the response carries the same ID. UI clients can
		// surface it in toasts; bug reporters can quote it.
		w.Header().Set(HeaderName, id)

		ctx := WithID(r.Context(), id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// InjectHeader sets the X-Request-Id header on req using the
// correlation ID from ctx. No-op when ctx has no correlation ID.
//
// Call before client.Do(req) on every outbound HTTP call to a
// PowerLab service. This is what carries the correlation forward
// across service boundaries.
func InjectHeader(req *http.Request, ctx context.Context) {
	if id := FromContext(ctx); id != "" {
		req.Header.Set(HeaderName, id)
	}
}
