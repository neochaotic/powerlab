package config

import (
	"fmt"
	"log"
	"os"

	"github.com/neochaotic/powerlab/backend/common/utils/constants"
	"github.com/neochaotic/powerlab/backend/message-bus/common"
	"github.com/neochaotic/powerlab/backend/message-bus/model"
	"gopkg.in/ini.v1"
)

var (
	CommonInfo = &model.CommonModel{
		RuntimePath: constants.DefaultRuntimePath,
	}

	AppInfo = &model.APPModel{
		LogPath:     constants.DefaultLogPath,
		LogSaveName: common.MessageBusServiceName,
		DBPath:      constants.DefaultDataPath,
		LogFileExt:  "log",
	}

	// SecurityInfo mirrors the [security] section of message-bus.conf.
	// Empty AllowedOrigins → only same-origin SocketIO connections
	// pass the check. See #219 + ADR-0023.
	SecurityInfo = &model.SecurityModel{
		AllowedOrigins: "",
	}

	Cfg            *ini.File
	ConfigFilePath string
)

// InitSetup loads message-bus.conf into the package-level Cfg / CommonInfo
// / AppInfo singletons. If the file at config (or MessageBusConfigFilePath
// when config is empty) does not exist, the embedded sample string is
// written to disk first so a fresh install boots with sane defaults.
//
// Called once at process start before any other package reads config.
func InitSetup(config string, sample string) {
	ConfigFilePath = MessageBusConfigFilePath
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

	Cfg, err = ini.Load(ConfigFilePath)
	if err != nil {
		panic(err)
	}

	mapTo("common", CommonInfo)
	mapTo("app", AppInfo)
	mapTo("security", SecurityInfo)
}

func mapTo(section string, v interface{}) {
	err := Cfg.Section(section).MapTo(v)
	if err != nil {
		log.Fatalf("Cfg.MapTo %s err: %v", section, err)
	}
}
