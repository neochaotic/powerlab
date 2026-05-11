package route

import (
	"encoding/json"
	"os"

	file1 "github.com/neochaotic/powerlab/backend/common/utils/file"
	"github.com/neochaotic/powerlab/backend/common/utils/logger"
	"github.com/neochaotic/powerlab/backend/core/common"
	"github.com/neochaotic/powerlab/backend/core/model"
	"github.com/neochaotic/powerlab/backend/core/pkg/config"
	"github.com/neochaotic/powerlab/backend/core/pkg/utils/encryption"
	"github.com/neochaotic/powerlab/backend/core/pkg/utils/file"
	v1 "github.com/neochaotic/powerlab/backend/core/route/v1"
	"github.com/neochaotic/powerlab/backend/core/service"
	"go.uber.org/zap"
)

func InitFunction() {
	go InitInfo()
	//go InitZerotier()
}

func InitInfo() {
	mb := model.BaseInfo{}
	if file.Exists(config.AppInfo.DBPath + "/baseinfo.conf") {
		err := json.Unmarshal(file.ReadFullFile(config.AppInfo.DBPath+"/baseinfo.conf"), &mb)
		if err != nil {
			logger.Error("baseinfo.conf", zap.String("error", err.Error()))
		}
	}
	if file.Exists("/etc/CHANNEL") {
		channel := file.ReadFullFile("/etc/CHANNEL")
		mb.Channel = string(channel)
	}
	mac, err := service.MyService.System().GetMacAddress()
	if err != nil {
		logger.Error("GetMacAddress", zap.String("error", err.Error()))
	}
	mb.Hash = encryption.GetMD5ByStr(mac)
	mb.Version = common.VERSION
	osRelease, _ := file1.ReadOSRelease()

	mb.DriveModel = osRelease["MODEL"]
	if len(mb.DriveModel) == 0 {
		mb.DriveModel = "PowerLab"
	}
	os.Remove(config.AppInfo.DBPath + "/baseinfo.conf")
	by, err := json.Marshal(mb)
	if err != nil {
		logger.Error("init info err", zap.Any("err", err))
		return
	}
	file.WriteToFullPath(by, config.AppInfo.DBPath+"/baseinfo.conf", 0o666)
}

func InitZerotier() {
	v1.CheckNetwork()
}
