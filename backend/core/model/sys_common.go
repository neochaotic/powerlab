package model

import "time"

// SysInfoModel mirrors the [system] config section — currently
// just the system name shown in the UI title bar.
type SysInfoModel struct {
	Name string // 系统名称
}

// ServerModel mirrors the [server] section — admin-toggleable
// runtime flags. LockAccount is the post-failed-login lockout
// switch; USBAutoMount controls hot-plug auto-mount.
type ServerModel struct {
	HttpPort     string
	RunMode      string
	LockAccount  bool
	Token        string
	USBAutoMount string
}

// APPModel mirrors the [app] section — paths the service writes to
// (logs, sqlite DB, user-data, shell-script helper) plus the
// date/time format strings the legacy V1 endpoints echo back.
type APPModel struct {
	LogPath        string
	LogSaveName    string
	LogFileExt     string
	DateStrFormat  string
	DateTimeFormat string
	UserDataPath   string
	TimeFormat     string
	DateFormat     string
	DBPath         string
	ShellPath      string
}

// CommonModel mirrors the [common] section — host-wide paths
// shared across all PowerLab services.
type CommonModel struct {
	RuntimePath string
}

// Result is the legacy V1 JSON response envelope. Same shape as
// common's model.Result — kept duplicated so the core service
// stays loosely coupled.
type Result struct {
	Success int         `json:"success" example:"200"`
	Message string      `json:"message" example:"ok"`
	Data    interface{} `json:"data" example:"返回结果"`
}

// RedisModel mirrors the [redis] section. PowerLab does not
// currently ship with redis enabled; the type is retained so
// existing CasaOS configs load without errors.
type RedisModel struct {
	Host        string
	Password    string
	MaxIdle     int
	MaxActive   int
	IdleTimeout time.Duration
}

// SystemConfig holds runtime-discovered config paths surfaced via
// the V1 sys-info endpoint.
type SystemConfig struct {
	ConfigPath string `json:"config_path"`
}

// FileSetting is the persisted per-user file-browser preferences:
// remembered share dirs + the default download target. The "|"
// delim tag is the legacy CasaOS storage format.
type FileSetting struct {
	ShareDir    []string `json:"share_dir" delim:"|"`
	DownloadDir string   `json:"download_dir"`
}

// BaseInfo is the compact device-id payload sent in the auto-
// discovery beacon. Single-letter JSON keys are the over-the-wire
// format used by zimaos-discovery clients.
type BaseInfo struct {
	Hash       string `json:"i"`
	Version    string `json:"v"`
	Channel    string `json:"c,omitempty"`
	DriveModel string `json:"m,omitempty"`
}
