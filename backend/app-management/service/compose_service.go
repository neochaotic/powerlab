package service

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/neochaotic/powerlab/backend/app-management/common"
	"github.com/neochaotic/powerlab/backend/app-management/pkg/config"
	"github.com/neochaotic/powerlab/backend/common/utils/file"
	"github.com/neochaotic/powerlab/backend/common/utils/logger"
	portutil "github.com/neochaotic/powerlab/backend/common/utils/port"
	timeutils "github.com/neochaotic/powerlab/backend/common/utils/time"
	"gopkg.in/yaml.v3"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	"github.com/docker/compose/v2/pkg/api"
	"github.com/docker/compose/v2/pkg/compose"
	"github.com/docker/docker/client"

	"go.uber.org/zap"
)

// ComposeService is the orchestration surface around docker-compose
// installs/uninstalls. Tracks in-flight installations in a sync.Map
// so concurrent install requests for the same app are rejected
// instead of double-pulling.
type ComposeService struct {
	installationInProgress sync.Map
}

// PrepareWorkingDirectory ensures the per-app workdir under
// AppsPath exists and returns its path. Each compose app gets a
// dedicated directory holding its rendered docker-compose.yml plus
// any per-instance overrides.
func (s *ComposeService) PrepareWorkingDirectory(name string) (string, error) {
	workingDirectory := filepath.Join(config.AppInfo.AppsPath, name)

	if err := file.IsNotExistMkDir(workingDirectory); err != nil {
		logger.Error("failed to create working dir", zap.Error(err), zap.String("path", workingDirectory))
		return "", err
	}

	return workingDirectory, nil
}

// IsInstalling reports whether an install for appName is currently
// in progress. Used to reject duplicate install requests + drive
// the "spinner" state on the frontend.
func (s *ComposeService) IsInstalling(appName string) bool {
	_, ok := s.installationInProgress.Load(appName)
	return ok
}

