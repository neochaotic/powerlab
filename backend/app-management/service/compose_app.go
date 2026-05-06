package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	dockerTypes "github.com/docker/docker/api/types"
	v1 "github.com/IceWhaleTech/CasaOS-AppManagement/service/v1"

	"github.com/IceWhaleTech/CasaOS-AppManagement/codegen"
	"github.com/IceWhaleTech/CasaOS-AppManagement/common"
	"github.com/IceWhaleTech/CasaOS-AppManagement/pkg/config"
	"github.com/IceWhaleTech/CasaOS-AppManagement/pkg/docker"
	"github.com/IceWhaleTech/CasaOS-Common/external"
	"github.com/IceWhaleTech/CasaOS-Common/utils"
	"github.com/IceWhaleTech/CasaOS-Common/utils/file"
	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
	"github.com/IceWhaleTech/CasaOS-Common/utils/port"
	"github.com/IceWhaleTech/CasaOS-Common/utils/random"
	"github.com/compose-spec/compose-go/cli"
	"github.com/compose-spec/compose-go/loader"
	"github.com/compose-spec/compose-go/types"
	composeCmd "github.com/docker/compose/v2/cmd/compose"

	"github.com/docker/compose/v2/cmd/formatter"
	"github.com/docker/compose/v2/pkg/api"
	"github.com/go-resty/resty/v2"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type ComposeApp codegen.ComposeApp

func (a *ComposeApp) StoreInfo(includeApps bool) (*codegen.ComposeAppStoreInfo, error) {
	ex, ok := a.getExtension()
	if !ok {
		return nil, ErrComposeExtensionNameXCasaOSNotFound
	}

	var storeInfo codegen.ComposeAppStoreInfo
	if err := loader.Transform(ex, &storeInfo); err != nil {
		logger.Error("Transform store info fail", zap.Error(err))
		return nil, err
	}

	// Aliases: map 'web' or 'port' to 'port_map' if 'port_map' is empty.
	// PowerLab store YAMLs use 'web:' for clarity; CasaOS uses 'port_map:' or 'port:'.
	if storeInfo.PortMap == "" {
		if extMap, ok := ex.(map[string]interface{}); ok {
			for _, key := range []string{"web", "port"} {
				if v, ok := extMap[key].(string); ok && v != "" {
					storeInfo.PortMap = v
					break
				} else if v, ok := extMap[key].(int); ok {
					storeInfo.PortMap = strconv.Itoa(v)
					break
				}
			}
		}
	}

	// TODO refactor this with ComposeAppWithStoreInfo
	if extMap, ok := a.getExtensionMap(); ok {
		if val, ok := extMap[common.ComposeExtensionPropertyNameIsUncontrolled]; ok {
			if isUncontrolled, ok := val.(bool); ok {
				storeInfo.IsUncontrolled = &isUncontrolled
			}
		}
	}

	// locate main app
	if storeInfo.Main == nil || *storeInfo.Main == "" {
		// if main app is not specified, use the first app
		for _, app := range a.Apps() {
			storeInfo.Main = &app.Name
			break
		}
	}

	if storeInfo.Scheme == nil || *storeInfo.Scheme == "" {
		storeInfo.Scheme = lo.ToPtr(codegen.Http)
	}

	if includeApps {
		apps := map[string]codegen.AppStoreInfo{}

		for _, app := range a.Apps() {
			appStoreInfo, err := app.StoreInfo()
			if err != nil {
				if err == ErrComposeExtensionNameXCasaOSNotFound {
					logger.Info("App does not have x-casaos extension - skipping", zap.String("app", app.Name))
					continue
				}

				return nil, err
			}
			apps[app.Name] = appStoreInfo
		}

		storeInfo.Apps = &apps
	}

	return &storeInfo, nil
}

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

// getExtension returns the compose extension regardless of which alias the
// author used (x-powerlab, x-web, or x-casaos). See service/extension.go.
func (a *ComposeApp) getExtension() (interface{}, bool) {
	v, _, ok := LookupAppExtension(a.Extensions)
	return v, ok
}

// getExtensionMap returns the extension as a map regardless of which alias
// the author used. See service/extension.go.
func (a *ComposeApp) getExtensionMap() (map[string]interface{}, bool) {
	m, _, ok := LookupAppExtensionMap(a.Extensions)
	return m, ok
}

func (a *ComposeApp) AuthorType() codegen.StoreAppAuthorType {
	storeInfo, err := a.StoreInfo(false)
	if err != nil {
		return codegen.Unknown
	}

	if strings.EqualFold(storeInfo.Author, storeInfo.Developer) {
		return codegen.Official
	}
	if strings.EqualFold(storeInfo.Author, common.ComposeAppAuthorCasaOSTeam) {
		return codegen.ByCasaos
	}

	return codegen.Community
}

