package audit

import (
	"net/url"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
)

// Middleware returns an echo MiddlewareFunc that records every
// request into the audit log via the given Recorder. Mount AFTER
// the JWT middleware so the user_id/username headers set by the
// JWT ParseTokenFunc are already on the request context.
//
// The middleware never blocks the response path. It captures
// entry time, runs the next handler, then assembles a Record and
// calls Recorder.Submit() — which is non-blocking and at most ~µs.
//
// The "token=" query parameter is stripped before storage (it's
// the EventSource auth fallback; never store credentials).
//
// Skipper, when provided, returns true to bypass recording for a
// specific request — useful for high-frequency health checks that
// would otherwise dominate the audit log.
func Middleware(rec *Recorder, opts MiddlewareOptions) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if opts.Skipper != nil && opts.Skipper(c) {
				return next(c)
			}

			start := time.Now()
			err := next(c)

			req := c.Request()
			res := c.Response()

			// Resolve user_id / username from headers the JWT
			// middleware sets. Absent = loopback / pre-auth.
			var (
				userIDPtr   *int64
				usernamePtr *string
			)
			if uidStr := req.Header.Get("user_id"); uidStr != "" {
				if uid, perr := strconv.ParseInt(uidStr, 10, 64); perr == nil {
					userIDPtr = &uid
				}
			}
			if uname := req.Header.Get("user_name"); uname != "" {
				usernamePtr = &uname
			}

			// RemoteIP — collapse loopback variants to the
			// canonical sentinel so the audit UI can filter
			// "admin tools / on-host" requests as one bucket.
			remote := c.RealIP()
			if remote == "::1" || remote == "127.0.0.1" {
				remote = LoopbackSentinel
			}

			r := Record{
				Method:        req.Method,
				Path:          req.URL.Path,
				Query:         stripTokenParam(req.URL.RawQuery),
				Status:        res.Status,
				LatencyMicros: time.Since(start).Microseconds(),
				UserID:        userIDPtr,
				Username:      usernamePtr,
				RemoteIP:      remote,
				RequestID:     req.Header.Get(echo.HeaderXRequestID),
			}
			r.FillTimestamps(start)
			rec.Submit(r)

			return err
		}
	}
}

// MiddlewareOptions configures the audit middleware. Currently
// only a per-request skipper; future options (sampling, path
// allow/deny, etc) extend this struct without breaking callers.
type MiddlewareOptions struct {
	// Skipper returns true to skip recording for a request.
	// Typical use: skip health checks under /v1/sys/heartbeat
	// or /v1/users/status that fire every 5s.
	Skipper func(c echo.Context) bool
}

// stripTokenParam removes the token=<jwt> query parameter from the
// raw query string. The EventSource fallback sends the JWT as a
// query param when it can't send a header — we must never persist
// that credential in the audit log.
//
// Returns the original query unchanged if no token= is present, so
// the common-case fast path is allocation-free.
func stripTokenParam(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		// Malformed query — return as-is. The audit table doesn't
		// enforce query format, and a parse error already means
		// the request itself was suspect.
		return rawQuery
	}
	if _, has := values["token"]; !has {
		return rawQuery
	}
	values.Del("token")
	return values.Encode()
}