// Install brings a ComposeApp up end-to-end: remap volume paths to
// the configured StoragePath (macOS dev installs use /tmp/powerlab-
// data instead of /DATA), rewrite legacy AppData paths to the
// canonical PowerLabAppData tree (ADR-0021), auto-resolve port
// conflicts, render the YAML to disk, and then drive `compose up`.
// Idempotent on retry — the working-directory write + sync.Map
// guard make double-install safe.
func (s *ComposeService) Install(ctx context.Context, composeApp *ComposeApp) error {
	// set store_app_id (by convention is the same as app name at install time if it does not exist)
	_, isStoreApp := composeApp.SetStoreAppID(composeApp.Name)
	if !isStoreApp {
		logger.Info("the compose app getting installed is not a store app, skipping store app id setting.")
	}

	logger.Info("installing compose app", zap.String("name", composeApp.Name))

	// Remap /DATA volume sources to the configured StoragePath.
	// On Linux the default /DATA exists; on macOS it cannot be created (SIP),
	// so StoragePath is set to /tmp/powerlab-data which Docker Desktop shares by default.
	remapVolumePaths(composeApp, config.AppInfo.StoragePath)

	// Per ADR-0021: rewrite <storagePath>/AppData/ → <storagePath>/PowerLabAppData/
	// so newly installed apps bind into the per-product canonical tree.
	// Existing apps' compose files are not rewritten here; they migrate
	// via service.MigrateAppData on the NEXT app-management startup.
	rewriteAppDataPathsToCanonical(composeApp, config.AppInfo.StoragePath)

	// Auto-resolve port conflicts: if any published port is already in use,
	// pick the next available port on the host. The mapping (oldPort → newPort)
	// is also applied to x-casaos.port_map so the "Open UI" button stays correct.
	portRemap := autoRemapPorts(composeApp)
	if len(portRemap) > 0 {
		updateStorePortMap(composeApp, portRemap)
		logger.Info("auto-remapped conflicting ports",
			zap.String("name", composeApp.Name),
			zap.Any("remap", portRemap))
	}

	composeYAMLInterpolated, err := yaml.Marshal(composeApp)
	if err != nil {
		return err
	}

	workingDirectory, err := s.PrepareWorkingDirectory(composeApp.Name)
	if err != nil {
		return err
	}

	yamlFilePath := filepath.Join(workingDirectory, common.ComposeYAMLFileName)

	// Bind-mount perms: create + chmod 0o777 every bind-mount source
	// dir AFTER remapVolumePaths/rewriteAppDataPathsToCanonical have
	// resolved the final on-disk paths. Without this, Laravel/Node/
	// Postgres containers can't write to /DATA/PowerLabAppData/<id>/
	// and crash with "Permission denied" / "Please provide a valid
	// cache path". Sprint 21 PR 10 (#427).
	_ = PrepareBindMountSources(composeYAMLInterpolated)

	// Image-skeleton-seed: when a bind-mount source is empty AND the
	// image ships content at the target path, copy the image content
	// into the source before docker compose up. Closes the "bind-
	// mount overlay" class — Laravel apps need
	// `storage/framework/{cache,views,sessions}` pre-seeded from
	// their image, but an empty bind-mount source overlays the image
	// content and the app crashes. Sprint 22 PR 3 (#428). Best-
	// effort: docker CLI failures are swallowed, install proceeds.
	_ = SeedBindMountsFromImage(composeYAMLInterpolated)

	if err := os.WriteFile(yamlFilePath, composeYAMLInterpolated, 0o600); err != nil {
		logger.Error("failed to save compose file", zap.Error(err), zap.String("path", yamlFilePath))

		if err := file.RMDir(workingDirectory); err != nil {
			logger.Error("failed to cleanup working dir after failing to save compose file", zap.Error(err), zap.String("path", workingDirectory))
		}
		return err
	}

	// load project
	composeApp, err = LoadComposeAppFromConfigFile(composeApp.Name, yamlFilePath)

	if err != nil {
		logger.Error("failed to install compose app", zap.Error(err), zap.String("name", composeApp.Name))
		cleanup(workingDirectory)
		return err
	}

	// prepare for message bus events
	eventProperties := common.PropertiesFromContext(ctx)
	eventProperties[common.PropertyTypeAppName.Name] = composeApp.Name

	if err := composeApp.UpdateEventPropertiesFromStoreInfo(eventProperties); err != nil {
		logger.Info("failed to update event properties from store info", zap.Error(err), zap.String("name", composeApp.Name))
	}

	go func(ctx context.Context) {
		s.installationInProgress.Store(composeApp.Name, true)

		// Create/Get task for logs
		task := MyTaskService.GetOrCreate(composeApp.Name)
		defer func() {
			s.installationInProgress.Delete(composeApp.Name)
			task.Finish()
		}()

		go PublishEventWrapper(ctx, common.EventTypeAppInstallBegin, nil)

		defer PublishEventWrapper(ctx, common.EventTypeAppInstallEnd, nil)

		if err := composeApp.PullAndInstall(ctx, task); err != nil {
			go PublishEventWrapper(ctx, common.EventTypeAppInstallError, map[string]string{
				common.PropertyTypeMessage.Name: err.Error(),
			})

			logger.Error("failed to install compose app", zap.Error(err), zap.String("name", composeApp.Name))
		}
	}(ctx)

	return nil
}

