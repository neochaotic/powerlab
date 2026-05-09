package sqlite

import (
	"context"
	"time"

	"github.com/IceWhaleTech/CasaOS-UserService/model"
	"github.com/IceWhaleTech/CasaOS-UserService/pkg/utils/file"
	model2 "github.com/IceWhaleTech/CasaOS-UserService/service/model"
	"github.com/glebarez/sqlite"
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

	err = db.AutoMigrate(model2.UserDBModel{}, model.EventModel{})
	if err != nil {
		_log.Error(context.Background(), "check or create db error", err)
	}
	return db
}
