package repository

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/neochaotic/powerlab/backend/message-bus/model"
	"github.com/neochaotic/powerlab/backend/message-bus/pkg/ysk"
	pkgmigrations "github.com/neochaotic/powerlab/backend/pkg/migrations"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DatabaseRepository struct {
	db        *gorm.DB
	persistDB *gorm.DB
}

func (r *DatabaseRepository) GetYSKCardList() ([]ysk.YSKCard, error) {
	var cardList []ysk.YSKCard
	if err := r.persistDB.Order("updated desc, id desc").Find(&cardList).Error; err != nil {
		return nil, err
	}
	return cardList, nil
}

func (r *DatabaseRepository) UpsertYSKCard(card ysk.YSKCard) error {
	return r.persistDB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).Create(&card).Error
}

func (r *DatabaseRepository) DeleteYSKCard(id string) error {
	return r.persistDB.Where("id LIKE ?", id+"%").Delete(&ysk.YSKCard{}).Error
}

func (r *DatabaseRepository) GetEventTypes() ([]model.EventType, error) {
	var eventTypes []model.EventType

	if err := r.db.Preload(model.PropertyTypeList).Find(&eventTypes).Error; err != nil {
		return nil, err
	}

	return eventTypes, nil
}

func (r *DatabaseRepository) GetSettings(key string) (*model.Settings, error) {
	var settings model.Settings
	if err := r.persistDB.Where(&model.Settings{Key: key}).First(&settings).Error; err != nil {
		return nil, err
	}
	return &settings, nil
}

func (r *DatabaseRepository) UpsertSettings(settings model.Settings) error {
	return r.persistDB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		UpdateAll: true,
	}).Create(&settings).Error
}

func (r *DatabaseRepository) RegisterEventType(eventType model.EventType) (*model.EventType, error) {
	// upsert
	if err := r.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&eventType).Error; err != nil {
		return nil, err
	}

	return &eventType, nil
}

func (r *DatabaseRepository) GetEventTypesBySourceID(sourceID string) ([]model.EventType, error) {
	var eventTypes []model.EventType

	if err := r.db.Preload(model.PropertyTypeList).Where(&model.EventType{SourceID: sourceID}).Find(&eventTypes).Error; err != nil {
		return nil, err
	}

	return eventTypes, nil
}

func (r *DatabaseRepository) GetEventType(sourceID string, name string) (*model.EventType, error) {
	var eventType model.EventType

	if err := r.db.Preload(model.PropertyTypeList).Where(&model.EventType{SourceID: sourceID, Name: name}).First(&eventType).Error; err != nil {
		_log.Error(context.Background(), "can't find event type", err, slog.String("sourceID", sourceID), slog.String("EventName", name))
		return nil, err
	}

	return &eventType, nil
}

func (r *DatabaseRepository) GetActionTypes() ([]model.ActionType, error) {
	return GetTypes[model.ActionType](r.db)
}

func (r *DatabaseRepository) RegisterActionType(actionType model.ActionType) (*model.ActionType, error) {
	return RegisterType(r.db, actionType)
}

func (r *DatabaseRepository) GetActionTypesBySourceID(sourceID string) ([]model.ActionType, error) {
	return GetTypesBySourceID[model.ActionType](r.db, sourceID)
}

func (r *DatabaseRepository) GetActionType(sourceID string, name string) (*model.ActionType, error) {
	return GetType[model.ActionType](r.db, sourceID, name)
}

func (r *DatabaseRepository) Close() {
	for _, db := range []*gorm.DB{r.db, r.persistDB} {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	}
}

func GetTypes[T any](db *gorm.DB) ([]T, error) {
	var types []T

	if err := db.Preload(model.PropertyTypeList).Find(&types).Error; err != nil {
		return nil, err
	}

	return types, nil
}

func RegisterType[T any](db *gorm.DB, t T) (*T, error) {
	// upsert
	if err := db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&t).Error; err != nil {
		return nil, err
	}

	return &t, nil
}

func GetTypesBySourceID[T any](db *gorm.DB, sourceID string) ([]T, error) {
	var types []T

	if err := db.Preload(model.PropertyTypeList).Where(&model.GenericType{SourceID: sourceID}).Find(&types).Error; err != nil {
		return nil, err
	}

	return types, nil
}

func GetType[T any](db *gorm.DB, sourceID string, name string) (*T, error) {
	var t T

	if err := db.Preload(model.PropertyTypeList).Where(&model.GenericType{SourceID: sourceID, Name: name}).First(&t).Error; err != nil {
		return nil, err
	}

	return &t, nil
}

func NewDatabaseRepositoryInMemory() (Repository, error) {
	// Use distinct named in-memory shared caches per DB. Previously
	// both DBs were "file::memory:?cache=shared" — the same identifier
	// — which made them connect to the SAME backing store. With
	// pkg/migrations, goose's version-tracking table is then shared
	// across the two migration runs, so the second Up sees v=1 from
	// the first and skips its own migrations. Result: persistDB never
	// gets settings/ysk_cards.
	//
	// Each cache name produces its own isolated in-memory DB.
	return NewDatabaseRepository(
		"file:events?mode=memory&cache=shared",
		"file:persist?mode=memory&cache=shared",
	)
}

func NewDatabaseRepository(databaseFilePath string, persistDatabaseFilePath string) (Repository, error) {
	// mkdir dbpath, 777 is copied from zimaos-local-storage
	if err := os.MkdirAll(filepath.Dir(databaseFilePath), 0o777); err != nil {
		return nil, err
	}
	// And the *persist* db path. Without this, fresh installs panic at
	// startup with sqlite's "out of memory (14)" — really
	// SQLITE_CANTOPEN — because /var/lib/powerlab/db/ doesn't exist
	// yet on a system that's never run message-bus before.
	if err := os.MkdirAll(filepath.Dir(persistDatabaseFilePath), 0o777); err != nil {
		return nil, err
	}
	db, err := gorm.Open(sqlite.Open(databaseFilePath))
	if err != nil {
		return nil, err
	}
	persistDB, err := gorm.Open(sqlite.Open(persistDatabaseFilePath))
	if err != nil {
		return nil, err
	}

	for _, db := range []*gorm.DB{db, persistDB} {
		if c, err := db.DB(); err == nil {
			c.SetMaxIdleConns(10)
			c.SetMaxOpenConns(1)
			c.SetConnMaxIdleTime(1000 * time.Second)
		}
	}

	// Run versioned migrations in place of GORM's AutoMigrate. Two
	// embed.FS — one per DB — because each goose run owns its own
	// goose_db_version table and migration sequence. ADR-0018.
	//
	// On a pre-existing install with tables already created by
	// AutoMigrate, the 0001_initial.sql files use CREATE TABLE
	// IF NOT EXISTS so the migration is a safe no-op that simply
	// records the schema as version 1 in goose_db_version.
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		if err := pkgmigrations.Up(context.Background(), sqlDB, migrationsEventsFS, "migrations_events"); err != nil {
			return nil, err
		}
	}
	if sqlDB, dbErr := persistDB.DB(); dbErr == nil {
		if err := pkgmigrations.Up(context.Background(), sqlDB, migrationsPersistFS, "migrations_persist"); err != nil {
			return nil, err
		}
	}

	return &DatabaseRepository{
		db:        db,
		persistDB: persistDB,
	}, nil
}