// Uninstall stops + removes the compose app. When deleteConfigFolder
// is true the working directory + bind-mounted AppData are wiped
// too; pass false for upgrade flows that want to swap the compose
// file but keep user data.
func (s *ComposeService) Uninstall(ctx context.Context, composeApp *ComposeApp, deleteConfigFolder bool) error {
	// prepare for message bus events
	eventProperties := common.PropertiesFromContext(ctx)
	eventProperties[common.PropertyTypeAppName.Name] = composeApp.Name

	if err := composeApp.UpdateEventPropertiesFromStoreInfo(eventProperties); err != nil {
		logger.Info("failed to update event properties from store info", zap.Error(err), zap.String("name", composeApp.Name))
	}

	go func(ctx context.Context) {
		go PublishEventWrapper(ctx, common.EventTypeAppUninstallBegin, nil)

		defer PublishEventWrapper(ctx, common.EventTypeAppUninstallEnd, nil)

		if err := composeApp.Uninstall(ctx, deleteConfigFolder); err != nil {
			go PublishEventWrapper(ctx, common.EventTypeAppUninstallError, map[string]string{
				common.PropertyTypeMessage.Name: err.Error(),
			})

			logger.Error("failed to uninstall compose app", zap.Error(err), zap.String("name", composeApp.Name))
		}
	}(ctx)

	return nil
}

// Status returns the lifecycle string ("running"/"exited"/etc.)
// for the compose app identified by appID.
func (s *ComposeService) Status(ctx context.Context, appID string) (string, error) {
	service, dockerClient, err := apiService()
	if err != nil {
		return "", err
	}
	defer dockerClient.Close()

	stackList, err := service.List(ctx, api.ListOptions{
		All: true,
	})
	if err != nil {
		return "", err
	}

	for _, stack := range stackList {
		if stack.ID == appID {
			return stack.Status, nil
		}
	}

	return "", ErrComposeAppNotFound
}

// List returns every compose app currently known to docker, keyed by
// app name. The map's value is the loaded ComposeApp (not the raw
// docker-compose project) so callers can reach into x-extension
// metadata directly.
func (s *ComposeService) List(ctx context.Context) (map[string]*ComposeApp, error) {
	service, dockerClient, err := apiService()
	if err != nil {
		return nil, err
	}
	defer dockerClient.Close()

	stackList, err := service.List(ctx, api.ListOptions{
		All: true,
	})
	if err != nil {
		return nil, err
	}

	result := map[string]*ComposeApp{}

	appsPrefix := config.AppInfo.AppsPath + string(os.PathSeparator)
	for _, stack := range stackList {
		// Use path-separator-aware prefix to avoid matching sibling directories
		// (e.g. "apps-backup/" must not match when appsPath is "apps/").
		if !strings.HasPrefix(stack.ConfigFiles, appsPrefix) {
			continue
		}
		composeApp, err := LoadComposeAppFromConfigFile(stack.ID, stack.ConfigFiles)
		// load project
		if err != nil {
			logger.Error("failed to load compose file", zap.Error(err), zap.String("path", stack.ConfigFiles))
			continue
		}

		result[stack.ID] = composeApp
	}

	return result, nil
}

// NewComposeService returns a fresh ComposeService. No long-running
// state — safe to construct per-request, but the install-progress
// map only deduplicates within a single instance, so the package
// uses one process-wide singleton in practice.
func NewComposeService() *ComposeService {
	return &ComposeService{
		installationInProgress: sync.Map{},
	}
}

func baseInterpolationMap() map[string]string {
	return map[string]string{
		"DefaultUserName": common.DefaultUserName,
		"DefaultPassword": common.DefaultPassword,
		"PUID":            common.DefaultPUID,
		"PGID":            common.DefaultPGID,
		"TZ":              timeutils.GetSystemTimeZoneName(),
	}
}

func apiService() (api.Compose, client.APIClient, error) {
	// Docker SDK v24 caps at API 1.43; daemons ≥25.0 require minimum 1.44.
	// Setting DOCKER_API_VERSION lets the client negotiate above its compile-time cap.
	if os.Getenv("DOCKER_API_VERSION") == "" {
		os.Setenv("DOCKER_API_VERSION", "1.44") //nolint:errcheck
	}

	dockerCli, err := command.NewDockerCli()
	if err != nil {
		return nil, nil, err
	}

	if err := dockerCli.Initialize(&flags.ClientOptions{}); err != nil {
		return nil, nil, err
	}

	dockerCli.Client().NegotiateAPIVersion(context.Background())

	return compose.NewComposeService(dockerCli), dockerCli.Client(), nil
}

