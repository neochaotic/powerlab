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

type CasaOSGlobalVariables struct {
	AppChange bool
}
