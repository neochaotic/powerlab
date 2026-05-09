package service_test

// Tests for the PowerLab local app store feature.
//
// Covers three regression-prone behaviors:
//   1. A local filesystem path must NOT trigger an HTTP HEAD request.
//   2. BuildCatalog must discover docker-compose.yml files in Apps/<name>/.
//   3. When two stores share the same app ID, the first-registered store wins.
//
// These tests are the safety net for any developer who touches appstore.go,
// appstore_management.go, or the Catalog() merge logic.

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/neochaotic/powerlab/backend/app-management/pkg/config"
	"github.com/neochaotic/powerlab/backend/app-management/service"
	"github.com/neochaotic/powerlab/backend/common/utils/logger"
	"gotest.tools/v3/assert"
)

// makeMinimalStore writes the smallest valid app store directory tree and returns its path.
// appName is the compose project name; author is written into x-casaos so tests can verify priority.
func makeMinimalStore(t *testing.T, appName, author string) string {
	t.Helper()
	dir := t.TempDir()
	appsDir := filepath.Join(dir, "Apps", appName)
	if err := os.MkdirAll(appsDir, 0o755); err != nil {
		t.Fatalf("makeMinimalStore: %v", err)
	}
	compose := `name: ` + appName + `
services:
  ` + appName + `:
    image: ` + appName + `:latest
x-casaos:
  title:
    en_us: ` + appName + `
  author: ` + author + `
  tagline:
    en_us: A test app
`
	if err := os.WriteFile(filepath.Join(appsDir, "docker-compose.yml"), []byte(compose), 0o644); err != nil {
		t.Fatalf("makeMinimalStore: write compose: %v", err)
	}
	return dir
}

// initConfig initialises a blank config with a writable app store path.
// Returns the temp dir used for AppStorePath.
func initConfig(t *testing.T) string {
	t.Helper()
	defer func() {
		runtime.GC()
	}()
	f, err := os.CreateTemp("", "app-mgmt-*.conf")
	assert.NilError(t, err)
	t.Cleanup(func() { os.Remove(f.Name()) })

	config.InitSetup(f.Name(), "")
	storeDir := t.TempDir()
	config.AppInfo.AppStorePath = storeDir
	return storeDir
}

// --- Test 1: local path does not trigger HTTP ---

// TestLocalAppStoreNeverCallsHTTP verifies that registering an absolute local
// path as an app store URL loads the catalog without any HTTP activity.
// If the isLocalPath guard were missing, AppStoreByURL.UpdateCatalog would
// call http.Head on the local path and return "unsupported protocol scheme".
func TestLocalAppStoreNeverCallsHTTP(t *testing.T) {
	logger.LogInitConsoleOnly()
	initConfig(t)

	storeDir := makeMinimalStore(t, "myapp", "TestAuthor")

	appStore, err := service.AppStoreByURL(storeDir)
	assert.NilError(t, err)

	// UpdateCatalog must succeed; an HTTP error would make this fail.
	err = appStore.UpdateCatalog()
	assert.NilError(t, err, "UpdateCatalog on a local path must not attempt HTTP")

	catalog, err := appStore.Catalog()
	assert.NilError(t, err)
	assert.Assert(t, len(catalog) > 0, "catalog must contain at least one app")
	_, ok := catalog["myapp"]
	assert.Assert(t, ok, "catalog must contain the 'myapp' entry")
}

// --- Test 2: BuildCatalog discovers compose files correctly ---

// TestBuildCatalogFindsAppsDirectory verifies that BuildCatalog walks the
// Apps/ subdirectory and indexes every valid docker-compose.yml it finds.
func TestBuildCatalogFindsAppsDirectory(t *testing.T) {
	logger.LogInitConsoleOnly()
	initConfig(t)

	storeDir := makeMinimalStore(t, "alpha", "BuildAuthor")
	// Add a second app to the same store.
	betaDir := filepath.Join(storeDir, "Apps", "beta")
	assert.NilError(t, os.MkdirAll(betaDir, 0o755))
	assert.NilError(t, os.WriteFile(filepath.Join(betaDir, "docker-compose.yml"), []byte(`name: beta
services:
  beta:
    image: beta:latest
x-casaos:
  title:
    en_us: Beta
  tagline:
    en_us: Beta app
`), 0o644))

	storeRoot, err := service.StoreRoot(storeDir)
	assert.NilError(t, err)

	catalog, err := service.BuildCatalog(storeRoot)
	assert.NilError(t, err)
	assert.Equal(t, 2, len(catalog), "catalog must index both alpha and beta")
	assert.Assert(t, catalog["alpha"] != nil)
	assert.Assert(t, catalog["beta"] != nil)
}

// --- Test 3: catalog merge — first store wins ---

// TestCatalogFirstStorePriority verifies that when two stores share an app ID
// the first-registered store's version is kept in the merged catalog.
// This protects the PowerLab local store from being overwritten by the CasaOS
// remote stores when they contain apps with the same name.
func TestCatalogFirstStorePriority(t *testing.T) {
	logger.LogInitConsoleOnly()
	initConfig(t)

	// Two stores — both have an app called "shared".
	firstStore := makeMinimalStore(t, "shared", "FirstAuthor")
	secondStore := makeMinimalStore(t, "shared", "SecondAuthor")

	// Seed config with both URLs in order (first store has priority).
	config.ServerInfo.AppStoreList = []string{firstStore, secondStore}

	// Load both into the global appStoreMap via AppStoreByURL.
	store1, err := service.AppStoreByURL(firstStore)
	assert.NilError(t, err)
	assert.NilError(t, store1.UpdateCatalog())

	store2, err := service.AppStoreByURL(secondStore)
	assert.NilError(t, err)
	assert.NilError(t, store2.UpdateCatalog())

	// Use AppStoreManagement.Catalog() which implements the priority merge.
	mgmt := service.NewAppStoreManagement()
	catalog, err := mgmt.Catalog()
	assert.NilError(t, err)

	app, ok := catalog["shared"]
	assert.Assert(t, ok, "catalog must contain the shared app")

	storeInfo, err := app.StoreInfo(false)
	assert.NilError(t, err)

	assert.Equal(t, "FirstAuthor", storeInfo.Author,
		"first-registered store must win when two stores share an app ID")
}

// --- Test 4: file:// URL is also treated as local ---

// TestFileURLSchemeIsLocalPath verifies that a file:// URL is recognised as a
// local path and avoids HTTP. This is the alternate local-path syntax.
func TestFileURLSchemeIsLocalPath(t *testing.T) {
	logger.LogInitConsoleOnly()
	initConfig(t)

	storeDir := makeMinimalStore(t, "fileapp", "FileAuthor")
	fileURL := "file://" + storeDir

	appStore, err := service.AppStoreByURL(fileURL)
	assert.NilError(t, err)

	err = appStore.UpdateCatalog()
	assert.NilError(t, err, "file:// URL must not trigger HTTP")

	catalog, err := appStore.Catalog()
	assert.NilError(t, err)
	_, ok := catalog["fileapp"]
	assert.Assert(t, ok, "catalog must contain the 'fileapp' entry from file:// URL")
}
