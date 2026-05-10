package service

import (
	"errors"

	"github.com/neochaotic/powerlab/backend/message-bus/model"
	"github.com/neochaotic/powerlab/backend/message-bus/repository"
)

// ActionTypeService is the registry of ActionType schemas. Thin
// wrapper over the Repository; lives here so the route layer
// depends on a service-shaped seam, not on the storage interface
// directly.
type ActionTypeService struct {
	repository *repository.Repository
}

var (
	ErrActionSourceIDNotFound = errors.New("event source id not found")
	ErrActionNameNotFound     = errors.New("event name not found")
)

// GetActionTypes returns every registered ActionType.
func (s *ActionTypeService) GetActionTypes() ([]model.ActionType, error) {
	return (*s.repository).GetActionTypes()
}

// RegisterActionType upserts an ActionType keyed on (SourceID, Name).
// Idempotent — publishers call on every startup.
func (s *ActionTypeService) RegisterActionType(actionType model.ActionType) (*model.ActionType, error) {
	// TODO - ensure sourceID and name are URL safe

	return (*s.repository).RegisterActionType(actionType)
}

// GetActionTypesBySourceID returns every ActionType registered under
// the given publisher.
func (s *ActionTypeService) GetActionTypesBySourceID(sourceID string) ([]model.ActionType, error) {
	return (*s.repository).GetActionTypesBySourceID(sourceID)
}

// GetActionType returns the ActionType identified by (sourceID, name)
// or gorm.ErrRecordNotFound.
func (s *ActionTypeService) GetActionType(sourceID string, name string) (*model.ActionType, error) {
	return (*s.repository).GetActionType(sourceID, name)
}

// NewActionTypeService constructs an ActionTypeService that delegates
// to the given Repository.
func NewActionTypeService(repository *repository.Repository) *ActionTypeService {
	return &ActionTypeService{
		repository: repository,
	}
}