// ApiService constructs a fresh docker-compose api.Compose paired
// with the underlying APIClient. Intended for one-shot operations
// (rather than as a long-lived service); callers must Close the
// APIClient when done.
func ApiService() (api.Compose, client.APIClient, error) {
	return apiService()
}

func cleanup(workDir string) {
	logger.Info("cleaning up working dir", zap.String("path", workDir))
	if err := file.RMDir(workDir); err != nil {
		logger.Error("failed to cleanup working dir", zap.Error(err), zap.String("path", workDir))
	}
}

// autoRemapPorts finds the first free host port for any published port that's
// already in use, and updates the compose project in place. Returns a map of
// old → new ports so callers can update store metadata. Only the "host side"
// (Published) is touched — the container target stays the same.
//
// TCP and UDP can independently bind the same host port (different socket types),
// so we track usage per protocol. When the same Published port appears for
// multiple protocols (e.g. AdGuard publishes 53/tcp + 53/udp), the remap is
// applied as a group — they always share the same new port to preserve the
// service convention (DNS etc.).
func autoRemapPorts(app *ComposeApp) map[string]string {
	type portRef struct {
		svcKey string // key into app.Services (compose-go v2 map, keyed by service name)
		portJ  int
		proto  string
	}

	// Group all published ports by their host port string. Each group becomes
	// a single remap decision (so TCP/UDP pairs migrate together).
	groups := map[string][]portRef{}
	for i, svc := range app.Services {
		for j, p := range svc.Ports {
			if p.Published == "" {
				continue
			}
			proto := strings.ToLower(p.Protocol)
			if proto == "" {
				proto = "tcp"
			}
			groups[p.Published] = append(groups[p.Published], portRef{i, j, proto})
		}
	}

	remap := map[string]string{}
	// Track host ports we've claimed within this install, per protocol. A port
	// claimed for tcp does NOT block the same port being claimed for udp.
	used := map[string]bool{} // key: "tcp/531" / "udp/531"

	mark := func(p int, proto string) { used[proto+"/"+strconv.Itoa(p)] = true }
	isUsed := func(p int, proto string) bool { return used[proto+"/"+strconv.Itoa(p)] }

	for oldPub, refs := range groups {
		oldInt, err := strconv.Atoi(oldPub)
		if err != nil {
			continue
		}
		// Determine which protocols this group needs.
		protos := map[string]bool{}
		for _, r := range refs {
			protos[r.proto] = true
		}

		// Group is fine if every protocol it needs is available AND not already
		// claimed by an earlier remap on the same install.
		allFree := true
		for proto := range protos {
			if isUsed(oldInt, proto) || !portutil.IsPortAvailable(oldInt, proto) {
				allFree = false
				break
			}
		}
		if allFree {
			for proto := range protos {
				mark(oldInt, proto)
			}
			continue
		}

		// Need to remap. Find one new port that's free for ALL protocols in the group.
		newPort := findCommonAvailablePort(oldInt, protos, used)
		if newPort == 0 {
			logger.Info("could not find a free host port for protocol group; leaving as-is",
				zap.String("port", oldPub),
				zap.Any("protos", protos))
			continue
		}
		newStr := strconv.Itoa(newPort)
		remap[oldPub] = newStr
		for proto := range protos {
			mark(newPort, proto)
		}
		// Apply to all services in the group
		for _, r := range refs {
			// compose-go v2 Services is a map; mutate a copy and write back.
			// (Ports is a slice, so editing it in the copy is enough, but the
			// write-back keeps the intent explicit.)
			svc := app.Services[r.svcKey]
			svc.Ports[r.portJ].Published = newStr
			app.Services[r.svcKey] = svc
		}
	}
	return remap
}

