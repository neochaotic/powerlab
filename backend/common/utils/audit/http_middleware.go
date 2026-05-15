package audit

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// HTTPMiddleware wraps an http.Handler with audit recording. Mirror
// of the Echo Middleware for the gateway's public stdlib mux per
// ADR-0035. Captures the same fields (method, path, status, latency,
// user_id/username from JWT headers, remote IP, request_id).
//
// JWT decoding upstream must set "user_id" / "user_name" request
// headers (same as the Echo path). When absent the audit row carries
// null user fields — exactly what the Echo middleware does on
// loopback / pre-auth requests.
//
// Non-blocking on the hot path: status capture uses a thin response
// wrapper; the recorder Submit is at most µs and channel-full drops
// rather than blocking.
func HTTPMiddleware(rec *Recorder, opts HTTPMiddlewareOptions) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if opts.Skipper != nil && opts.Skipper(r) {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()
			sw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)

			var userIDPtr *int64
			var usernamePtr *string
			if uidStr := r.Header.Get("user_id"); uidStr != "" {
				if uid, err := strconv.ParseInt(uidStr, 10, 64); err == nil {
					userIDPtr = &uid
				}
			}
			if uname := r.Header.Get("user_name"); uname != "" {
				usernamePtr = &uname
			}

			remote := realIP(r)
			if remote == "::1" || remote == "127.0.0.1" {
				remote = LoopbackSentinel
			}

			rec0 := Record{
				Method:        r.Method,
				Path:          r.URL.Path,
				Query:         stripTokenParam(r.URL.RawQuery),
				Status:        sw.status,
				LatencyMicros: time.Since(start).Microseconds(),
				UserID:        userIDPtr,
				Username:      usernamePtr,
				RemoteIP:      remote,
				RequestID:     r.Header.Get("X-Request-Id"),
			}
			rec0.FillTimestamps(start)
			rec.Submit(rec0)
		})
	}
}

// HTTPMiddlewareOptions configures the stdlib audit middleware.
type HTTPMiddlewareOptions struct {
	// Skipper returns true to skip recording for a request.
	// Useful for high-frequency static asset paths or health checks.
	Skipper func(r *http.Request) bool
}

// statusRecorder snoops the status code written upstream. Wraps
// http.ResponseWriter so WriteHeader sets `status` before delegating.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wroteHeader {
		s.status = code
		s.wroteHeader = true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.wroteHeader {
		// Implicit 200 — Go's http server does this on first Write
		// without WriteHeader. Record matches Go's behaviour.
		s.status = http.StatusOK
		s.wroteHeader = true
	}
	return s.ResponseWriter.Write(b)
}

// realIP returns the client's IP. Prefers X-Forwarded-For (first hop)
// when present, falling back to RemoteAddr (host portion).
func realIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first hop; downstream proxies append, leftmost is
		// the original client.
		if i := strings.IndexByte(xff, ','); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	addr := r.RemoteAddr
	if i := strings.LastIndexByte(addr, ':'); i > 0 {
		return addr[:i]
	}
	return addr
}
