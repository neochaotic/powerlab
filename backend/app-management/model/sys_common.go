// Package model holds DB row + config types for the app-management
// service. App-store catalog shapes live in app.go / category.go;
// installed-app manifests in manifest.go; system config here.
package model

// CommonModel mirrors the [common] section of app-management.conf —
// host-wide paths shared across all PowerLab services.
type CommonModel struct {
	RuntimePath string
}

// APPModel mirrors the [app] section — paths the service writes to
// (logs, app catalog cache, installed-app config + bind-mount data).
type APPModel struct {
	LogPath      string
	LogSaveName  string
	LogFileExt   string
	AppStorePath string
	AppsPath     string
	StoragePath  string // root path for app data volumes (e.g. /DATA on Linux, local path on macOS)
}

// ServerModel mirrors the [server] section — currently just the
// admin-configured app-store list. Each entry is a git URL + branch
// the catalog refresher pulls from.
type ServerModel struct {
	AppStoreList []string `ini:"appstore,,allowshadow"`
}

// GlobalModel holds runtime-injected secrets (currently just the
// OpenAI key the AI assistant feature uses). Not persisted to the
// conf file — set via env or admin endpoint.
type GlobalModel struct {
	OpenAIAPIKey string
}

// AppLifecycleFlags is process-global state used to invalidate
// app-list caches when something changes. AppChange is set true by
// install/uninstall handlers in route/v1/docker.go and consumed by
// service/container.go's GetContainerAppList to know when to skip
// the cache and re-enumerate.
//
// Sprint 4 PR2 rename (#85): was `CasaOSGlobalVariables`, an
// unhelpful name that surfaced upstream branding without describing
// the actual purpose. Same struct, same one field.
type AppLifecycleFlags struct {
	AppChange bool
}