func (a *ComposeApp) SetStoreAppID(storeAppID string) (string, bool) {
	// set store_app_id (by convention is the same as app name at install time if it does not exist)
	composeAppStoreInfo, ok := a.getExtensionMap()
	if !ok {
		logger.Info("compose app does not have a valid extension - might not be a PowerLab app", zap.String("app", a.Name))
		return "", false
	}

	value, ok := composeAppStoreInfo[common.ComposeExtensionPropertyNameStoreAppID]
	if ok {
		currentStoreAppID, ok := value.(string)
		if ok {
			logger.Info("compose app already has store_app_id", zap.String("app", a.Name), zap.String("storeAppID", currentStoreAppID))
			return currentStoreAppID, true
		}
	}

	composeAppStoreInfo[common.ComposeExtensionPropertyNameStoreAppID] = storeAppID
	return storeAppID, true
}

func (a *ComposeApp) SetTitle(title, lang string) {
	if a.Extensions == nil {
		a.Extensions = make(map[string]interface{})
	}

	composeAppStoreInfo, ok := a.getExtensionMap()
	if !ok {
		// Create a new extension using the preferred key
		composeAppStoreInfo = map[string]interface{}{}
		a.Extensions[common.ComposeExtensionNameWeb] = composeAppStoreInfo
	}

	if _, ok := composeAppStoreInfo[common.ComposeExtensionPropertyNameTitle]; !ok {
		composeAppStoreInfo[common.ComposeExtensionPropertyNameTitle] = map[string]string{}
	}

	titleMap, ok := composeAppStoreInfo[common.ComposeExtensionPropertyNameTitle].(map[string]string)
	if !ok {
		logger.Info("compose app does not have valid title map in its extension", zap.String("app", a.Name))
		return
	}

	if _, ok := titleMap[lang]; !ok {
		titleMap[lang] = title
	}
}

func (a *ComposeApp) Update(ctx context.Context) error {
	if len(a.ComposeFiles) <= 0 {
		return ErrComposeFileNotFound
	}

	if len(a.ComposeFiles) > 1 {
		logger.Info("warning: multiple compose files found, only the first one will be used", zap.String("compose files", strings.Join(a.ComposeFiles, ",")))
	}

	storeInfo, err := a.StoreInfo(true)
	if err != nil {
		return err
	}

	if storeInfo == nil || storeInfo.StoreAppID == nil || *storeInfo.StoreAppID == "" {
		return ErrStoreInfoNotFound
	}

	storeComposeApp, err := MyService.AppStoreManagement().ComposeApp(*storeInfo.StoreAppID)
	if err != nil {
		return err
	}

	if storeComposeApp == nil {
		return ErrNotFoundInAppStore
	}

	localComposeAppServices := lo.Map(a.Services, func(service types.ServiceConfig, i int) string { return service.Name })
	storeComposeAppServices := lo.Map(storeComposeApp.Services, func(service types.ServiceConfig, i int) string { return service.Name })

	localAbsentOfStore, storeAbsentOfLocal := lo.Difference(localComposeAppServices, storeComposeAppServices)
	if len(localAbsentOfStore) > 0 {
		logger.Error("local compose app has container apps that are not present in store compose app, thus update is not possible", zap.Strings("absent", localAbsentOfStore))
		return ErrComposeAppNotMatch
	}

	if len(storeAbsentOfLocal) > 0 {
		logger.Error("store compose app has container apps that are not present in local compose app, thus update is not possible", zap.Strings("absent", storeAbsentOfLocal))
		return ErrComposeAppNotMatch
	}

	for _, service := range storeComposeApp.Services {
		localComposeAppService := a.App(service.Name)

		for _, tag := range common.NeedCheckDigestTags {
			if strings.HasSuffix(service.Image, tag) {
				// keep latest
			} else {
				localComposeAppService.Image = service.Image
			}
		}
	}

	// the code is need by stable diffusion.
	removeRuntime(a)

	newComposeYAML, err := yaml.Marshal(a)
	if err != nil {
		return err
	}

	// prepare for message bus events
	eventProperties := common.PropertiesFromContext(ctx)
	eventProperties[common.PropertyTypeAppName.Name] = a.Name

	if err := a.UpdateEventPropertiesFromStoreInfo(eventProperties); err != nil {
		logger.Info("failed to update event properties from store info", zap.Error(err), zap.String("name", a.Name))
	}

	go func(ctx context.Context) {
		go PublishEventWrapper(ctx, common.EventTypeAppUpdateBegin, nil)

		defer PublishEventWrapper(ctx, common.EventTypeAppUpdateEnd, nil)

		MyService.AppStoreManagement().StartUpgrade(a.Name)
		defer MyService.AppStoreManagement().FinishUpgrade(a.Name)

		if err := a.PullAndApply(ctx, newComposeYAML); err != nil {
			go PublishEventWrapper(ctx, common.EventTypeAppUpdateError, map[string]string{
				common.PropertyTypeMessage.Name: err.Error(),
			})

			logger.Error("failed to update compose app", zap.Error(err), zap.String("name", a.Name))
		}
	}(ctx)

	return nil
}

