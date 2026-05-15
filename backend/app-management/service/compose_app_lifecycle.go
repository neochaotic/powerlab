package service

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/compose-spec/compose-go/types"
	"github.com/docker/compose/v2/pkg/api"
	dockerTypes "github.com/docker/docker/api/types"
	"github.com/neochaotic/powerlab/backend/app-management/codegen"
	"github.com/neochaotic/powerlab/backend/app-management/common"
	"github.com/neochaotic/powerlab/backend/common/utils/file"
	"github.com/neochaotic/powerlab/backend/common/utils/logger"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// Update is the in-place upgrade flow — pulls the latest image
// tags, regenerates the working directory, and re-runs `compose
// up`. Idempotent: if no images changed, becomes a no-op.
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

// PullAndApply replaces the compose file with newComposeYAML and
// runs the upgrade flow (pull, recreate, restart). Used by the
// "Update available" flow when only image versions changed.
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

// PullAndInstall is the full first-install flow — pull every
// image (streaming progress to logWriter), then bring the project
// up. Replaces Pull + Up for callers that want both phases under
// one ctx.
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

	// Sweep Docker-auto-renamed orphans from prior failed/interrupted
	// installs of this project BEFORE the create phase. Without this,
	// compose-go races the stale orphan during recreate and surfaces
	// "Error response from daemon: No such container: <sha>" — same
	// bug class fixed on the Uninstall side. Symmetric guard.
	// Best-effort: orphan removal failure logs and continues.
	if removed, err := cleanupAutoRenamedOrphans(ctx, dockerClient, a.Name); err != nil {
		logger.Error("pre-install orphan cleanup scan failed; continuing",
			zap.String("project", a.Name),
			zap.Error(err))
	} else if removed > 0 {
		fmt.Fprintf(logWriter, "Pre-install cleanup: removed %d Docker-auto-renamed orphan(s)\n", removed)
		logger.Info("pre-install removed Docker-auto-renamed orphans",
			zap.String("project", a.Name),
			zap.Int("count", removed))
	}

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
				// #334: chown the host source to the container's `user:`
				// field so non-root processes (postgres UID 999, blinko
				// UID 1000) can write to their bind mounts on first init.
				// Docker auto-creates missing source dirs as root:root,
				// which breaks postgres initdb with "Operation not
				// permitted". Silent no-op when user: is unset or
				// non-numeric.
				_ = chownBindMountSource(path, app.User)
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
		// Tolerant fallback for #397: catalog entries that set
		// explicit `container_name:` produce containers that lose
		// the `com.docker.compose.project` label. compose-go's
		// Start() filter then sees zero matches and errors "no
		// container found for project X" even though the
		// containers ARE running. Verify directly with a
		// ContainerList() and a project-name fallback before
		// surfacing the failure to the user.
		all, listErr := dockerClient.ContainerList(ctx, dockerTypes.ContainerListOptions{All: true})
		if listErr == nil && projectHasContainers(all, a.Name) {
			fmt.Fprintf(logWriter, "Note: compose-go reported no labeled containers for project %q; healthy containers detected via name fallback (likely explicit container_name in compose). Original: %v\n", a.Name, err)
		} else {
			fmt.Fprintf(logWriter, "Error during start: %v\n", err)
			go PublishEventWrapper(ctx, common.EventTypeContainerStartError, map[string]string{
				common.PropertyTypeMessage.Name: err.Error(),
			})
			return err
		}
	}

	fmt.Fprintf(logWriter, "Installation completed successfully!\n")
	return nil
}

// Uninstall stops + removes the project. deleteConfigFolder true
// also wipes the working directory + bind-mounted AppData; false
// preserves user data for re-install.
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

	// Catch Docker-auto-renamed orphans that compose-go's
	// RemoveOrphans doesn't see (Sprint 17 C1, .142 incident with
	// 2fauth reinstall). Best-effort — failure here logs and
	// continues; the user-visible Down already succeeded.
	if removed, err := cleanupAutoRenamedOrphans(ctx, dockerClient, a.Name); err != nil {
		logger.Error("orphan cleanup scan failed; continuing",
			zap.String("project", a.Name),
			zap.Error(err))
	} else if removed > 0 {
		logger.Info("removed Docker-auto-renamed orphans",
			zap.String("project", a.Name),
			zap.Int("count", removed))
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

// Apply replaces the compose file with newComposeYAML and recreates
// containers WITHOUT pulling. Used by config-only edits (port map,
// env vars) where image versions haven't changed.
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

// SetStatus is the start/stop/restart dispatcher. Polls for the
// "exited" state before issuing start (lets a previous shutdown
// finish before re-launching). Async: returns before the state
// change completes; observe via the message-bus events.
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
