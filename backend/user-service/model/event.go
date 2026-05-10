package model

// EventModel is the DB row for a user-scoped event (login,
// password reset, etc.) recorded by user-service in user.db's
// `events` table. Properties is a serialised JSON map; SourceID
// names the originating service.
type EventModel struct {
	UUID       string `gorm:"primaryKey" json:"uuid"`
	SourceID   string `gorm:"index" json:"source_id"`
	Name       string `json:"name"`
	Properties string `gorm:"serializer:json" json:"properties"`
	Timestamp  int64  `gorm:"autoCreateTime:milli" json:"timestamp"`
}

func (p *EventModel) TableName() string {
	return "events"
}