// TODO rename the function to service and add error return value
func (a *ComposeApp) App(name string) *App {
	if name == "" {
		return nil
	}

	for i, service := range a.Services {
		if service.Name == name {
			return (*App)(&a.Services[i])
		}
	}

	return nil
}

func (a *ComposeApp) Apps() map[string]*App {
	apps := make(map[string]*App)

	for i, service := range a.Services {
		apps[service.Name] = (*App)(&a.Services[i])
	}

	return apps
}

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

func (a *ComposeApp) MainTag() (string, error) {
	mainService, err := a.MainService()
	if err != nil {
		return "", err
	}
	_, newTag := docker.ExtractImageAndTag(mainService.Image)

	return newTag, nil
}

func (a *ComposeApp) Containers(ctx context.Context) (map[string][]api.ContainerSummary, error) {
	service, dockerClient, err := apiService()
	if err != nil {
		return nil, err
	}
	defer dockerClient.Close()

	containers, err := service.Ps(ctx, a.Name, api.PsOptions{
		All: true,
	})
	if err != nil {
		return nil, err
	}

	// it is possible a `service` contains multiple containers.
	// See https://docs.docker.com/compose/compose-file/deploy/#replicas
	return lo.GroupBy(containers, func(container api.ContainerSummary) string {
		return container.Service
	}), nil
}

func (a *ComposeApp) Pull(ctx context.Context, logWriter io.Writer) error {
	if logWriter == nil {
		logWriter = io.Discard
	}
	// pull
	serviceNum := len(a.Services)

	for i, app := range a.Services {
		if err := func() error {
			go PublishEventWrapper(ctx, common.EventTypeImagePullBegin, map[string]string{
				common.PropertyTypeImageName.Name: app.Image,
			})

			defer PublishEventWrapper(ctx, common.EventTypeImagePullEnd, map[string]string{
				common.PropertyTypeImageName.Name: app.Image,
			})

			fmt.Fprintf(logWriter, "Pulling %s (%d/%d)...\n", app.Image, i+1, serviceNum)

			if err := docker.PullImage(ctx, app.Image, func(out io.ReadCloser) {
				// We still want the percentage progress in message bus
				// But we also want the raw output in our logWriter
				
				// Create a pipe or just copy?
				// docker.PullImage output is JSON messages.
				// pullImageProgress handles the decoding.
				
				// For the logWriter, we'll write the raw JSON decoded messages for now,
				// or just a "Pulling..." message.
				
				// Actually, let's wrap pullImageProgress to also write to our logWriter.
				pullImageProgress(ctx, out, "INSTALL", serviceNum, i+1, logWriter)
			}); err != nil {
				fmt.Fprintf(logWriter, "Error pulling %s: %v\n", app.Image, err)
				go PublishEventWrapper(ctx, common.EventTypeImagePullError, map[string]string{
					common.PropertyTypeImageName.Name: app.Image,
					common.PropertyTypeMessage.Name:   err.Error(),
				})
			}

			return nil
		}(); err != nil {
			return err
		}
	}

	return nil
}

func (a *ComposeApp) injectEnvVariableToComposeApp() {
	for _, service := range a.Services {
		for k, v := range config.Global {
			// if there is same name var declared in environment in compose yaml
			// we should not reassign a value to it.
			if service.Environment[k] == nil {
				service.Environment[k] = utils.Ptr(v)
			}
		}
	}
}

func (a *ComposeApp) Up(ctx context.Context, service api.Service) error {
	a.injectEnvVariableToComposeApp()

	if err := service.Up(ctx, (*codegen.ComposeApp)(a), api.UpOptions{
		Start: api.StartOptions{
			CascadeStop: true,
			Wait:        true,
		},
	}); err != nil {
		logger.Error("failed to start original compose app", zap.Error(err), zap.String("name", a.Name))
		return err
	}
	return nil
}

func (a *ComposeApp) UpWithCheckRequire(ctx context.Context, service api.Service) error {
	// prepare source path for volumes if not exist
	for i, app := range a.Services {
		for _, volume := range app.Volumes {
			if _, ok := a.Volumes[volume.Source]; ok {
				// this is a internal volume, so skip.
				continue
			}

			path := volume.Source
			if err := file.IsNotExistMkDir(path); err != nil {
				go PublishEventWrapper(ctx, common.EventTypeContainerStartError, map[string]string{
					common.PropertyTypeMessage.Name: err.Error(),
				})
				return err
			}
		}

		// check if each required device exists
		deviceMapFiltered := []string{}
		for _, deviceMap := range app.Devices {
			devicePath := strings.SplitN(deviceMap, ":", 2)[0]
			if file.CheckNotExist(devicePath) {
				logger.Info("device not found", zap.String("device", devicePath))
				continue
			}
			deviceMapFiltered = append(deviceMapFiltered, deviceMap)
		}
		a.Services[i].Devices = deviceMapFiltered
	}

	if err := a.Up(ctx, service); err != nil {
		go PublishEventWrapper(ctx, common.EventTypeContainerStartError, map[string]string{
			common.PropertyTypeMessage.Name: err.Error(),
		})
		return err
	}
	return nil
}

