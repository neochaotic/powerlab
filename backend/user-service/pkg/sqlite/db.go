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

var gdb *gorm.DB

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
