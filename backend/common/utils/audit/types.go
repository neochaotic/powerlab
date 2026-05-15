// Package audit records per-request audit events (method, path, status,
// latency, user, IP) into a JSONL file rotated by lumberjack, with an
// in-memory ring buffer serving the UI hot path. Designed for the
// operator answer to "who did what when" inside the PowerLab panel.
//
// Storage policy, retention contract, performance budget, and PII
// stance are locked in ADR-0033 (middleware shape) and ADR-0035
// (JSONL storage, supersedes the SQLite section of ADR-0033).
//
// This file declares the wire types. The recorder runtime lives in
// recorder.go; the ring buffer + file reader in store.go.
package audit

import "time"

// LoopbackSentinel is the canonical RemoteIP value the middleware
// writes for requests originating from 127.0.0.1 or ::1. Exposed as
// a constant so middleware authors and the read-side filter can
// align on the same magic value.
const LoopbackSentinel = "loopback"

// Record is a single audit event. Persisted as one JSON line in
// /var/log/powerlab/audit.jsonl and held in the in-memory ring
// buffer that serves /v1/audit/recent.
//
// Fields that may be absent on a given record use pointer types so
// the JSON encoder emits explicit null — `UserID == nil` means the
// request was a loopback admin call where JWT auth was skipped, not
// "user 0".
//
// JSON tags use the operator-readable form: snake_case fields, plus
// an RFC 3339 timestamp string ("ts") for human grep ergonomics
// alongside the int64 microseconds ("ts_us") for sortable scanning.
type Record struct {
	// Ts is the request timestamp formatted as RFC 3339 with
	// microsecond precision. Set automatically by the middleware
	// when the request is captured. Human-greppable.
	Ts string `json:"ts"`

	// TsUnixMicros is the request timestamp in microseconds since
	// the Unix epoch. Same instant as Ts, but in sortable integer
	// form for scan/filter performance.
	TsUnixMicros int64 `json:"ts_us"`

	// Method is the HTTP verb (GET, POST, PUT, DELETE, PATCH).
	Method string `json:"method"`

	// Path is the request URL path, without the query string.
	// Example: "/v2/app_management/compose".
	Path string `json:"path"`

	// Query is the URL query string with the token= parameter
	// stripped (the EventSource auth fallback must never appear
	// in audit records). Empty when the request had no query.
	Query string `json:"query,omitempty"`

	// Status is the HTTP response status code (200, 401, 500, ...).
	Status int `json:"status"`

	// LatencyMicros is the elapsed time between request entry and
	// response completion, in microseconds.
	LatencyMicros int64 `json:"latency_us"`

	// UserID is the authenticated user id from JWT claims. null
	// when the request was loopback-skipped (admin tools) or
	// otherwise unauthenticated.
	UserID *int64 `json:"user_id"`

	// Username is denormalised from JWT claims so the UI can show
	// a human name without joining against the user-service. null
	// when UserID is null.
	Username *string `json:"username"`

	// RemoteIP is the client's IP per Echo's c.RealIP(). Stored as
	// the literal "loopback" sentinel when the request came from
	// 127.0.0.1 or ::1 (per ADR-0033 PII section).
	RemoteIP string `json:"remote_ip"`

	// RequestID is the X-Request-Id header passed through if the
	// caller set one; empty otherwise. Useful for correlating with
	// systemd journal entries that carry the same id via slog.
	RequestID string `json:"request_id,omitempty"`

	// Kind is the record-type discriminator. Empty string (the JSON
	// `omitempty` keeps it out of legacy records) means an HTTP
	// request audit captured by the middleware — the original record
	// type and the overwhelming majority of records. Non-empty values
	// signal records produced outside the middleware path:
	//
	//   - "ui_error": a frontend window.onerror / unhandledrejection
	//     captured by the SvelteKit shell and posted to
	//     /v1/audit/frontend-error. Payload carries message + stack +
	//     url + ua + viewport.
	//
	// Storage stays a single JSONL file with a single ring buffer;
	// readers switch on Kind to render differently.
	Kind string `json:"kind,omitempty"`

	// Payload holds Kind-specific fields. Empty for HTTP requests
	// (`omitempty` keeps it out of the wire). For "ui_error" the
	// shape is {message, stack, url, ua, viewport: {w, h}} — see
	// ADR-0033 for the contract.
	Payload map[string]any `json:"payload,omitempty"`
}

// Time returns the record's timestamp as a time.Time in UTC.
// Convenience over manual microsecond conversion.
func (r Record) Time() time.Time {
	return time.UnixMicro(r.TsUnixMicros).UTC()
}

// Latency returns the recorded latency as a time.Duration.
// Convenience over manual microsecond conversion.
func (r Record) Latency() time.Duration {
	return time.Duration(r.LatencyMicros) * time.Microsecond
}

// FillTimestamps sets both Ts and TsUnixMicros from the given
// time.Time. The middleware calls this once per request so the two
// representations stay in sync.
func (r *Record) FillTimestamps(t time.Time) {
	r.TsUnixMicros = t.UnixMicro()
	r.Ts = t.UTC().Format("2006-01-02T15:04:05.000000Z07:00")
}
