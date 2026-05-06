package model

type UserInfo struct {
	NickName string `json:"nick_name"`
	Desc     string `json:"desc"`
	ShareId  string `json:"share_id"`
	Avatar   string `json:"avatar"`
	Version  int    `json:"version,omitempty"`
}

type UserDBModel struct {
	ID uint `json:"id"`
}

type SystemUser struct {
	Username string `json:"username"`
	UID      string `json:"uid"`
	GID      string `json:"gid"`
	HomeDir  string `json:"home_dir"`
	Shell    string `json:"shell"`
}
