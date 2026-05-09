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

	Cfg            *ini.File
	ConfigFilePath string
)

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
}

func mapTo(section string, v interface{}) {
	err := Cfg.Section(section).MapTo(v)
	if err != nil {
		log.Fatalf("Cfg.MapTo %s err: %v", section, err)
	}
}
