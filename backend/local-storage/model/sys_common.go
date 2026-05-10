// Package model holds DB row + config types for the local-storage
// service. Storage-specific shapes live in obj.go / object.go;
// settings + system config live here.
package model

// CommonModel mirrors the [common] section of local-storage.conf —
// host-wide paths shared across all PowerLab services.
type CommonModel struct {
	RuntimePath string
}

// APPModel mirrors the [app] section of local-storage.conf — paths
// the service writes to (logs, sqlite DB) plus the shell-script
// helper path used for mount/format operations.
type APPModel struct {
	LogPath     string
	LogSaveName string
	LogFileExt  string
	ShellPath   string
	DBPath      string
}

// ServerModel mirrors the [server] section — admin-toggleable
// behavioural flags. USBAutoMount controls auto-mount on hot-plug;
// EnableMergerFS controls whether new disks join the unified pool.
type ServerModel struct {
	USBAutoMount   string
	EnableMergerFS string
}
