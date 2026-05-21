package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/compose-spec/compose-go/cli"
	"github.com/compose-spec/compose-go/loader"
	"github.com/compose-spec/compose-go/types"
	composeCmd "github.com/docker/compose/v2/cmd/compose"
	"github.com/neochaotic/powerlab/backend/app-management/codegen"
	"github.com/neochaotic/powerlab/backend/app-management/common"
	"github.com/neochaotic/powerlab/backend/app-management/pkg/config"
	v1 "github.com/neochaotic/powerlab/backend/app-management/service/v1"
	"github.com/neochaotic/powerlab/backend/common/external"
	"github.com/neochaotic/powerlab/backend/common/utils"
	"github.com/neochaotic/powerlab/backend/common/utils/logger"
	"github.com/neochaotic/powerlab/backend/common/utils/port"
	"github.com/neochaotic/powerlab/backend/common/utils/random"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// ComposeApp is the in-process view of a docker-compose project +
// the PowerLab x-extension metadata that wraps it. Type-aliased to
// codegen.ComposeApp so the OpenAPI codegen output reuses the same
// underlying shape.
//
// Method set is split across companion files (Sprint 7 #227 split):
//   - compose_app.go (this file): type declaration, factories
//     (LoadComposeAppFromConfigFile, NewComposeAppFromYAML), and
//     private package-level helpers
//   - compose_app_metadata.go: x-extension read/write
//     (StoreInfo, AuthorType, SetStoreAppID, SetTitle,
//     SetUncontrolled, UpdateEventPropertiesFromStoreInfo)
//   - compose_app_lifecycle.go: mutation surface
//     (Update, PullAndApply, PullAndInstall, Apply, Uninstall,
//     SetStatus)
//   - compose_app_runtime.go: docker engine surface
//     (Up, UpWithCheckRequire, Create, Pull, Containers, Logs,
//     HealthCheck, Stats)
//   - compose_app_query.go: read-only helpers
//     (App, Apps, MainService, MainTag, DiskUsage,
//     GetPortsInUse)
type ComposeApp codegen.ComposeApp

// injectEnvVariableToComposeApp merges config.Global env vars into
// each service's Environment map. Vars already set in the compose
// file win — the global is a fallback, not an override.
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

// LoadComposeAppFromConfigFile loads a compose app from an existing
// docker-compose.yml on disk. Used by the install path after the
// YAML has been written to the per-app working directory + by the
// upgrade path when the file is being re-read.
//
// Pre-processes the file to support PowerLab's `web:` alias for
// `x-web:` (writes the transformed bytes back to disk).
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
		// Local-only: our compose files never reference remote git/OCI
		// includes, so disable remote resource loaders. This also lets
		// ToProject run without a docker CLI instance.
		Offline: true,
	}

	env := []string{fmt.Sprintf("%s=%s", "AppID", appID)}
	for k, v := range baseInterpolationMap() {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// load project
	project, _, err := options.ToProject(
		context.Background(),
		nil, // dockerCli: unused because Offline disables remote loaders
		nil, // services: load all
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

// gpuCache memoizes the result of nvidia-smi at process start so
// removeRuntime doesn't reshell on every install.
var gpuCache *([]external.GPUInfo) = nil

// removeRuntime strips the nvidia runtime declaration from each
// service when no NVIDIA GPU is detected on the host. Required
// because compose-go refuses to start a project that asks for the
// nvidia runtime when the runtime isn't installed.
//
// Gated on config.RemoveRuntimeIfNoNvidiaGPUFlag — flip to false
// when shipping on a host that always has nvidia-container-toolkit.
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

// NewComposeAppFromYAML parses raw compose-YAML bytes into a
// ComposeApp. Used by the V2 install-from-YAML endpoint and the
// Apply (config-only update) path. Auto-generates a Docker-safe
// project name when the YAML doesn't declare one.
//
// Supports PowerLab's `web:` alias for `x-web:`. Interpolates
// $WEBUI_PORT to a freshly-allocated free port so apps that bind a
// random web port don't collide.
func NewComposeAppFromYAML(yaml []byte, skipInterpolation, skipValidation bool) (*ComposeApp, error) {
	tmpWorkingDir, err := os.MkdirTemp("", "powerlab-compose-app-*")
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

// getNameFrom extracts the top-level `name:` field from compose-
// YAML bytes. Returns "" when the field is absent or the YAML is
// malformed.
func getNameFrom(composeYAML []byte) string {
	var baseStructure struct {
		Name string `yaml:"name"`
	}

	if err := yaml.Unmarshal(composeYAML, &baseStructure); err != nil {
		return ""
	}

	return baseStructure.Name
}
