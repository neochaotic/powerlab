// Package model holds the user-service's config + DTO types. The
// CommonModel / APPModel pair mirrors the [common] / [user] sections
// of `/etc/powerlab/user-service.conf`; Result is the standard
// envelope for every JSON response from this service.
package model

// CommonModel is the [common] section of user-service.conf —
// process-level paths shared with other PowerLab services.
type CommonModel struct {
	RuntimePath string
}

// APPModel is the [user] section of user-service.conf — the
// user-service's own paths.
type APPModel struct {
	LogPath      string
	LogSaveName  string
	LogFileExt   string
	UserDataPath string
	DBPath       string
}

// Result is the standard JSON envelope: every user-service handler
// returns one of these. Success is the protocol-level status code
// (mirrored to HTTP), Message is the i18n key, Data is the
// optional handler-specific payload.
type Result struct {
	Success int         `json:"success" example:"200"`
	Message string      `json:"message" example:"ok"`
	Data    interface{} `json:"data" example:"返回结果"`
}
