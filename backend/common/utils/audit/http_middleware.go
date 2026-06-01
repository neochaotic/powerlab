package audit

import (
	"bufio"
	"net"
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
			// Per-request enrichment hook. Lets a host service surface
			// protocol-specific facts the generic HTTP middleware can't
			// see (e.g. powerlab-mcp parses the JSON-RPC envelope and
			// fills Kind=mcp.tool_call + Payload{tool_name}). The hook
			// runs AFTER ServeHTTP so the enricher can inspect both
			// the request (typically via context set by an upstream
			// body-tee) and the captured status. nil → no-op.
			if opts.Enricher != nil {
				opts.Enricher(r, &rec0)
			}
			rec.Submit(rec0)
		})
	}
}

// HTTPMiddlewareOptions configures the stdlib audit middleware.
type HTTPMiddlewareOptions struct {
	// Skipper returns true to skip recording for a request.
	// Useful for high-frequency static asset paths or health checks.
	Skipper func(r *http.Request) bool

	// Enricher is an optional per-request hook that lets the host
	// service mutate the captured Record before it's submitted —
	// adding Kind, Payload, or any other protocol-specific fields the
	// generic HTTP middleware doesn't know about. Runs AFTER
	// next.ServeHTTP so the request context can carry parsed-by-the-
	// inner-handler facts. nil → no-op. Used by powerlab-mcp to
	// surface tool_name / uri / kind into the audit record without
	// forking the middleware (ADR-0047 + issue #644).
	Enricher func(r *http.Request, rec *Record)
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

// Flush forwards to the underlying ResponseWriter. Required for
// SSE / chunked-transfer downstream handlers — without this, the
// type assertion `w.(http.Flusher)` in handlers and reverse proxies
// fails and writes silently buffer until the stream closes. That
// makes a streaming install-log endpoint appear hung.
func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack forwards to the underlying ResponseWriter when it supports
// it. Required for WebSocket upgrades and any handler that takes
// over the raw TCP connection.
func (s *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := s.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
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
