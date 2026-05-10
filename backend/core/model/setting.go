package model

// Setting Group identifiers — used by the admin UI to render groups
// of settings on separate panels. Stable; reordering is a breaking
// change for the frontend.
const (
	SINGLE = iota
	SITE
	STYLE
	PREVIEW
	GLOBAL
	ARIA2
	INDEX
	GITHUB
)

// Setting Flag values — visibility / mutability classification.
// PUBLIC: editable in the UI; PRIVATE: not exposed; READONLY: shown
// but not editable; DEPRECATED: hidden + scheduled for removal.
const (
	PUBLIC = iota
	PRIVATE
	READONLY
	DEPRECATED
)

// SettingItem is one row of the persistent settings KV store.
// Same shape as local-storage's SettingItem — kept in sync
// across services but lives separately so neither depends on the
// other's gorm setup.
type SettingItem struct {
	Key     string `json:"key" gorm:"primaryKey" binding:"required"` // unique key
	Value   string `json:"value"`                                    // value
	Help    string `json:"help"`                                     // help message
	Type    string `json:"type"`                                     // string, number, bool, select
	Options string `json:"options"`                                  // values for select
	Group   int    `json:"group"`                                    // use to group setting in frontend
	Flag    int    `json:"flag"`                                     // 0 = public, 1 = private, 2 = readonly, 3 = deprecated, etc.
}

// IsDeprecated reports whether the setting has been flagged for
// removal — admin UI hides deprecated rows.
func (s SettingItem) IsDeprecated() bool {
	return s.Flag == DEPRECATED
}
