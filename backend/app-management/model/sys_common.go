package model

type CommonModel struct {
	RuntimePath string
}

type APPModel struct {
	LogPath      string
	LogSaveName  string
	LogFileExt   string
	AppStorePath string
	AppsPath     string
	StoragePath  string // root path for app data volumes (e.g. /DATA on Linux, local path on macOS)
}

type ServerModel struct {
	AppStoreList []string `ini:"appstore,,allowshadow"`
}

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
