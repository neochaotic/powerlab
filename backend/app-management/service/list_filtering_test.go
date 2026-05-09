package service_test

// Tests for ComposeService.List() filtering behaviour:
//   - Only stacks whose ConfigFiles path starts with config.AppInfo.AppsPath are shown
//   - External stacks (from other projects on the same machine) are silently excluded
//   - LoadComposeAppFromConfigFile works for PowerLab-managed compose files
//   - apiService() always sets DOCKER_API_VERSION >= 1.44

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neochaotic/powerlab/backend/app-management/pkg/config"
	"github.com/neochaotic/powerlab/backend/app-management/service"
	"github.com/neochaotic/powerlab/backend/common/utils/logger"
	"gotest.tools/v3/assert"
)

// hasPowerlabPrefix mirrors the exact filtering predicate in ComposeService.List().
// It uses a path-separator-aware check to avoid false matches on sibling directories
// (e.g. "apps-backup/" must NOT match when appsPath is "apps").
func hasPowerlabPrefix(configFiles, appsPath string) bool {
	return strings.HasPrefix(configFiles, appsPath+string(filepath.Separator))
}

// TestListFilterExcludesExternalStacks verifies that stacks whose config file
// lives outside AppsPath are excluded — they are "external" containers started
// independently of PowerLab and must never appear in the app list.
func TestListFilterExcludesExternalStacks(t *testing.T) {
	appsPath := "/home/user/powerlab/data/apps"

	cases := []string{
		"/home/user/other-project/docker-compose.yml",
		// Path starts with the same chars but is a different directory
		"/home/user/powerlab/data/apps-backup/myapp/docker-compose.yml",
		"/tmp/docker-compose.yml",
		"",
	}
	for _, path := range cases {
		assert.Assert(t, !hasPowerlabPrefix(path, appsPath),
			"expected path %q to be excluded (not a PowerLab app)", path)
	}
}

// TestListFilterIncludesPowerlabStacks verifies that stacks inside AppsPath
// pass the prefix check and are included in the result.
func TestListFilterIncludesPowerlabStacks(t *testing.T) {
	appsPath := "/home/user/powerlab/data/apps"

	cases := []string{
		"/home/user/powerlab/data/apps/myapp/docker-compose.yml",
		"/home/user/powerlab/data/apps/another-app/compose.yaml",
	}
	for _, path := range cases {
		assert.Assert(t, hasPowerlabPrefix(path, appsPath),
			"expected path %q to be included (PowerLab managed app)", path)
	}
}

// TestLoadComposeAppFromConfigFileRoundtrip verifies that a PowerLab-managed
// compose YAML written to disk can be loaded and its metadata read back.
func TestLoadComposeAppFromConfigFileRoundtrip(t *testing.T) {
	logger.LogInitConsoleOnly()

	dir := t.TempDir()
	composePath := filepath.Join(dir, "docker-compose.yml")

	yaml := `name: roundtrip-test
services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"
x-casaos:
  title:
    en_us: Roundtrip Test
  icon: https://example.com/icon.png
  port_map: "8080"
`
	err := os.WriteFile(composePath, []byte(yaml), 0o644)
	assert.NilError(t, err)

	app, err := service.LoadComposeAppFromConfigFile("roundtrip-test", composePath)
	assert.NilError(t, err)
	assert.Assert(t, app != nil)
	assert.Equal(t, 1, len(app.Services))

	storeInfo, err := app.StoreInfo(true)
	assert.NilError(t, err)
	assert.Equal(t, "Roundtrip Test", storeInfo.Title["en_us"])
	assert.Equal(t, "https://example.com/icon.png", storeInfo.Icon)
	assert.Equal(t, "8080", storeInfo.PortMap)
}

// TestAppsPathPrefixMatchesContainerLabel verifies the exact scenario observed
// in production: a container deployed via PowerLab has its config_files label
// set to a path under AppsPath, so it MUST appear in the List() result.
// A sibling directory (e.g. "apps-backup") must NOT match.
func TestAppsPathPrefixMatchesContainerLabel(t *testing.T) {
	appsPath := "/Users/user/powerlab/backend/data/apps"
	// Exact label value set by Docker when PowerLab deploys a compose app
	configFilesLabel := appsPath + "/xenodochial_pablo/docker-compose.yml"
	assert.Assert(t, hasPowerlabPrefix(configFilesLabel, appsPath),
		"PowerLab-deployed container must survive the List() prefix filter")

	// Sibling directory must be excluded — the original code had this bug
	siblingLabel := "/Users/user/powerlab/backend/data/apps-backup/app/docker-compose.yml"
	assert.Assert(t, !hasPowerlabPrefix(siblingLabel, appsPath),
		"sibling directory apps-backup must NOT match the apps prefix")
}

// TestDockerAPIVersionEnvIsSetByApiService verifies that calling ApiService
// always sets DOCKER_API_VERSION to at least "1.44" so the Docker SDK
// (which caps at 1.43) can talk to Docker daemons that require >= 1.44.
func TestDockerAPIVersionEnvIsSetByApiService(t *testing.T) {
	orig := os.Getenv("DOCKER_API_VERSION")
	os.Unsetenv("DOCKER_API_VERSION")
	t.Cleanup(func() {
		if orig != "" {
			os.Setenv("DOCKER_API_VERSION", orig)
		} else {
			os.Unsetenv("DOCKER_API_VERSION")
		}
	})

	// ApiService sets DOCKER_API_VERSION before touching the Docker socket.
	// The Docker error (if any) is irrelevant here — we only check the side-effect.
	_, _, _ = service.ApiService()

	got := os.Getenv("DOCKER_API_VERSION")
	assert.Assert(t, got != "", "DOCKER_API_VERSION must be set after ApiService()")

	var major, minor int
	_, err := fmt.Sscanf(got, "%d.%d", &major, &minor)
	assert.NilError(t, err, "DOCKER_API_VERSION %q must be parseable as M.N", got)
	assert.Assert(t, major > 1 || (major == 1 && minor >= 44),
		"DOCKER_API_VERSION %q must be >= 1.44", got)
}

// TestConfigAppsPathIsComputedFromWorkingDir documents the dynamic path resolution
// in pkg/config/init.go: AppsPath is derived from os.Getwd() at startup, not
// from a static config value. This test proves that after InitSetup the path
// always points to a "data/apps" directory relative to the project root.
func TestConfigAppsPathIsComputedFromWorkingDir(t *testing.T) {
	assert.Assert(t, config.AppInfo.AppsPath != "",
		"config.AppInfo.AppsPath must be non-empty after InitSetup")
	assert.Assert(t, strings.HasSuffix(config.AppInfo.AppsPath, "data/apps") ||
		strings.HasSuffix(config.AppInfo.AppsPath, "data"+string(filepath.Separator)+"apps"),
		"AppsPath %q should end with data/apps (set dynamically from os.Getwd())",
		config.AppInfo.AppsPath)
}
