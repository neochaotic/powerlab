package audit

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	// Pure-Go SQLite driver per ADR-0033. No CGO; the audit package
	// must compile cleanly on the same CGO-free build settings the
	// rest of the backend uses (CGO_ENABLED=0 from package-linux.sh).
	_ "modernc.org/sqlite"
)

// LoopbackSentinel is the canonical RemoteIP value the middleware
// writes for requests originating from 127.0.0.1 or ::1. Exposed as
// a constant so middleware authors and the read-side filter can
// reference the same literal (ADR-0033 PII section).
const LoopbackSentinel = "loopback"

// DB wraps a *sql.DB and exposes only the audit-table operations.
// Callers use this instead of database/sql directly so the schema
// stays a black box.
type DB struct {
	sql *sql.DB
}

// OpenDB opens (or creates) the SQLite audit database at path. Runs
// the schema migration on first open and again on every open — the
// migration uses CREATE TABLE IF NOT EXISTS so it's idempotent.
//
// WAL journal mode is enabled so the recorder's writer goroutine
// can flush while read-side handlers (recent/stats) query in
// parallel without lock contention.
func OpenDB(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return nil, fmt.Errorf("audit: open %s: %w", path, err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("audit: ping %s: %w", path, err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("audit: migrate %s: %w", path, err)
	}
	return &DB{sql: db}, nil
}

// Close releases the underlying *sql.DB. Safe to call multiple
// times; subsequent calls are no-ops once the handle is closed.
func (d *DB) Close() error {
	if d.sql == nil {
		return nil
	}
	err := d.sql.Close()
	d.sql = nil
	return err
}

// Insert writes a single record. The recorder normally batches
// many of these per transaction (see InsertBatch); the single-row
// form is here for tests and for callers that want a synchronous
// audit entry (rare; the async path via Recorder is the contract).
func (d *DB) Insert(ctx context.Context, r Record) error {
	_, err := d.sql.ExecContext(ctx, insertSQL,
		r.TsUnixMicros, r.Method, r.Path, r.Query, r.Status, r.LatencyMicros,
		nullableInt64(r.UserID), nullableString(r.Username),
		r.RemoteIP, r.RequestID,
	)
	return err
}

// InsertBatch writes many records in a single transaction. Used by
// the recorder's writer goroutine to amortise commit cost — one
// transaction per BatchSize records (default 50) rather than one
// per record.
func (d *DB) InsertBatch(ctx context.Context, records []Record) error {
	if len(records) == 0 {
		return nil
	}
	tx, err := d.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, insertSQL)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	for _, r := range records {
		if _, err := stmt.ExecContext(ctx,
			r.TsUnixMicros, r.Method, r.Path, r.Query, r.Status, r.LatencyMicros,
			nullableInt64(r.UserID), nullableString(r.Username),
			r.RemoteIP, r.RequestID,
		); err != nil {
			_ = stmt.Close()
			_ = tx.Rollback()
			return err
		}
	}
	if err := stmt.Close(); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// GetMostRecent returns the newest record by ts_unix_us. Returns
// sql.ErrNoRows when the table is empty — callers check via
// errors.Is. Used by middleware tests + the /v1/audit/recent
// pagination cursor.
func (d *DB) GetMostRecent(ctx context.Context) (Record, error) {
	row := d.sql.QueryRowContext(ctx, `
		SELECT id, ts_unix_us, method, path, query, status, latency_us,
		       user_id, username, remote_ip, request_id
		FROM audit
		ORDER BY ts_unix_us DESC
		LIMIT 1
	`)
	return scanRecord(row)
}

// rowScanner is the minimal interface satisfied by both *sql.Row
// and *sql.Rows for the shared scan helper.
type rowScanner interface {
	Scan(dest ...interface{}) error
}

// scanRecord materialises a Record from a row scanner, converting
// the NullInt64/NullString into the public-type pointers.
func scanRecord(row rowScanner) (Record, error) {
	var (
		r        Record
		uid      sql.NullInt64
		username sql.NullString
	)
	err := row.Scan(&r.ID, &r.TsUnixMicros, &r.Method, &r.Path, &r.Query,
		&r.Status, &r.LatencyMicros, &uid, &username,
		&r.RemoteIP, &r.RequestID)
	if err != nil {
		return Record{}, err
	}
	if uid.Valid {
		v := uid.Int64
		r.UserID = &v
	}
	if username.Valid {
		v := username.String
		r.Username = &v
	}
	return r, nil
}

// Count returns the total number of audit rows. O(1) on SQLite
// when no WHERE clause is present.
func (d *DB) Count(ctx context.Context) (int64, error) {
	var n int64
	err := d.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit`).Scan(&n)
	return n, err
}

// PruneByAge removes rows with TsUnixMicros older than cutoff.
// Returns the number of rows removed. Called by the recorder's
// hourly retention goroutine for the age-based limit.
func (d *DB) PruneByAge(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := d.sql.ExecContext(ctx, `DELETE FROM audit WHERE ts_unix_us < ?`, cutoff.UnixMicro())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// PruneToMaxRows removes the oldest rows until the table is at or
// below maxRows. Returns the number of rows removed. Used as the
// size-based limit's coarse cousin; the byte-cap goroutine measures
// file size and decides how many rows to ask for here.
func (d *DB) PruneToMaxRows(ctx context.Context, maxRows int64) (int64, error) {
	// Two-step: count current, decide how many to drop, then drop
	// the oldest. Doing it in a single DELETE ... ORDER BY LIMIT
	// is also possible but SQLite needs an extra subquery for that
	// shape; the two-step is clearer and the row count is cheap.
	var current int64
	if err := d.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit`).Scan(&current); err != nil {
		return 0, err
	}
	if current <= maxRows {
		return 0, nil
	}
	toDrop := current - maxRows
	res, err := d.sql.ExecContext(ctx,
		`DELETE FROM audit WHERE id IN (SELECT id FROM audit ORDER BY ts_unix_us ASC LIMIT ?)`,
		toDrop,
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// nullableInt64 converts an optional *int64 (UserID) into the
// sql.NullInt64 the driver wants for nullable columns.
func nullableInt64(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}

// nullableString same idea for Username.
func nullableString(v *string) sql.NullString {
	if v == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *v, Valid: true}
}

// schemaSQL is the table + indexes per ADR-0033. CREATE ... IF NOT
// EXISTS so re-running on existing DBs is a no-op.
const schemaSQL = `
CREATE TABLE IF NOT EXISTS audit (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  ts_unix_us  INTEGER NOT NULL,
  method      TEXT    NOT NULL,
  path        TEXT    NOT NULL,
  query       TEXT    NOT NULL DEFAULT '',
  status      INTEGER NOT NULL,
  latency_us  INTEGER NOT NULL,
  user_id     INTEGER NULL,
  username    TEXT    NULL,
  remote_ip   TEXT    NOT NULL,
  request_id  TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS audit_ts ON audit(ts_unix_us DESC);
CREATE INDEX IF NOT EXISTS audit_user ON audit(user_id, ts_unix_us DESC);
`

const insertSQL = `
INSERT INTO audit (ts_unix_us, method, path, query, status, latency_us, user_id, username, remote_ip, request_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
