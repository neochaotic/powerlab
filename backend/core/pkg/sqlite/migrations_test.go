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
// it to pkgmigrations. Mirrors user-service's test helper to keep
// the migration adoption pattern uniform across services.
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

// TestEmbeddedMigrations_Up_ProducesExpectedTables — Sprint 3
// Phase 2.3. The smoke for core's pkg/migrations adoption: opens a
// fresh in-memory SQLite, runs the embedded migrations, asserts the
// 4 tables core operates on exist (`o_notify`, `o_shares`,
// `o_connections`, `peer_drive_db_models`).
//
// These names are sticky — they came verbatim from GORM's
// AutoMigrate output (captured by a one-shot dump-schema helper at
// migration creation time). Renaming any of them would break
// existing user installs that already have data under the old name.
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

	for _, want := range []string{"o_notify", "o_shares", "o_connections", "peer_drive_db_models"} {
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

// TestEmbeddedMigrations_Up_Idempotent — service restarts call
// GetDb → Up. Every Up after the first must be a no-op.
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