func (a *ComposeApp) PullAndApply(ctx context.Context, newComposeYAML []byte) error {
	// backup current compose file
	currentComposeFile := a.ComposeFiles[0]

	backupComposeFile := currentComposeFile + "." + "bak"
	if err := file.CopySingleFile(currentComposeFile, backupComposeFile, ""); err != nil {
		return err
	}

	// start compose app
	service, dockerClient, err := apiService()
	if err != nil {
		return err
	}
	defer dockerClient.Close()

	success := false

	defer func() {
		if !success {
			if err := file.CopySingleFile(backupComposeFile, currentComposeFile, ""); err != nil {
				logger.Error("failed to restore original compose file", zap.Error(err), zap.String("src", backupComposeFile), zap.String("dst", currentComposeFile))
				return
			}

			if err := a.Up(ctx, service); err != nil {
				logger.Error("failed to start original compose app", zap.Error(err), zap.String("name", a.Name))
				return
			}

		}
	}()

	// save new compose file
	if err := file.WriteToFullPath(newComposeYAML, currentComposeFile, 0o600); err != nil {
		return err
	}

	newComposeApp, err := LoadComposeAppFromConfigFile(a.Name, currentComposeFile)
	if err != nil {
		return err
	}

	if err := newComposeApp.Pull(ctx, nil); err != nil {
		return err
	}

	go PublishEventWrapper(ctx, common.EventTypeContainerStartBegin, nil)

	defer PublishEventWrapper(ctx, common.EventTypeContainerStartEnd, nil)

	err = newComposeApp.UpWithCheckRequire(ctx, service)

	success = true

	return err
}

func (a *ComposeApp) Create(ctx context.Context, options api.CreateOptions, service api.Service) error {
	a.injectEnvVariableToComposeApp()
	return service.Create(ctx, (*codegen.ComposeApp)(a), api.CreateOptions{})
}

func (a *ComposeApp) PullAndInstall(ctx context.Context, logWriter io.Writer) error {
	if logWriter == nil {
		logWriter = io.Discard
	}

	service, dockerClient, err := apiService()
	if err != nil {
		return err
	}
	defer dockerClient.Close()

	fmt.Fprintf(logWriter, "Starting installation of app: %s\n", a.Name)

	// pull
	fmt.Fprintf(logWriter, "Phase 1/3: Pulling images...\n")
	if err := a.Pull(ctx, logWriter); err != nil {
		return err
	}

	// create
	if err := func() error {
		fmt.Fprintf(logWriter, "Phase 2/3: Creating containers...\n")
		go PublishEventWrapper(ctx, common.EventTypeContainerCreateBegin, nil)

		defer PublishEventWrapper(ctx, common.EventTypeContainerCreateEnd, nil)

		for i, app := range a.Services {
			// prepare source path for volumes if not exist
			for _, volume := range app.Volumes {
				if _, ok := a.Volumes[volume.Source]; ok {
					// this is a internal volume, so skip.
					continue
				}

				path := volume.Source
				// Skip mkdir for kernel/system filesystems — these are mount points
				// owned by the OS (e.g. /dev/net/tun for Tailscale). Trying to create
				// them fails with "operation not permitted" on macOS and is unnecessary
				// on Linux because Docker handles them. Let Docker surface its own error
				// if the mount is genuinely unavailable.
				if isSystemPath(path) {
					logger.Info("skipping mkdir for system path; Docker will handle the mount",
						zap.String("path", path))
					continue
				}
				if err := file.IsNotExistMkDir(path); err != nil {
					go PublishEventWrapper(ctx, common.EventTypeContainerCreateError, map[string]string{
						common.PropertyTypeMessage.Name: err.Error(),
					})
					return err
				}
			}

			// check if each required device exists
			deviceMapFiltered := []string{}
			for _, deviceMap := range app.Devices {
				devicePath := strings.SplitN(deviceMap, ":", 2)[0]
				if file.CheckNotExist(devicePath) {
					logger.Info("device not found", zap.String("device", devicePath))
					continue
				}
				deviceMapFiltered = append(deviceMapFiltered, deviceMap)
			}
			a.Services[i].Devices = deviceMapFiltered
		}

		if err := a.Create(ctx, api.CreateOptions{}, service); err != nil {
			go PublishEventWrapper(ctx, common.EventTypeContainerCreateError, map[string]string{
				common.PropertyTypeMessage.Name: err.Error(),
			})
			return err
		}

		return nil
	}(); err != nil {
		fmt.Fprintf(logWriter, "Error during creation: %v\n", err)
		return err
	}

	fmt.Fprintf(logWriter, "Phase 3/3: Starting containers...\n")
	go PublishEventWrapper(ctx, common.EventTypeContainerStartBegin, nil)

	defer PublishEventWrapper(ctx, common.EventTypeContainerStartEnd, nil)

	if err := service.Start(ctx, a.Name, api.StartOptions{
		CascadeStop: true,
		Wait:        true,
	}); err != nil {
		fmt.Fprintf(logWriter, "Error during start: %v\n", err)
		go PublishEventWrapper(ctx, common.EventTypeContainerStartError, map[string]string{
			common.PropertyTypeMessage.Name: err.Error(),
		})
		return err
	}

	fmt.Fprintf(logWriter, "Installation completed successfully!\n")
	return nil
}

