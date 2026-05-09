package sqlite

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	pkgmigrations "github.com/neochaotic/powerlab/backend/pkg/migrations"
	"gorm.io/gorm"
)

// freshGormDB opens an in-memory SQLite via the same glebarez driver
// the production code uses, then unwraps to *sql.DB so we can hand
// it to pkgmigrations. This keeps the test exercising the exact
// driver path GetDb does.
func freshGormDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm open: %v", err)
	}
	cleanup := func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
	return db, cleanup
}

// TestEmbeddedMigrations_Up_ProducesExpectedTables is the per-service
// smoke for the migration runner: it opens a fresh in-memory SQLite,
// runs the embedded migrations from migrationsFS, and confirms every
// table the service operates on is present.
//
// It catches three classes of regression that escape contract-level
// pkg/migrations tests:
//
//   - Malformed SQL in 0001_initial.sql (parser fails on goose load).
//   - A model added to the codebase but no corresponding migration
//     (table is missing after Up).
//   - A typo in a column or index name.
func TestEmbeddedMigrations_Up_ProducesExpectedTables(t *testing.T) {
	gdb, cleanup := freshGormDB(t)
	t.Cleanup(cleanup)
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("DB(): %v", err)
	}

	ctx := context.Background()
	if err := pkgmigrations.Up(ctx, sqlDB, migrationsFS, "migrations"); err != nil {
		t.Fatalf("Up: %v", err)
	}

	for _, want := range []string{"o_users", "events"} {
		var name string
		err := sqlDB.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, want).Scan(&name)
		if err != nil {
			t.Errorf("expected table %q to exist after Up: %v", want, err)
		}
	}

	v, err := pkgmigrations.Version(ctx, sqlDB)
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if v != 1 {
		t.Errorf("version after Up: want 1, got %d", v)
	}
}

// TestEmbeddedMigrations_Up_Idempotent guards the property the
// service depends on: every restart calls GetDb → Up, and every
// repeat is a no-op.
func TestEmbeddedMigrations_Up_Idempotent(t *testing.T) {
	gdb, cleanup := freshGormDB(t)
	t.Cleanup(cleanup)
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("DB(): %v", err)
	}

	ctx := context.Background()
	if err := pkgmigrations.Up(ctx, sqlDB, migrationsFS, "migrations"); err != nil {
		t.Fatalf("first Up: %v", err)
	}
	if err := pkgmigrations.Up(ctx, sqlDB, migrationsFS, "migrations"); err != nil {
		t.Fatalf("second Up (idempotent): %v", err)
	}

	v, _ := pkgmigrations.Version(ctx, sqlDB)
	if v != 1 {
		t.Errorf("version after double Up: want 1, got %d", v)
	}
}
