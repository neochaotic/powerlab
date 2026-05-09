package sqlite

import (
	"context"
	"time"

	"github.com/neochaotic/powerlab/backend/core/pkg/utils/file"
	"github.com/glebarez/sqlite"
	pkgmigrations "github.com/neochaotic/powerlab/backend/pkg/migrations"
	"gorm.io/gorm"
)

var gdb *gorm.DB

// GetDb returns the core SQLite database, lazily opening it from
// dbPath/casaOS.db on the first call. Subsequent calls reuse the
// same handle.
//
// SetMaxOpenConns(1) because SQLite serializes writes process-wide
// and multiple writers race-deadlock on file locks. Up to 10 idle
// connections are kept warm; idle conns expire after ~16 minutes.
//
// Schema migration runs eagerly via pkg/migrations on first call
// (Sprint 3 Phase 2.3). The migration is also responsible for
// dropping the legacy CasaOS tables (`o_application`, `o_friend`,
// `o_person_download`, `o_person_down_record`) on upgrade —
// previously those drops happened in Go code right after
// AutoMigrate; now they're co-located with the schema definition
// inside `migrations/0001_initial.sql`.
//
// A migration failure is logged but does NOT panic — the caller
// proceeds with best-effort reads against whatever schema is on
// disk. (Same behavior as user-service. Versioned migrations
// per #100; ADR-0018.)
func GetDb(dbPath string) *gorm.DB {
	if gdb != nil {
		return gdb
	}
	file.IsNotExistMkDir(dbPath)
	db, err := gorm.Open(sqlite.Open(dbPath+"/casaOS.db"), &gorm.Config{})
	if err != nil {
		panic("sqlite connect error")
	}

	c, _ := db.DB()
	c.SetMaxIdleConns(10)
	c.SetMaxOpenConns(1)
	c.SetConnMaxIdleTime(time.Second * 1000)
	gdb = db

	if sqlDB, dbErr := db.DB(); dbErr == nil {
		_ = pkgmigrations.Up(context.Background(), sqlDB, migrationsFS, "migrations")
	}
	return db
}