// findCommonAvailablePort scans up to 1000 ports above `start` for one that is
// available across every protocol the group needs.
func findCommonAvailablePort(start int, protos map[string]bool, used map[string]bool) int {
	for i := 1; i <= 1000; i++ {
		candidate := start + i
		if candidate > 65535 {
			return 0
		}
		ok := true
		for proto := range protos {
			if used[proto+"/"+strconv.Itoa(candidate)] || !portutil.IsPortAvailable(candidate, proto) {
				ok = false
				break
			}
		}
		if ok {
			return candidate
		}
	}
	return 0
}

// updateStorePortMap rewrites the PowerLab/CasaOS extension's port_map / web /
// port fields (if present) to reflect any port remapping so the "Open UI"
// button on the launchpad opens the correct host port. The extension is
// looked up via the translation layer, so it works regardless of which
// alias (x-powerlab, x-web, x-casaos) the YAML actually uses.
func updateStorePortMap(app *ComposeApp, remap map[string]string) {
	if len(remap) == 0 {
		return
	}
	xcMap, _, ok := LookupAppExtensionMap(app.Extensions)
	if !ok {
		return
	}
	for _, key := range []string{"port_map", "web", "port"} {
		if v, ok := xcMap[key]; ok {
			if vs, ok := v.(string); ok {
				if newStr, ok := remap[vs]; ok {
					xcMap[key] = newStr
				}
			} else if vi, ok := v.(int); ok {
				if newStr, ok := remap[strconv.Itoa(vi)]; ok {
					if newInt, err := strconv.Atoi(newStr); err == nil {
						xcMap[key] = newInt
					}
				}
			}
		}
	}
}

// remapVolumePaths replaces the /DATA prefix in all bind-mount source paths with storagePath.
// On Linux storagePath == "/DATA" (no-op). On macOS it points to a Docker Desktop-accessible location.
func remapVolumePaths(app *ComposeApp, storagePath string) {
	if storagePath == "" || storagePath == "/DATA" {
		return
	}
	for name, svc := range app.Services {
		changed := false
		for j, vol := range svc.Volumes {
			if vol.Type == "bind" && strings.HasPrefix(vol.Source, "/DATA") {
				svc.Volumes[j].Source = storagePath + vol.Source[5:]
				changed = true
			}
		}
		if changed {
			app.Services[name] = svc
		}
	}
}

// rewriteAppDataPathsToCanonical rewrites every bind-mount source whose
// path lies under <storagePath>/AppData/ to the canonical
// <storagePath>/PowerLabAppData/ — per ADR-0021. Compose files come
// from upstream stores authored against /DATA/AppData/$AppID/...; this
// rewrite happens at install time so the on-disk compose files
// PowerLab persists already point at the canonical location, and
// `docker compose up` does not silently bind into the legacy tree.
//
// MUST run after remapVolumePaths so the storagePath prefix is already
// substituted; this function works against the post-remap tree.
//
// No-op for paths that don't start with <storagePath>/AppData/ — apps
// using volumes outside the per-app data tree (e.g. Trilium binding
// straight to <storagePath>/$AppID) are not affected here. Future
// migrations of those should be considered explicitly per-app.
func rewriteAppDataPathsToCanonical(app *ComposeApp, storagePath string) {
	if storagePath == "" {
		return
	}
	legacyPrefix := storagePath + "/" + common.LegacyAppDataDirName + "/"
	canonicalPrefix := storagePath + "/" + common.AppDataDirName + "/"
	for name, svc := range app.Services {
		changed := false
		for j, vol := range svc.Volumes {
			if vol.Type == "bind" && strings.HasPrefix(vol.Source, legacyPrefix) {
				svc.Volumes[j].Source = canonicalPrefix + vol.Source[len(legacyPrefix):]
				changed = true
			}
		}
		if changed {
			app.Services[name] = svc
		}
	}
}
