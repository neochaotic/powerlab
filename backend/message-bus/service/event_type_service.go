package service

import (
	"errors"

	"github.com/neochaotic/powerlab/backend/message-bus/model"
	"github.com/neochaotic/powerlab/backend/message-bus/repository"
)

// EventTypeService is the registry of EventType schemas. Symmetric
// with ActionTypeService; same wrapper-over-Repository pattern.
type EventTypeService struct {
	repository *repository.Repository
}

var (
	ErrEventSourceIDNotFound = errors.New("event source id not found")
	ErrEventNameNotFound     = errors.New("event name not found")
)

// GetEventTypes returns every registered EventType.
func (s *EventTypeService) GetEventTypes() ([]model.EventType, error) {
	return (*s.repository).GetEventTypes()
}

// RegisterEventType upserts an EventType keyed on (SourceID, Name).
// Idempotent.
func (s *EventTypeService) RegisterEventType(eventType model.EventType) (*model.EventType, error) {
	// TODO - ensure sourceID and name are URL safe

	return (*s.repository).RegisterEventType(eventType)
}

// GetEventTypesBySourceID returns every EventType registered under
// the given publisher.
func (s *EventTypeService) GetEventTypesBySourceID(sourceID string) ([]model.EventType, error) {
	return (*s.repository).GetEventTypesBySourceID(sourceID)
}

// GetEventType returns the EventType identified by (sourceID, name)
// or gorm.ErrRecordNotFound.
func (s *EventTypeService) GetEventType(sourceID string, name string) (*model.EventType, error) {
	return (*s.repository).GetEventType(sourceID, name)
}

// NewEventTypeService constructs an EventTypeService that delegates
// to the given Repository.
func NewEventTypeService(repository *repository.Repository) *EventTypeService {
	return &EventTypeService{
		repository: repository,
	}
}
