// Package model defines the wire-shape and DB-row types for the
// message-bus service. EventType / ActionType are the schema
// declarations a publisher registers up-front; Event / Action are
// the live instances that flow through the bus.
package model

// Discriminators used by route handlers + repository code to refer
// to type-list endpoints + bulk operations.
const (
	EventTypeList    = "EventTypeList"
	ActionTypeList   = "ActionTypeList"
	PropertyTypeList = "PropertyTypeList"
)

// EventType is a publisher-registered event schema. SourceID names
// the producing service (e.g. "user-service"); Name is the event
// kind (e.g. "powerlab:user:save_config"). PropertyTypeList
// enumerates the named keys event Properties may carry — readers
// can use it for validation + UI introspection.
type EventType struct {
	SourceID         string         `gorm:"primaryKey"`
	Name             string         `gorm:"primaryKey"`
	PropertyTypeList []PropertyType `gorm:"many2many:event_type_property_type;"`
}

// Event is a live event published to the bus. Properties is a
// free-form key/value bag whose allowed keys are defined by the
// matching EventType.PropertyTypeList; Timestamp is the producer-
// side wall-clock millis (not bus-arrival time). UUID lets
// idempotent consumers de-dupe.
type Event struct {
	ID         uint              `gorm:"primaryKey"`
	SourceID   string            `gorm:"index"`
	Name       string            `gorm:"index"`
	Properties map[string]string `gorm:"foreignKey:Id"`
	Timestamp  int64             `gorm:"autoCreateTime:milli"`
	UUID       string            `json:"uuid,omitempty"`
}

// ActionType is a publisher-registered action schema — symmetric
// with EventType but for request-shaped traffic (the bus routes
// actions to the registered handler instead of fan-out
// broadcasting).
type ActionType struct {
	SourceID         string         `gorm:"primaryKey"`
	Name             string         `gorm:"primaryKey"`
	PropertyTypeList []PropertyType `gorm:"many2many:action_type_property_type;"`
}

// Action is a live action dispatched to the bus. Same shape as
// Event but with request-response semantics handled by the
// matching ActionType handler.
type Action struct {
	ID         uint              `gorm:"primaryKey"`
	SourceID   string            `gorm:"index"`
	Name       string            `gorm:"index"`
	Properties map[string]string `gorm:"foreignKey:Id"`
	Timestamp  int64             `gorm:"autoCreateTime:milli"`
}

// PropertyType names a key that may appear in an Event/Action's
// Properties map. Many2many'd to types so a single property can
// be reused across schemas.
type PropertyType struct {
	Name string `gorm:"primaryKey"`
}

// GenericType is the shared SourceID + Name pair used by helper
// queries that don't care whether they're looking at events or
// actions — internal optimisation, not part of the public API.
type GenericType struct {
	SourceID string `gorm:"primaryKey"`
	Name     string `gorm:"primaryKey"`
}
