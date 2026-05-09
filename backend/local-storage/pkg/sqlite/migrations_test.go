package sqlite

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	pkgmigrations "github.com/neochaotic/powerlab/backend/pkg/migrations"
	"gorm.io/gorm"
)

// freshGormDB opens an in-memory SQLite via the same glebarez driver
// the production code uses. Mirrors the test helpers in
// user-service / message-bus / core for uniform migration adoption.
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
// Phase 2.4. Smoke for local-storage's pkg/migrations adoption.
//
// The 3 tables (`o_disk` for Volume, `o_merge` for Merge,
// `o_merge_disk` for the many2many junction) are STICKY — they came
// verbatim from GORM AutoMigrate output and renaming any of them
// would break user installs that already have data under the old
// names. Volume's table name is `o_disk` (legacy CasaOS naming
// preserved via Volume.TableName() override).
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

	for _, want := range []string{"o_disk", "o_merge", "o_merge_disk"} {
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

// TestEmbeddedMigrations_Up_Idempotent — every service restart
// calls GetDBByFile → Up. Repeats must be no-ops.
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
