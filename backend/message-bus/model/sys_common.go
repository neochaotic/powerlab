package model

// CommonModel mirrors the [common] section of message-bus.conf —
// host-wide paths shared across all PowerLab services.
type CommonModel struct {
	RuntimePath string
}

// APPModel mirrors the [app] section of message-bus.conf — paths the
// service writes to (logs, sqlite DB). Defaulted from
// constants.Default* on first boot when the conf file is generated.
type APPModel struct {
	LogPath     string
	LogSaveName string
	LogFileExt  string
	DBPath      string
}