func (a *ComposeApp) Uninstall(ctx context.Context, deleteConfigFolder bool) error {
	service, dockerClient, err := apiService()
	if err != nil {
		return err
	}
	defer dockerClient.Close()

	// stop
	if err := func() error {
		go PublishEventWrapper(ctx, common.EventTypeContainerStopBegin, nil)

		defer PublishEventWrapper(ctx, common.EventTypeContainerStopEnd, nil)

		if err := service.Stop(ctx, a.Name, api.StopOptions{}); err != nil {
			go PublishEventWrapper(ctx, common.EventTypeContainerStopError, map[string]string{
				common.PropertyTypeMessage.Name: err.Error(),
			})

			return err
		}

		return nil
	}(); err != nil {
		return err
	}

	// remove
	go PublishEventWrapper(ctx, common.EventTypeContainerRemoveBegin, nil)

	defer PublishEventWrapper(ctx, common.EventTypeContainerRemoveEnd, nil)

	if err := service.Down(ctx, a.Name, api.DownOptions{
		RemoveOrphans: true,
		Images:        "all",
		Volumes:       true,
	}); err != nil {
		go PublishEventWrapper(ctx, common.EventTypeImageRemoveError, map[string]string{
			common.PropertyTypeMessage.Name: err.Error(),
		})

		return err
	}

	if err := file.RMDir(a.WorkingDir); err != nil {
		go PublishEventWrapper(ctx, common.EventTypeImageRemoveError, map[string]string{
			common.PropertyTypeMessage.Name: err.Error(),
		})
	}

	if !deleteConfigFolder {
		return nil
	}

	for _, app := range a.Services {
		for _, volume := range app.Volumes {
			if strings.Contains(volume.Source, a.Name) {
				path := filepath.Join(strings.Split(volume.Source, a.Name)[0], a.Name)
				if err := file.RMDir(path); err != nil {
					logger.Error("failed to remove compose app config folder", zap.Error(err), zap.String("path", path))

					go PublishEventWrapper(ctx, common.EventTypeImageRemoveError, map[string]string{
						common.PropertyTypeMessage.Name: err.Error(),
					})
				}
			}
		}
	}

	return nil
}

func (a *ComposeApp) Apply(ctx context.Context, newComposeYAML []byte) error {
	// compare new ComposeApp with current ComposeApp
	if getNameFrom(newComposeYAML) != a.Name {
		return ErrComposeAppNotMatch
	}

	newComposeApp, err := NewComposeAppFromYAML(newComposeYAML, true, true)
	if err != nil {
		return err
	}

	if len(a.ComposeFiles) <= 0 {
		return ErrComposeFileNotFound
	}

	if len(a.ComposeFiles) > 1 {
		logger.Info("warning: multiple compose files found, only the first one will be used", zap.String("compose files", strings.Join(a.ComposeFiles, ",")))
	}

	// prepare for message bus events
	eventProperties := common.PropertiesFromContext(ctx)
	eventProperties[common.PropertyTypeAppName.Name] = a.Name

	// prepare for message bus events
	if err := newComposeApp.UpdateEventPropertiesFromStoreInfo(eventProperties); err != nil {
		logger.Info("failed to update event properties from store info", zap.Error(err), zap.String("name", a.Name))
	}

	go func(ctx context.Context) {
		go PublishEventWrapper(ctx, common.EventTypeAppApplyChangesBegin, nil)

		defer PublishEventWrapper(ctx, common.EventTypeAppApplyChangesEnd, nil)

		if err := a.PullAndApply(ctx, newComposeYAML); err != nil {
			go PublishEventWrapper(ctx, common.EventTypeAppApplyChangesError, map[string]string{
				common.PropertyTypeMessage.Name: err.Error(),
			})

			logger.Error("failed to apply changes to compose app", zap.Error(err), zap.String("name", a.Name))
		}
	}(ctx)

	return nil
}

