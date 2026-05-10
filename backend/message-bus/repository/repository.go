package repository

import (
	"github.com/neochaotic/powerlab/backend/message-bus/model"
	"github.com/neochaotic/powerlab/backend/message-bus/pkg/ysk"
)

// Repository is the storage contract for message-bus state. Two
// physical sqlite DBs back this interface: an event/action-type DB
// (EventType / ActionType registrations) and a persist DB
// (YSKCard pinned-card list + Settings k/v). The split lets the
// event DB be wiped on schema change without losing UI state.
//
// Implementations: DatabaseRepository (sqlite, prod) +
// NewDatabaseRepositoryInMemory (per-test isolation).
type Repository interface {
	GetEventTypes() ([]model.EventType, error)
	RegisterEventType(eventType model.EventType) (*model.EventType, error)
	GetEventTypesBySourceID(sourceID string) ([]model.EventType, error)
	GetEventType(sourceID string, name string) (*model.EventType, error)

	GetActionTypes() ([]model.ActionType, error)
	RegisterActionType(actionType model.ActionType) (*model.ActionType, error)
	GetActionTypesBySourceID(sourceID string) ([]model.ActionType, error)
	GetActionType(sourceID string, name string) (*model.ActionType, error)

	GetYSKCardList() ([]ysk.YSKCard, error)
	UpsertYSKCard(card ysk.YSKCard) error
	DeleteYSKCard(id string) error

	GetSettings(key string) (*model.Settings, error)
	UpsertSettings(settings model.Settings) error

	Close()
}
