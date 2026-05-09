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

type ComposeService struct {
	installationInProgress sync.Map
}

func (s *ComposeService) PrepareWorkingDirectory(name string) (string, error) {
	workingDirectory := filepath.Join(config.AppInfo.AppsPath, name)

	if err := file.IsNotExistMkDir(workingDirectory); err != nil {
		logger.Error("failed to create working dir", zap.Error(err), zap.String("path", workingDirectory))
		return "", err
	}

	return workingDirectory, nil
}

func (s *ComposeService) IsInstalling(appName string) bool {
	_, ok := s.installationInProgress.Load(appName)
	return ok
}

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

func apiService() (api.Service, client.APIClient, error) {
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

func ApiService() (api.Service, client.APIClient, error) {
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
		svcIdx int // index into app.Services (it's a slice, not a map)
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
			app.Services[r.svcIdx].Ports[r.portJ].Published = newStr
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