func (a *ComposeApp) SetStatus(ctx context.Context, status codegen.RequestComposeAppStatus) error {
	service, dockerClient, err := apiService()
	if err != nil {
		return err
	}
	defer dockerClient.Close()

	eventProperties := common.PropertiesFromContext(ctx)
	eventProperties[common.PropertyTypeAppName.Name] = a.Name

	switch status {
	case codegen.RequestComposeAppStatusStart:
		go func(ctx context.Context) {
			go PublishEventWrapper(ctx, common.EventTypeAppStartBegin, nil)

			defer PublishEventWrapper(ctx, common.EventTypeAppStartEnd, nil)

			// to make sure the container is stopped
			// timeout is 20s
			for index := 0; index < 10; index++ {
				containerSummarys, err := service.Ps(ctx, a.Name, api.PsOptions{
					All: true,
				})
				if err != nil {
					logger.Error("failed to get compose app info", zap.Error(err), zap.String("name", a.Name))
				}
				isContainerExited := true
				for _, containerSummary := range containerSummarys {
					// to make sure every service of the container is stopped
					// I think "exited" can be replace by constant value.
					isContainerExited = isContainerExited && (containerSummary.State == "exited")
				}
				if isContainerExited {
					break
				}
				time.Sleep(2 * time.Second)
			}

			if err := service.Start(ctx, a.Name, api.StartOptions{
				CascadeStop: true,
				Wait:        true,
			}); err != nil {
				go PublishEventWrapper(ctx, common.EventTypeAppStartError, map[string]string{
					common.PropertyTypeMessage.Name: err.Error(),
				})

				logger.Error("failed to start compose app", zap.Error(err), zap.String("name", a.Name))
			}
		}(ctx)
	case codegen.RequestComposeAppStatusStop:
		go func(ctx context.Context) {
			go PublishEventWrapper(ctx, common.EventTypeAppStopBegin, nil)

			defer PublishEventWrapper(ctx, common.EventTypeAppStopEnd, nil)

			if err := service.Stop(ctx, a.Name, api.StopOptions{}); err != nil {
				go PublishEventWrapper(ctx, common.EventTypeAppStopError, map[string]string{
					common.PropertyTypeMessage.Name: err.Error(),
				})

				logger.Error("failed to stop compose app", zap.Error(err), zap.String("name", a.Name))
			}
		}(ctx)
	case codegen.RequestComposeAppStatusRestart:
		go func(ctx context.Context) {
			go PublishEventWrapper(ctx, common.EventTypeAppRestartBegin, nil)

			defer PublishEventWrapper(ctx, common.EventTypeAppRestartEnd, nil)

			if err := service.Restart(ctx, a.Name, api.RestartOptions{}); err != nil {
				go PublishEventWrapper(ctx, common.EventTypeAppRestartError, map[string]string{
					common.PropertyTypeMessage.Name: err.Error(),
				})

				logger.Error("failed to restart compose app", zap.Error(err), zap.String("name", a.Name))
			}
		}(ctx)
	default:
		return ErrInvalidComposeAppStatus
	}

	return nil
}

