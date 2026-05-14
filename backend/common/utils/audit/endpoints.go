package audit

import (
	"context"
	"net/http"
	"os"
	"strconv"

	"github.com/labstack/echo/v4"
)

// ─── DB query layer (Recent / Stats) ────────────────────────────────────────

// RecentOptions configures the paginated query behind
// GET /v1/audit/recent. All fields optional — zero values mean
// "default" per ADR-0033.
type RecentOptions struct {
	// Limit caps the result count. Zero → default 100. Values
	// greater than 1000 are clamped to 1000 (DoS-defence; the
	// audit log can contain millions of rows on a busy host).
	Limit int

	// UserID filters to rows owned by this user. Nil = no filter.
	// Used by "show only my actions" in the Settings → Audit
	// pane.
	UserID *int64

	// SinceUnixMicros filters to rows with ts > SinceUnixMicros.
	// Zero = no filter. Used by the UI's "show only events since
	// I opened this view" cursor.
	SinceUnixMicros int64
}

// recentDefaults clamps/defaults the options to safe values.
func (o RecentOptions) clamp() RecentOptions {
	if o.Limit <= 0 {
		o.Limit = 100
	}
	if o.Limit > 1000 {
		o.Limit = 1000
	}
	return o
}

// Recent returns up to opts.Limit audit rows ordered newest-first,
// optionally filtered by UserID and SinceUnixMicros.
func (d *DB) Recent(ctx context.Context, opts RecentOptions) ([]Record, error) {
	opts = opts.clamp()

	// Build the query incrementally so optional filters compose
	// without sprawling SQL templates.
	q := `SELECT id, ts_unix_us, method, path, query, status, latency_us,
	             user_id, username, remote_ip, request_id
	      FROM audit
	      WHERE 1=1`
	args := []interface{}{}
	if opts.UserID != nil {
		q += ` AND user_id = ?`
		args = append(args, *opts.UserID)
	}
	if opts.SinceUnixMicros > 0 {
		q += ` AND ts_unix_us > ?`
		args = append(args, opts.SinceUnixMicros)
	}
	q += ` ORDER BY ts_unix_us DESC LIMIT ?`
	args = append(args, opts.Limit)

	rows, err := d.sql.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Record, 0, opts.Limit)
	for rows.Next() {
		r, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// StatsResult is the response body of /v1/audit/stats. Operator UI
// shows "47,329 records (12 MB) since 2026-04-13".
type StatsResult struct {
	RowCount         int64  `json:"row_count"`
	OldestUnixMicros int64  `json:"oldest_unix_us"`
	NewestUnixMicros int64  `json:"newest_unix_us"`
	FileSizeBytes    int64  `json:"file_size_bytes"`
	Path             string `json:"path"`
}

// Stats returns the audit-table summary. Empty table returns zero
// timestamps without erroring — operator just opened a fresh box.
func (d *DB) Stats(ctx context.Context) (StatsResult, error) {
	var s StatsResult
	s.Path = d.path

	// Count + oldest + newest in one round-trip. COALESCE so an
	// empty table returns 0/0 rather than NULL → scan error.
	row := d.sql.QueryRowContext(ctx, `
		SELECT COUNT(*),
		       COALESCE(MIN(ts_unix_us), 0),
		       COALESCE(MAX(ts_unix_us), 0)
		FROM audit
	`)
	if err := row.Scan(&s.RowCount, &s.OldestUnixMicros, &s.NewestUnixMicros); err != nil {
		return s, err
	}

	// File size — best-effort, doesn't fail the whole call if the
	// stat() raced with a checkpoint (rare).
	if info, err := os.Stat(d.path); err == nil {
		s.FileSizeBytes = info.Size()
	}

	return s, nil
}

// ─── HTTP handlers ──────────────────────────────────────────────────────────

// envelope mirrors the existing model.Result shape used by other
// PowerLab endpoints (data wrapper). Kept local here so we don't
// import model.Result and create a circular package edge.
type envelope[T any] struct {
	Data    T      `json:"data"`
	Message string `json:"message,omitempty"`
}

// RecentHandler binds db to an echo.HandlerFunc serving
// GET /v1/audit/recent. Reads limit / user_id / since query params
// and clamps them via RecentOptions.
func RecentHandler(db *DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		opts := RecentOptions{}

		if l := c.QueryParam("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil {
				opts.Limit = n
			}
		}
		if u := c.QueryParam("user_id"); u != "" {
			if uid, err := strconv.ParseInt(u, 10, 64); err == nil {
				opts.UserID = &uid
			}
		}
		if s := c.QueryParam("since"); s != "" {
			if ts, err := strconv.ParseInt(s, 10, 64); err == nil {
				opts.SinceUnixMicros = ts
			}
		}

		rows, err := db.Recent(c.Request().Context(), opts)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, envelope[[]Record]{
				Data:    nil,
				Message: err.Error(),
			})
		}
		return c.JSON(http.StatusOK, envelope[[]Record]{Data: rows})
	}
}

// StatsHandler binds db to an echo.HandlerFunc serving
// GET /v1/audit/stats.
func StatsHandler(db *DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		s, err := db.Stats(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, envelope[StatsResult]{
				Data:    StatsResult{},
				Message: err.Error(),
			})
		}
		return c.JSON(http.StatusOK, envelope[StatsResult]{Data: s})
	}
}
