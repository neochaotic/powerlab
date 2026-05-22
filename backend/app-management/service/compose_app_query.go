package service

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/neochaotic/powerlab/backend/app-management/codegen"
	"github.com/neochaotic/powerlab/backend/app-management/pkg/config"
	"github.com/neochaotic/powerlab/backend/app-management/pkg/docker"
	"github.com/neochaotic/powerlab/backend/common/utils/port"
)

// DiskUsage returns the on-disk size in bytes of this app's AppData directory.
// Returns 0 (no error) when the directory doesn't exist yet — apps that
// installed but haven't written any files have no usage to report.
//
// Implementation walks the directory in pure Go so it works identically on
// Linux and macOS (no shelling out to `du`). Symlinks are not followed and
// per-file permission errors are silently skipped.
func (a *ComposeApp) DiskUsage() (int64, error) {
	appDataPath := filepath.Join(config.AppInfo.StoragePath, "AppData", a.Name)

	if _, err := os.Stat(appDataPath); os.IsNotExist(err) {
		return 0, nil
	}

	var size int64
	err := filepath.Walk(appDataPath, func(_ string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			if os.IsPermission(walkErr) {
				return nil // skip unreadable files, don't fail the whole walk
			}
			return walkErr
		}
		if info.Mode().IsRegular() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// App returns the named service from the compose project, or nil
// if no such service exists.
//
// TODO rename the function to service and add error return value
func (a *ComposeApp) App(name string) *App {
	if name == "" {
		return nil
	}

	for _, service := range a.Services {
		if service.Name == name {
			// compose-go v2 Services is a map; values aren't addressable.
			// service is a per-iteration copy (Go 1.22+), safe to return.
			return (*App)(&service)
		}
	}

	return nil
}

// Apps returns every service in the compose project, keyed by
// service name.
func (a *ComposeApp) Apps() map[string]*App {
	apps := make(map[string]*App)

	for _, service := range a.Services {
		apps[service.Name] = (*App)(&service)
	}

	return apps
}

// MainService returns the service flagged as the project's "main"
// app via the x-extension main: field. Returns
// ErrMainAppNotFound if no main app is declared and the project
// has multiple services.
func (a *ComposeApp) MainService() (*App, error) {
	storeInfo, err := a.StoreInfo(false)
	if err != nil {
		return nil, err
	}

	if storeInfo.Main == nil || *storeInfo.Main == "" {
		return nil, ErrMainServiceNotSpecified
	}

	return a.App(*storeInfo.Main), nil
}

// MainTag returns the image tag of the main service ("latest"
// when unset). Used by the update-checker to decide whether a
// pull is worth doing.
func (a *ComposeApp) MainTag() (string, error) {
	mainService, err := a.MainService()
	if err != nil {
		return "", err
	}
	_, newTag := docker.ExtractImageAndTag(mainService.Image)

	return newTag, nil
}

// GetPortsInUse returns the list of declared host ports that are
// already bound on this host. Cross-platform — tries to bind each
// port directly so it works on macOS (no /proc/net/tcp), Linux,
// and Windows alike. Returns (nil, nil) when nothing conflicts.
func (a *ComposeApp) GetPortsInUse() (*codegen.ComposeAppValidationErrorsPortsInUse, error) {
	// Cross-platform port detection: try to bind each declared port directly.
	// This works on macOS (where /proc/net/tcp doesn't exist), Linux, and Windows.
	tcpPortInUse := []string{}
	udpPortInUse := []string{}

	for _, s := range a.Services {
		for _, p := range s.Ports {
			if p.Published == "" {
				continue
			}
			pubInt, err := strconv.Atoi(p.Published)
			if err != nil {
				continue
			}
			proto := strings.ToLower(p.Protocol)
			if proto == "" {
				proto = "tcp"
			}
			if !port.IsPortAvailable(pubInt, proto) {
				switch proto {
				case "tcp":
					tcpPortInUse = append(tcpPortInUse, p.Published)
				case "udp":
					udpPortInUse = append(udpPortInUse, p.Published)
				}
			}
		}
	}

	if len(tcpPortInUse) == 0 && len(udpPortInUse) == 0 {
		return nil, nil
	}

	portsInUse := struct {
		TCP *codegen.PortList "json:\"TCP,omitempty\""
		UDP *codegen.PortList "json:\"UDP,omitempty\""
	}{TCP: &tcpPortInUse, UDP: &udpPortInUse}

	return &codegen.ComposeAppValidationErrorsPortsInUse{PortsInUse: &portsInUse}, nil
}