func (a *ComposeApp) Logs(ctx context.Context, lines int) ([]byte, error) {
	service, dockerClient, err := apiService()
	if err != nil {
		return nil, err
	}
	defer dockerClient.Close()

	var buf bytes.Buffer

	consumer := formatter.NewLogConsumer(ctx, &buf, &buf, false, true, false)

	if err := service.Logs(ctx, a.Name, consumer, api.LogOptions{
		Project:  (*codegen.ComposeApp)(a),
		Services: lo.Map(a.Services, func(s types.ServiceConfig, i int) string { return s.Name }),
		Follow:   false,
		Tail:     lo.If(lines < 0, "all").Else(strconv.Itoa(lines)),
	}); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

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

// Try to update AppIcon and AppTitle in given event properties from store info
func (a *ComposeApp) UpdateEventPropertiesFromStoreInfo(eventProperties map[string]string) error {
	if eventProperties == nil {
		return fmt.Errorf("event properties is nil")
	}

	storeInfo, err := a.StoreInfo(false)
	if err != nil {
		return err
	}

	eventProperties[common.PropertyTypeAppIcon.Name] = storeInfo.Icon

	if storeInfo.Title == nil {
		return fmt.Errorf("compose app title not found in store info")
	}

	titles, err := json.Marshal(storeInfo.Title)
	if err != nil {
		return err
	}

	eventProperties[common.PropertyTypeAppTitle.Name] = string(titles)

	return nil
}

func (a *ComposeApp) HealthCheck() (bool, error) {
	storeInfo, err := a.StoreInfo(false)
	if err != nil {
		return false, err
	}

	scheme := "http"
	if storeInfo.Scheme != nil && *storeInfo.Scheme != "" {
		scheme = string(*storeInfo.Scheme)
	}

	hostname := common.Localhost
	if storeInfo.Hostname != nil && *storeInfo.Hostname != "" {
		hostname = *storeInfo.Hostname
	}

	url := fmt.Sprintf(
		"%s://%s:%s/%s",
		scheme,
		hostname,
		storeInfo.PortMap,
		strings.TrimLeft(storeInfo.Index, "/"),
	)

	logger.Info("checking compose app health at the specified web port...", zap.String("name", a.Name), zap.Any("url", url))

	client := resty.New()
	client.SetTimeout(30 * time.Second)
	client.SetHeader("Accept", "text/html")
	// ignore ssl error
	client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	response, err := client.R().Get(url)
	if err != nil {
		logger.Error("failed to check container health", zap.Error(err), zap.String("name", a.Name))
		return false, err
	}
	if response.StatusCode() == http.StatusOK || response.StatusCode() == http.StatusUnauthorized {
		return true, nil
	}

	logger.Error("compose app health check failed at the specified web port", zap.Any("name", a.Name), zap.Any("url", url), zap.String("status", fmt.Sprint(response.StatusCode())))
	return false, nil
}

func LoadComposeAppFromConfigFile(appID string, configFile string) (*ComposeApp, error) {
	// Support 'web:' alias for 'x-web:' by writing a temp copy if needed
	yamlContent, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`(?m)^web:`)
	if re.Match(yamlContent) {
		transformed := re.ReplaceAll(yamlContent, []byte("x-web:"))
		if err := os.WriteFile(configFile, transformed, 0o600); err != nil {
			logger.Error("failed to write transformed compose file", zap.Error(err))
			// continue with original file
		}
	}

	options := composeCmd.ProjectOptions{
		ProjectDir:  filepath.Dir(configFile),
		ProjectName: appID,
	}

	env := []string{fmt.Sprintf("%s=%s", "AppID", appID)}
	for k, v := range baseInterpolationMap() {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// load project
	project, err := options.ToProject(
		nil,
		nil,
		cli.WithWorkingDirectory(options.ProjectDir), // this has to be the first option, otherwise it will assume the dir where this program is running is the working directory.

		cli.WithOsEnv,
		cli.WithDotEnv,
		cli.WithEnv(env),
		cli.WithConfigFileEnv,
		cli.WithDefaultConfigPath,
		cli.WithEnvFiles(options.EnvFiles...),
		cli.WithName(options.ProjectName),
	)

	return (*ComposeApp)(project), err
}

// isSystemPath reports whether a bind-mount source path lives in a kernel/system
// filesystem that the OS owns. We must not try to mkdir these (they are mount
// points like /dev/net/tun or /proc/* which exist by virtue of the kernel).
func isSystemPath(p string) bool {
	systemRoots := []string{"/dev", "/proc", "/sys", "/run", "/host"}
	for _, root := range systemRoots {
		if p == root || strings.HasPrefix(p, root+"/") {
			return true
		}
	}
	return false
}

var gpuCache *([]external.GPUInfo) = nil

func removeRuntime(a *ComposeApp) {
	if config.RemoveRuntimeIfNoNvidiaGPUFlag {

		// if gpuCache is nil, it means it is first time fetching gpu info
		if gpuCache == nil {
			value, err := external.GPUInfoList()
			if err != nil {
				gpuCache = &([]external.GPUInfo{})
			} else {
				gpuCache = &value
			}

			// without nvidia-smi 	// no gpu or first time fetching gpu info failed
		}
		if len(*gpuCache) == 0 {
			for i := range a.Services {
				a.Services[i].Runtime = ""
			}
		}
	}
}

func NewComposeAppFromYAML(yaml []byte, skipInterpolation, skipValidation bool) (*ComposeApp, error) {
	tmpWorkingDir, err := os.MkdirTemp("", "casaos-compose-app-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpWorkingDir)

	// the WEBUI_PORT interpolate will tiger twice. In `pulished` and `port-map`.
	// So we need to promise multiple WEBUI_PORT interpolate is a same value.
	port, _ := port.GetAvailablePort("tcp")

	// Support 'web:' as an alias for 'x-web:'
	re := regexp.MustCompile(`(?m)^web:`)
	yaml = re.ReplaceAll(yaml, []byte("x-web:"))

	project, err := loader.Load(
		types.ConfigDetails{
			ConfigFiles: []types.ConfigFile{
				{
					Content: []byte(yaml),
				},
			},
			Environment: map[string]string{},

			// need to set a working dir because loader/normalize.go from github.com/compose-spec/compose-go makes
			// wrong assumption that the working dir is the same as the dir where this program is launched.
			WorkingDir: tmpWorkingDir,
		},
		func(o *loader.Options) {
			o.SkipInterpolation = skipInterpolation
			o.SkipValidation = skipValidation

			o.Interpolate.LookupValue = func(key string) (string, bool) {
				switch key {
				case "WEBUI_PORT":
					fmt.Printf("WEBUI_PORT is not specified, using %d\n", port)
					return strconv.Itoa(port), true
				}

				for k := range baseInterpolationMap() {
					if k == key {
						// example:  TZ => $TZ
						// we didn't want to interpolate base interpolation value.
						// they should be interpolated in LoadComposeAppFromConfig
						return fmt.Sprintf("$%s", k), true
					}
				}
				// the function may can to replace the above code.
				value, ok := os.LookupEnv(key)
				if ok {
					return value, true
				} else {
					return fmt.Sprintf("$%s", key), true
				}
			}

			if getNameFrom(yaml) != "" {
				return
			}

			// fix compose app name
			projectName := random.Name(nil)
			// Sanitize project name to be Docker-compliant: lowercase, alphanumeric, hyphens, and underscores
			projectName = strings.ToLower(projectName)
			reg := regexp.MustCompile(`[^a-z0-9_-]`)
			projectName = reg.ReplaceAllString(projectName, "")
			
			o.SetProjectName(projectName, false)
		},
	)
	if err != nil {
		return nil, err
	}

	composeApp := (*ComposeApp)(project)

	if composeApp.Extensions == nil {
		composeApp.Extensions = map[string]interface{}{}
	}

	storeInfo, err := composeApp.StoreInfo(false)

	if err != nil || storeInfo == nil || storeInfo.Title == nil {
		logger.Info("compose app does not have store info with title set, re-using app name as title", zap.String("app", composeApp.Name))
		composeApp.SetTitle(composeApp.Name, common.DefaultLanguage)
	}

	removeRuntime(composeApp)

	// pass icon information to v1 label for backward compatibility, because we are
	// still using `func getContainerStats()` from `container.go` to get container stats
	// (we are being lazy to upgrade that v1 API to v2 - please help if you can :D)
	if err == nil && storeInfo != nil && storeInfo.Icon != "" {
		for i := range composeApp.Services {
			if composeApp.Services[i].Labels == nil {
				composeApp.Services[i].Labels = map[string]string{}
			}
			composeApp.Services[i].Labels[v1.V1LabelIcon] = storeInfo.Icon
		}
	}

	return composeApp, nil
}

func getNameFrom(composeYAML []byte) string {
	var baseStructure struct {
		Name string `yaml:"name"`
	}

	if err := yaml.Unmarshal(composeYAML, &baseStructure); err != nil {
		return ""
	}

	return baseStructure.Name
}

func (a *ComposeApp) SetUncontrolled(uncontrolled bool) error {
	extMap, ok := a.getExtensionMap()
	if !ok {
		logger.Error("failed to get extension map", zap.String("composeAppID", a.Name))
		return ErrComposeExtensionNameXCasaOSNotFound
	}

	extMap[common.ComposeExtensionPropertyNameIsUncontrolled] = uncontrolled
	return nil
}

func (a *ComposeApp) Stats(ctx context.Context) (*codegen.ComposeAppStats, error) {
	_, dockerClient, err := apiService()
	if err != nil {
		return nil, err
	}
	defer dockerClient.Close()

	containers, err := a.Containers(ctx)
	if err != nil {
		return nil, err
	}

	var totalCPU float64
	var totalMemUsed int64
	var totalMemLimit int64
	var totalNetRx int64
	var totalNetTx int64

	for _, containerList := range containers {
		for _, c := range containerList {
			if c.State != "running" {
				continue
			}

			stats, err := dockerClient.ContainerStatsOneShot(ctx, c.ID)
			if err != nil {
				continue
			}

			var v dockerTypes.StatsJSON
			if err := json.NewDecoder(stats.Body).Decode(&v); err != nil {
				stats.Body.Close()
				continue
			}
			stats.Body.Close()

			// CPU %
			cpuDelta := float64(v.CPUStats.CPUUsage.TotalUsage) - float64(v.PreCPUStats.CPUUsage.TotalUsage)
			systemDelta := float64(v.CPUStats.SystemUsage) - float64(v.PreCPUStats.SystemUsage)
			onlineCPUs := float64(v.CPUStats.OnlineCPUs)
			if onlineCPUs == 0.0 {
				onlineCPUs = float64(len(v.CPUStats.CPUUsage.PercpuUsage))
			}
			if systemDelta > 0.0 && cpuDelta > 0.0 {
				totalCPU += (cpuDelta / systemDelta) * onlineCPUs * 100.0
			}

			// Memory
			// For memory usage, we use (usage - cache) to match Docker CLI behavior
			cache := v.MemoryStats.Stats["inactive_file"]
			if cache == 0 {
				cache = v.MemoryStats.Stats["cache"]
			}
			totalMemUsed += int64(v.MemoryStats.Usage - cache)
			totalMemLimit += int64(v.MemoryStats.Limit)

			// Network
			for _, n := range v.Networks {
				totalNetRx += int64(n.RxBytes)
				totalNetTx += int64(n.TxBytes)
			}
		}
	}

	return &codegen.ComposeAppStats{
		CPUPercent:       totalCPU,
		MemoryUsedBytes:  totalMemUsed,
		MemoryLimitBytes: totalMemLimit,
		NetRxBytes:       totalNetRx,
		NetTxBytes:       totalNetTx,
	}, nil
}
