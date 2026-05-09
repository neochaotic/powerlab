// Package migrations is PowerLab's wrapper over pressly/goose for
// versioned schema migrations. Every service that owns persistent
// state ships an embed.FS of numbered .sql files and calls
// migrations.Up at startup in place of GORM's AutoMigrate.
//
// See ADR-0018 for the rationale + alternatives considered.
//
// Migration file format (goose convention):
//
//	-- +goose Up
//	CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL);
//
//	-- +goose Down
//	DROP TABLE users;
//
// Files are numbered: `0001_initial.sql`, `0002_add_email.sql`, etc.
// goose discovers them in sort order and applies pending ones in
// numeric sequence.
package migrations

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"strings"

	"github.com/pressly/goose/v3"
)

// Up applies every pending migration in the supplied filesystem to db.
// Already-applied migrations are skipped via goose's `goose_db_version`
// table. Calling Up on an up-to-date DB is a cheap no-op — every
// service can call it unconditionally at startup.
//
// The migrations argument is typically an embed.FS rooted at the
// service's `migrations/` directory; dir is the path within that
// FS where the .sql files live (use "" or "." if migrations is
// already rooted at the .sql files).
//
// On error, the DB is left at whatever migration version was last
// successfully applied. SQLite wraps DDL in implicit transactions,
// so individual migration failures don't leave half-applied state
// for the failing migration.
func Up(ctx context.Context, db *sql.DB, migrations fs.FS, dir string) error {
	provider, err := newProvider(db, migrations, dir)
	if err != nil {
		return err
	}
	_, err = provider.Up(ctx)
	return err
}

// Down rolls back the single most-recently-applied migration. If the
// DB is already at version 0, Down is a no-op.
//
// In practice Down is invoked from ops tooling, not from service
// startup — services always run Up. The capability exists for
// emergency rollback after a bad release.
func Down(ctx context.Context, db *sql.DB, migrations fs.FS, dir string) error {
	provider, err := newProvider(db, migrations, dir)
	if err != nil {
		return err
	}
	_, err = provider.Down(ctx)
	return err
}

// Version returns the current goose schema version. A fresh DB
// (one that has never been migrated) returns 0 with a nil error —
// callers can use this to distinguish "fresh install" from
// "upgrade in progress" cleanly.
//
// Returns a non-nil error only on connection failure or on a malformed
// `goose_db_version` table (which would indicate manual tampering).
//
// Implementation note: this queries the `goose_db_version` table
// directly so it does not require an fs.FS of migration files.
// Allows version reporting at points where the embedded migrations
// aren't readily available (health endpoints, ops scripts).
func Version(ctx context.Context, db *sql.DB) (int64, error) {
	const q = `SELECT MAX(version_id) FROM goose_db_version WHERE is_applied = 1`
	var v sql.NullInt64
	err := db.QueryRowContext(ctx, q).Scan(&v)
	if err != nil {
		// goose_db_version doesn't exist on a never-migrated DB.
		// Translate that into "version 0" — the canonical "fresh DB"
		// signal.
		if isNoSuchTable(err) {
			return 0, nil
		}
		return 0, err
	}
	if !v.Valid {
		// Table exists but no rows. Should not happen in practice
		// (goose inserts a row for migration 0 at first run) but
		// treat it the same as fresh.
		return 0, nil
	}
	return v.Int64, nil
}

// newProvider constructs a goose.Provider with sane defaults for
// PowerLab usage. If dir is non-empty and not ".", the fsys is
// fs.Sub'd into that subdirectory before being handed to goose.
func newProvider(db *sql.DB, fsys fs.FS, dir string) (*goose.Provider, error) {
	if dir != "" && dir != "." {
		sub, err := fs.Sub(fsys, dir)
		if err != nil {
			return nil, err
		}
		fsys = sub
	}
	return goose.NewProvider(goose.DialectSQLite3, db, fsys,
		goose.WithVerbose(false),
	)
}

// isNoSuchTable detects the SQLite "no such table: goose_db_version"
// error string. The modernc.org/sqlite driver doesn't expose typed
// errors for this — we fall back to substring matching, which is
// stable across modernc/mattn drivers.
func isNoSuchTable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) {
		// "no rows" can happen on a freshly-created table that goose
		// hasn't populated yet; treat as "fresh".
		return true
	}
	return strings.Contains(err.Error(), "no such table")
}
