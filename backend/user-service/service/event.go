package service

import (
	"encoding/json"

	"github.com/neochaotic/powerlab/backend/user-service/model"
	"gorm.io/gorm"
)

// EventService is the local-event log persisted in user.db. NOT
// the same as the message-bus topic stream — these are user-scoped
// events (login, password reset, profile change) the user-service
// records for its own audit trail. Read by the /v1/users/event
// route bundle.
type EventService interface {
	CreateEvemt(m model.EventModel) model.EventModel
	GetEvents() (list []model.EventModel)
	GetEventByUUID(uuid string) (m model.EventModel)
	DeleteEvent(uuid string)
	DeleteEventBySerial(serial string)
}

type eventService struct {
	db *gorm.DB
}

func (e *eventService) CreateEvemt(m model.EventModel) model.EventModel {
	e.db.Create(&m)
	return m
}
func (e *eventService) GetEvents() (list []model.EventModel) {
	e.db.Find(&list)
	return
}
func (e *eventService) GetEventByUUID(uuid string) (m model.EventModel) {
	e.db.Where("uuid = ?", uuid).First(&m)
	return
}
func (e *eventService) DeleteEvent(uuid string) {
	e.db.Where("uuid = ?", uuid).Delete(&model.EventModel{})
}
func (e *eventService) DeleteEventBySerial(serial string) {
	list := []model.EventModel{}
	e.db.Find(&list)
	for _, v := range list {

		if v.SourceID == "local-storage" {
			properties := make(map[string]string)
			err := json.Unmarshal([]byte(v.Properties), &properties)
			if err != nil {
				continue
			}
			if properties["serial"] == serial {
				e.db.Delete(&v)
			}
		}
	}
}
// NewEventService constructs an EventService backed by user.db
// (same DB as the user store; events live in the `events` table
// per migration 0001).
func NewEventService(db *gorm.DB) EventService {
	return &eventService{db: db}
}
