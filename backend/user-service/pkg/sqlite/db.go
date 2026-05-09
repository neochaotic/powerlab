package sqlite

import (
	"context"
	"time"

	"github.com/neochaotic/powerlab/backend/user-service/pkg/utils/file"
	"github.com/glebarez/sqlite"
	pkgmigrations "github.com/neochaotic/powerlab/backend/pkg/migrations"
	"gorm.io/gorm"
)

// gdb is the package-singleton gorm.DB. Once GetDb has constructed
// it the first time, every subsequent call returns the same handle —
// the connection pool sized at SetMaxOpenConns(1) below makes
// concurrent writes safe under SQLite's "one writer at a time" rule.
var gdb *gorm.DB

// GetDb returns the user-service SQLite database, lazily opening it
// from dbPath/user.db on the first call. Subsequent calls reuse the
// same handle.
//
// The database is configured with a single open connection
// (SetMaxOpenConns(1)) because SQLite serializes writes process-wide
// and multiple writers race-deadlock on file locks. Up to 10 idle
// connections are kept warm; idle conns expire after ~16 minutes.
//
// Schema migration runs eagerly via gorm.AutoMigrate on UserDBModel
// and EventModel at first call. A failure here is logged but does
// NOT panic — the caller still gets a *gorm.DB and may proceed with
// best-effort reads against whatever schema is currently on disk.
// Versioned migrations are tracked in #100.
func GetDb(dbPath string) *gorm.DB {
	if gdb != nil {
		return gdb
	}

	file.IsNotExistMkDir(dbPath)
	db, err := gorm.Open(sqlite.Open(dbPath+"/user.db"), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	c, _ := db.DB()
	c.SetMaxIdleConns(10)
	c.SetMaxOpenConns(1)
	c.SetConnMaxIdleTime(time.Second * 1000)

	gdb = db

	// Run versioned migrations in place of GORM's AutoMigrate.
	// migrationsFS is defined in migrations.go alongside the .sql
	// files. ADR-0018 documents the choice (goose) and the
	// rationale for retiring AutoMigrate.
	//
	// On a pre-existing install with tables already created by
	// AutoMigrate, the 0001_initial.sql uses CREATE TABLE IF NOT
	// EXISTS so the migration is a safe no-op that simply records
	// the schema as being at version 1 in goose_db_version.
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		if err := pkgmigrations.Up(context.Background(), sqlDB, migrationsFS, "migrations"); err != nil {
			_log.Error(context.Background(), "migration error", err)
		}
	}
	return db
}
