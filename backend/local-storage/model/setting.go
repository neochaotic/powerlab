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
// Help/Type/Options are render hints for the admin UI; Group + Flag
// drive panel placement and visibility.
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
// removal. The admin UI hides deprecated rows; the persistence
// layer keeps them so existing values aren't lost on read.
func (s SettingItem) IsDeprecated() bool {
	return s.Flag == DEPRECATED
}
