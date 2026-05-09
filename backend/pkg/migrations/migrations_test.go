package migrations_test

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/neochaotic/powerlab/backend/pkg/migrations"

	_ "modernc.org/sqlite"
)

// freshDB opens an in-memory SQLite handle for the duration of one test.
// Cleanup is automatic when t finishes.
func freshDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open(:memory:): %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// fakeMigrations builds an in-memory fs.FS holding numbered .sql files
// using the goose +goose Up / +goose Down format.
func fakeMigrations(files map[string]string) fs.FS {
	out := fstest.MapFS{}
	for name, body := range files {
		out[name] = &fstest.MapFile{Data: []byte(body)}
	}
	return out
}

// TestUp_FreshDB_AppliesAllMigrations is the happy path: an empty DB
// + N migration files yields a populated DB at version N.
func TestUp_FreshDB_AppliesAllMigrations(t *testing.T) {
	db := freshDB(t)
	mfs := fakeMigrations(map[string]string{
		"0001_users.sql": `-- +goose Up
CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL);
-- +goose Down
DROP TABLE users;
`,
		"0002_apps.sql": `-- +goose Up
CREATE TABLE apps (id INTEGER PRIMARY KEY, name TEXT NOT NULL);
-- +goose Down
DROP TABLE apps;
`,
	})

	if err := migrations.Up(context.Background(), db, mfs, "."); err != nil {
		t.Fatalf("Up returned error: %v", err)
	}

	v, err := migrations.Version(context.Background(), db)
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if v != 2 {
		t.Errorf("version: want 2, got %d", v)
	}

	// Both tables must exist.
	for _, name := range []string{"users", "apps"} {
		var got string
		err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&got)
		if err != nil {
			t.Errorf("table %q missing: %v", name, err)
		}
	}
}

// TestUp_Idempotent verifies running Up twice is a no-op the second
// time. This is the property that makes startup safe — every boot
// calls Up; only the first one does work.
func TestUp_Idempotent(t *testing.T) {
	db := freshDB(t)
	mfs := fakeMigrations(map[string]string{
		"0001_init.sql": `-- +goose Up
CREATE TABLE t (id INTEGER PRIMARY KEY);
-- +goose Down
DROP TABLE t;
`,
	})
	ctx := context.Background()

	if err := migrations.Up(ctx, db, mfs, "."); err != nil {
		t.Fatalf("first Up: %v", err)
	}
	if err := migrations.Up(ctx, db, mfs, "."); err != nil {
		t.Fatalf("second Up (idempotent): %v", err)
	}

	v, _ := migrations.Version(ctx, db)
	if v != 1 {
		t.Errorf("version after double Up: want 1, got %d", v)
	}
}

// TestUp_PartialDB_AppliesPendingOnly verifies that Up applied to a DB
// that already has migration N skips re-applying it and only runs the
// pending ones.
func TestUp_PartialDB_AppliesPendingOnly(t *testing.T) {
	db := freshDB(t)
	ctx := context.Background()

	step1 := fakeMigrations(map[string]string{
		"0001_a.sql": `-- +goose Up
CREATE TABLE a (id INTEGER PRIMARY KEY);
-- +goose Down
DROP TABLE a;
`,
	})
	if err := migrations.Up(ctx, db, step1, "."); err != nil {
		t.Fatalf("step 1: %v", err)
	}

	step2 := fakeMigrations(map[string]string{
		"0001_a.sql": `-- +goose Up
CREATE TABLE a (id INTEGER PRIMARY KEY);
-- +goose Down
DROP TABLE a;
`,
		"0002_b.sql": `-- +goose Up
CREATE TABLE b (id INTEGER PRIMARY KEY);
-- +goose Down
DROP TABLE b;
`,
	})
	if err := migrations.Up(ctx, db, step2, "."); err != nil {
		t.Fatalf("step 2: %v", err)
	}

	v, _ := migrations.Version(ctx, db)
	if v != 2 {
		t.Errorf("version: want 2, got %d", v)
	}
}

// TestUp_MalformedSQL_ReturnsError verifies that a syntactically broken
// migration is reported as an error, not silently skipped. Malformed
// migrations in production must fail loudly.
func TestUp_MalformedSQL_ReturnsError(t *testing.T) {
	db := freshDB(t)
	mfs := fakeMigrations(map[string]string{
		"0001_broken.sql": `-- +goose Up
CREATE TABEL broken_typo (id INTEGER);
-- +goose Down
DROP TABLE broken_typo;
`,
	})

	err := migrations.Up(context.Background(), db, mfs, ".")
	if err == nil {
		t.Errorf("Up with malformed SQL: want error, got nil")
	}
}

// TestVersion_FreshDB_ReturnsZero verifies that an unmigrated DB
// reports version 0, not an error and not panic. This is what the
// startup health check needs in order to print "fresh install" vs.
// "upgrade in progress" messages cleanly.
func TestVersion_FreshDB_ReturnsZero(t *testing.T) {
	db := freshDB(t)

	v, err := migrations.Version(context.Background(), db)
	if err != nil {
		t.Fatalf("Version on fresh DB: %v", err)
	}
	if v != 0 {
		t.Errorf("fresh DB version: want 0, got %d", v)
	}
}

// TestDown_RollsBackMostRecent verifies that Down undoes the most
// recently applied migration but leaves earlier ones in place.
func TestDown_RollsBackMostRecent(t *testing.T) {
	db := freshDB(t)
	mfs := fakeMigrations(map[string]string{
		"0001_keep.sql": `-- +goose Up
CREATE TABLE keep (id INTEGER PRIMARY KEY);
-- +goose Down
DROP TABLE keep;
`,
		"0002_drop.sql": `-- +goose Up
CREATE TABLE drop_me (id INTEGER PRIMARY KEY);
-- +goose Down
DROP TABLE drop_me;
`,
	})
	ctx := context.Background()

	if err := migrations.Up(ctx, db, mfs, "."); err != nil {
		t.Fatalf("Up: %v", err)
	}
	if err := migrations.Down(ctx, db, mfs, "."); err != nil {
		t.Fatalf("Down: %v", err)
	}

	v, _ := migrations.Version(ctx, db)
	if v != 1 {
		t.Errorf("version after Down: want 1, got %d", v)
	}

	// drop_me should be gone, keep should remain.
	var name string
	if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='keep'`).Scan(&name); err != nil {
		t.Errorf("'keep' table should still exist: %v", err)
	}
	row := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='drop_me'`)
	if err := row.Scan(&name); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("'drop_me' table should be gone, got: scan err=%v name=%q", err, name)
	}
}
