package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/go-ini/ini"
	"github.com/neochaotic/powerlab/backend/common/utils/constants"
	"github.com/neochaotic/powerlab/backend/core/common"
	"github.com/neochaotic/powerlab/backend/core/model"
)

var (
	SysInfo = &model.SysInfoModel{}
	AppInfo = &model.APPModel{
		DBPath:       constants.DefaultDataPath,
		LogPath:      constants.DefaultLogPath,
		LogSaveName:  common.SERVICENAME,
		LogFileExt:   "log",
		ShellPath:    filepath.Join(constants.DefaultConstantPath, "shell"),
		UserDataPath: filepath.Join(constants.DefaultDataPath, "conf"),
	}
	CommonInfo = &model.CommonModel{
		RuntimePath: constants.DefaultRuntimePath,
	}
	ServerInfo       = &model.ServerModel{}
	SystemConfigInfo = &model.SystemConfig{}
	FileSettingInfo  = &model.FileSetting{}

	Cfg            *ini.File
	ConfigFilePath string
)

// InitSetup loads core.conf into the package-level singletons
// (Cfg, AppInfo, ServerInfo, etc.). If the file at config (or the
// CoreConfigFilePath default when config is empty) does not exist,
// the embedded sample string is written to disk first so a fresh
// install boots with sane defaults. Called once at process start
// before any other package reads config.
func InitSetup(config string, sample string) {
	ConfigFilePath = CoreConfigFilePath
	if len(config) > 0 {
		ConfigFilePath = config
	}

	// create default config file if not exist
	if _, err := os.Stat(ConfigFilePath); os.IsNotExist(err) {
		fmt.Println("config file not exist, create it")
		// create config file
		file, err := os.Create(ConfigFilePath)
		if err != nil {
			panic(err)
		}
		defer file.Close()

		// write default config
		_, err = file.WriteString(sample)
		if err != nil {
			panic(err)
		}
	}

	var err error

	// 读取文件
	Cfg, err = ini.Load(ConfigFilePath)
	if err != nil {
		panic(err)
	}

	mapTo("app", AppInfo)
	mapTo("server", ServerInfo)
	mapTo("system", SystemConfigInfo)
	mapTo("file", FileSettingInfo)
	mapTo("common", CommonInfo)
}

// 映射
func mapTo(section string, v interface{}) {
	err := Cfg.Section(section).MapTo(v)
	if err != nil {
		log.Fatalf("Cfg.MapTo %s err: %v", section, err)
	}
}
