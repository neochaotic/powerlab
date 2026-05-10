package model

// Settings is a key/value row in the persist DB used for cross-restart
// state that doesn't deserve its own table — e.g. the current YSK
// pinned-card ordering, feature flags written from the admin UI.
type Settings struct {
	Key   string `gorm:"primaryKey"`
	Value string
}
