// Package model holds the gorm row types + JSON shapes the core
// service speaks. Storage backends, file-stream wrappers, settings,
// system-user descriptors, the legacy V1 response envelope, etc.
package model

// UserInfo is the public profile envelope returned by the legacy V1
// user-info endpoint. Mirrors a subset of user-service's UserDBModel
// for clients that haven't migrated to the dedicated user-service
// API yet.
type UserInfo struct {
	NickName string `json:"nick_name"`
	Desc     string `json:"desc"`
	ShareId  string `json:"share_id"`
	Avatar   string `json:"avatar"`
	Version  int    `json:"version,omitempty"`
}

// UserDBModel is the minimal user reference used inside core's own
// gorm rows — just the foreign key. Full user state lives in the
// user-service.
type UserDBModel struct {
	ID uint `json:"id"`
}

// SystemUser describes a Linux system user (from /etc/passwd) — used
// by the SMB-share + file-permissions code that needs to map a
// PowerLab user back to its on-disk uid/gid.
type SystemUser struct {
	Username string `json:"username"`
	UID      string `json:"uid"`
	GID      string `json:"gid"`
	HomeDir  string `json:"home_dir"`
	Shell    string `json:"shell"`
}
