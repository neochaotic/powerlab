package repository

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	pkgmigrations "github.com/neochaotic/powerlab/backend/pkg/migrations"
	"gorm.io/gorm"
)

// TestEmbeddedMigrations_Events guards the events-DB schema.
// Locks: every table the message-bus code queries must exist
// after Up; goose version reaches 1; running Up twice is a
// no-op. Mirrors user-service/pkg/sqlite/migrations_test.go.
func TestEmbeddedMigrations_Events(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm open: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := gdb.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	sqlDB, _ := gdb.DB()

	ctx := context.Background()
	if err := pkgmigrations.Up(ctx, sqlDB, migrationsEventsFS, "migrations_events"); err != nil {
		t.Fatalf("Up: %v", err)
	}
	for _, want := range []string{"event_types", "action_types", "property_types", "event_type_property_type", "action_type_property_type"} {
		var name string
		if err := sqlDB.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, want).Scan(&name); err != nil {
			t.Errorf("expected table %q after Up: %v", want, err)
		}
	}
	v, _ := pkgmigrations.Version(ctx, sqlDB)
	if v != 1 {
		t.Errorf("version: want 1, got %d", v)
	}

	if err := pkgmigrations.Up(ctx, sqlDB, migrationsEventsFS, "migrations_events"); err != nil {
		t.Fatalf("idempotent Up: %v", err)
	}
}

// TestEmbeddedMigrations_Persist guards the persist-DB schema:
// settings + ysk_cards. Same shape as the events test.
func TestEmbeddedMigrations_Persist(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm open: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := gdb.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	sqlDB, _ := gdb.DB()

	ctx := context.Background()
	if err := pkgmigrations.Up(ctx, sqlDB, migrationsPersistFS, "migrations_persist"); err != nil {
		t.Fatalf("Up: %v", err)
	}
	for _, want := range []string{"settings", "ysk_cards"} {
		var name string
		if err := sqlDB.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, want).Scan(&name); err != nil {
			t.Errorf("expected table %q after Up: %v", want, err)
		}
	}
	v, _ := pkgmigrations.Version(ctx, sqlDB)
	if v != 1 {
		t.Errorf("version: want 1, got %d", v)
	}

	if err := pkgmigrations.Up(ctx, sqlDB, migrationsPersistFS, "migrations_persist"); err != nil {
		t.Fatalf("idempotent Up: %v", err)
	}
}
