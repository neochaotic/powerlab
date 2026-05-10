package service

import (
	"context"
	"log/slog"
	"os"

	"github.com/neochaotic/powerlab/backend/common/utils/command"
	"github.com/neochaotic/powerlab/backend/local-storage/pkg/config"
	"github.com/shirou/gopsutil/host"
)

// USBService manages the USB-auto-mount toggle (admin UI setting +
// the underlying udev rule that implements it) and exposes a couple
// of host-info helpers used by the device-tree-aware mount logic on
// SBCs.
type USBService interface {
	// UpdateUSBAutoMount writes state ("True"/"False") to the
	// [server] USBAutoMount config key and persists the conf file.
	UpdateUSBAutoMount(state string)
	// ExecUSBAutoMountShell runs the helper shell script that
	// installs/removes the udev rule mirroring the in-memory state.
	ExecUSBAutoMountShell(state string)
	// GetSysInfo returns the current host info (gopsutil) — used to
	// pick mount strategies on RPi vs x86.
	GetSysInfo() host.InfoStat
	// GetDeviceTree returns the contents of /proc/device-tree/model
	// (Linux SBCs) or empty string on systems without it.
	GetDeviceTree() (string, error)
}

type usbService struct{}

func (s *usbService) UpdateUSBAutoMount(state string) {
	config.ServerInfo.USBAutoMount = state
	config.Cfg.Section("server").Key("USBAutoMount").SetValue(state)
	if err := config.Cfg.SaveTo(config.ConfigFilePath); err != nil {
		_log.Error(context.Background(), "error when saving USB automount configuration", err, slog.String("path", config.ConfigFilePath))
	}
}

func (s *usbService) ExecUSBAutoMountShell(state string) {
	if state == "False" {
		if _, err := command.OnlyExec("source " + config.AppInfo.ShellPath + "/local-storage-helper.sh ;USB_Stop_Auto"); err != nil {
			_log.Error(context.Background(), "error when executing shell script to stop USB automount", err)
		}
	} else {
		if _, err := command.OnlyExec("source " + config.AppInfo.ShellPath + "/local-storage-helper.sh ;USB_Start_Auto"); err != nil {
			_log.Error(context.Background(), "error when executing shell script to start USB automount", err)
		}
	}
}

func (s *usbService) GetSysInfo() host.InfoStat {
	info, _ := host.Info()
	return *info
}

func (s *usbService) GetDeviceTree() (string, error) {
	deviceTreeFilePath := "/proc/device-tree/model"

	if _, err := os.Stat(deviceTreeFilePath); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	// read string from deviceTreeFilePath
	deviceTree, err := os.ReadFile(deviceTreeFilePath)
	if err != nil {
		return "", err
	}

	return string(deviceTree), nil
}

// NewUSBService returns a USBService with no startup work — the
// underlying state lives in config and on the host (udev rules).
func NewUSBService() USBService {
	return &usbService{}
}
