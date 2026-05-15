// Package audit records per-request audit events (method, path, status,
// latency, user, IP) into a per-service SQLite database. Designed for
// the operator answer to "who did what when" inside the PowerLab panel.
// Schema, retention policy, performance contract, and PII stance are
// locked in ADR-0033.
//
// This file declares the wire types. The recorder runtime lives in
// recorder.go; database open + schema migration in db.go.
package audit

import "time"

// Record is a single audit row. ID is assigned by SQLite on insert
// and zero on records that have not yet been flushed. Time fields
// live as int64 microseconds-since-epoch internally for sortable +
// compact storage; the Time() helper rehydrates a time.Time on read.
//
// Fields that may be NULL in the database use pointer types so the
// zero value is distinguishable from "unset" — UserID == nil means
// the request was a loopback admin call where JWT auth was skipped,
// not "user 0".
type Record struct {
	// ID is the SQLite-assigned primary key. Zero before insert.
	ID int64

	// TsUnixMicros is the request timestamp in microseconds since
	// the Unix epoch. Set automatically by the middleware when the
	// request is captured; clients should not set this manually.
	TsUnixMicros int64

	// Method is the HTTP verb (GET, POST, PUT, DELETE, PATCH).
	Method string

	// Path is the request URL path, without the query string.
	// Example: "/v2/app_management/compose".
	Path string

	// Query is the URL query string with the token= parameter
	// stripped (the EventSource auth fallback must never appear
	// in audit records). Empty when the request had no query.
	Query string

	// Status is the HTTP response status code (200, 401, 500, ...).
	Status int

	// LatencyMicros is the elapsed time between request entry and
	// response completion, in microseconds.
	LatencyMicros int64

	// UserID is the authenticated user id from JWT claims. NULL
	// when the request was loopback-skipped (admin tools) or
	// otherwise unauthenticated.
	UserID *int64

	// Username is denormalised from JWT claims so the UI can show
	// a human name without joining against the user-service. NULL
	// when UserID is NULL.
	Username *string

	// RemoteIP is the client's IP per Echo's c.RealIP(). Stored as
	// the literal "loopback" sentinel when the request came from
	// 127.0.0.1 or ::1 (per ADR-0033 PII section).
	RemoteIP string

	// RequestID is the X-Request-Id header passed through if the
	// caller set one; empty otherwise. Useful for correlating with
	// systemd journal entries that carry the same id via slog.
	RequestID string
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
