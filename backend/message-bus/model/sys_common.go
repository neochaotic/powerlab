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

// SecurityModel mirrors the [security] section of message-bus.conf —
// hardening knobs the operator can tune without rebuilding. Closes
// audit finding #219: the SocketIO transports previously accepted
// any Origin header. AllowedOrigins is a comma-separated list of
// full origins (e.g. `http://my-other-app.local:3000`); same-origin
// requests are always allowed without explicit configuration.
type SecurityModel struct {
	AllowedOrigins string
}
