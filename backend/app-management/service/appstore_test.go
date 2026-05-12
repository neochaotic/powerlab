package service_test

import (
	_ "embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/neochaotic/powerlab/backend/app-management/common"
	"github.com/neochaotic/powerlab/backend/app-management/pkg/config"
	"github.com/neochaotic/powerlab/backend/app-management/pkg/docker"
	"github.com/neochaotic/powerlab/backend/app-management/service"
	"github.com/neochaotic/powerlab/backend/common/utils/file"
	"github.com/neochaotic/powerlab/backend/common/utils/logger"
	"github.com/samber/lo"
	"gotest.tools/v3/assert"
)

// isURLReachable performs a quick HEAD request with a 4s timeout to check network reachability.
func isURLReachable(url string) bool {
	c := &http.Client{Timeout: 4 * time.Second}
	resp, err := c.Head(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

func TestGetComposeApp(t *testing.T) {
	logger.LogInitConsoleOnly()

	defer func() {
		// workaround due to https://github.com/patrickmn/go-cache/issues/166
		docker.Cache = nil
		runtime.GC()
	}()

	appStorePath, err := os.MkdirTemp("", "appstore")
	assert.NilError(t, err)

	defer os.RemoveAll(appStorePath)

	config.AppInfo.AppStorePath = appStorePath

	appStore, err := service.AppStoreByURL("https://github.com/IceWhaleTech/_appstore/archive/refs/heads/main.zip")
	assert.NilError(t, err)

	err = appStore.UpdateCatalog()
	assert.NilError(t, err)

	catalog, err := appStore.Catalog()
	assert.NilError(t, err)

	for name, composeApp := range catalog {
		assert.Equal(t, name, composeApp.Name)
	}
}

func TestGetApp(t *testing.T) {

	defer func() {
		// workaround due to https://github.com/patrickmn/go-cache/issues/166
		docker.Cache = nil
		runtime.GC()
	}()

	logger.LogInitConsoleOnly()

	appStorePath, err := os.MkdirTemp("", "appstore")
	assert.NilError(t, err)

	defer os.RemoveAll(appStorePath)

	config.AppInfo.AppStorePath = appStorePath

	appStore, err := service.AppStoreByURL("https://github.com/IceWhaleTech/_appstore/archive/refs/heads/main.zip")
	assert.NilError(t, err)

	err = appStore.UpdateCatalog()
	assert.NilError(t, err)

	catalog, err := appStore.Catalog()
	assert.NilError(t, err)

	for _, composeApp := range catalog {
		for _, service := range composeApp.Services {
			app := composeApp.App(service.Name)
			assert.Equal(t, app.Name, service.Name)
		}
	}
}

// Note: the test need root permission
func TestSkipUpdateCatalog(t *testing.T) {
	logger.LogInitConsoleOnly()

	// Was previously also `casaos.oss-cn-shanghai.aliyuncs.com` (China
	// region mirror). Dropped on 2026-05-12 — PowerLab never referenced
	// that mirror in any production conf, and the test was flaking ~40 %
	// of CI runs with TLS handshake timeouts that exceeded the test
	// timeout (the per-URL HEAD probe was fast enough to pass the
	// `isURLReachable` skip-gate, but the full ZIP GET that
	// `UpdateCatalog()` performs ran 28 s+ and failed). The remaining
	// `casaos.app` URL covers the cache-skip behavior under test
	// identically; we didn't lose coverage by dropping the duplicate.
	appStoreURL := []string{
		"https://casaos.app/store/main.zip",
	}

	for _, url := range appStoreURL {
		url := url
		name := strings.Split(url, "/")[2] // hostname as sub-test name
		t.Run(name, func(t *testing.T) {
			if !isURLReachable(url) {
				t.Skipf("URL not reachable (network/geo): %s", url)
			}

			appStore, err := service.AppStoreByURL(url)
			assert.NilError(t, err)
			workdir, err := appStore.WorkDir()
			assert.NilError(t, err)

			// mkdir workdir for first
			err = file.MkDir(workdir)
			assert.NilError(t, err)

			appStoreStat, err := os.Stat(workdir)
			assert.NilError(t, err)

			err = appStore.UpdateCatalog()
			assert.NilError(t, err)

			// get create and change time of appstore
			appStoreStatFirst, err := os.Stat(workdir)
			assert.NilError(t, err)

			assert.Equal(t, false, appStoreStatFirst.ModTime().Equal(appStoreStat.ModTime()))

			err = appStore.UpdateCatalog()
			assert.NilError(t, err)

			// get create and change time of appstore
			appStoreStatSecond, err := os.Stat(workdir)
			assert.NilError(t, err)

			assert.Equal(t, appStoreStatFirst.ModTime(), appStoreStatSecond.ModTime())
		})
	}
}

func TestWorkDir(t *testing.T) {

	defer func() {
		// workaround due to https://github.com/patrickmn/go-cache/issues/166
		docker.Cache = nil
		runtime.GC()
	}()

	// test for http
	hostport := "localhost:8080"
	appStore, err := service.AppStoreByURL("http://" + hostport)
	assert.NilError(t, err)

	workdir, err := appStore.WorkDir()
	assert.NilError(t, err)
	assert.Equal(t, workdir, filepath.Join(config.AppInfo.AppStorePath, hostport, "d41d8cd98f00b204e9800998ecf8427e"))

	// test for https
	appStore, err = service.AppStoreByURL("https://" + hostport)
	assert.NilError(t, err)

	workdir, err = appStore.WorkDir()
	assert.NilError(t, err)
	assert.Equal(t, workdir, filepath.Join(config.AppInfo.AppStorePath, hostport, "d41d8cd98f00b204e9800998ecf8427e"))

	// test for github
	hostname := "github.com"
	path := "/IceWhaleTech/CasaOS-AppStore/archive/refs/heads/main.zip"
	appStore, err = service.AppStoreByURL("https://" + hostname + path)
	assert.NilError(t, err)

	workdir, err = appStore.WorkDir()
	assert.NilError(t, err)
	assert.Equal(t, workdir, filepath.Join(config.AppInfo.AppStorePath, hostname, "8b0968a7d7cda3f813d05736a89d0c92"))
}

func TestStoreRoot(t *testing.T) {

	defer func() {
		// workaround due to https://github.com/patrickmn/go-cache/issues/166
		docker.Cache = nil
		runtime.GC()
	}()

	workdir := t.TempDir()

	expectedStoreRoot := filepath.Join(workdir, "github.com", "IceWhaleTech", "CasaOS-AppStore", "main")
	err := file.MkDir(filepath.Join(expectedStoreRoot, common.AppsDirectoryName))
	assert.NilError(t, err)

	actualStoreRoot, err := service.StoreRoot(workdir)
	assert.NilError(t, err)

	assert.Equal(t, actualStoreRoot, expectedStoreRoot)
}

func TestLoadCategoryList(t *testing.T) {

	defer func() {
		// workaround due to https://github.com/patrickmn/go-cache/issues/166
		docker.Cache = nil
		runtime.GC()
	}()

	logger.LogInitConsoleOnly()

	storeRoot := t.TempDir()

	categoryListFilePath := filepath.Join(storeRoot, common.CategoryListFileName)

	err := file.WriteToFullPath([]byte(common.SampleCategoryListJSON), categoryListFilePath, 0o644)
	assert.NilError(t, err)

	dummyList := []interface{}{}
	buf := file.ReadFullFile(categoryListFilePath)
	err = json.Unmarshal(buf, &dummyList)
	assert.NilError(t, err)

	actualCategoryMap := service.LoadCategoryMap(storeRoot)
	assert.Assert(t, actualCategoryMap != nil)
	assert.Equal(t, len(actualCategoryMap), len(dummyList))

	for name, category := range actualCategoryMap {
		assert.Assert(t, category.Name != nil)
		assert.Assert(t, *category.Name == name)

		assert.Assert(t, category.Font != nil)
		assert.Assert(t, *category.Font != "")

		assert.Assert(t, category.Description != nil)
	}
}

func TestLoadRecommend(t *testing.T) {

	defer func() {
		// workaround due to https://github.com/patrickmn/go-cache/issues/166
		docker.Cache = nil
		runtime.GC()
	}()

	logger.LogInitConsoleOnly()

	storeRoot := t.TempDir()

	recommendListFilePath := filepath.Join(storeRoot, common.RecommendListFileName)

	type recommendListItem struct {
		AppID string `json:"appid"`
	}

	expectedRecommendList := []recommendListItem{
		{AppID: "app1"},
		{AppID: "app2"},
		{AppID: "app3"},
	}
	buf, err := json.Marshal(expectedRecommendList)
	assert.NilError(t, err)

	err = file.WriteToFullPath(buf, recommendListFilePath, 0o644)
	assert.NilError(t, err)

	actualRecommendList := service.LoadRecommend(storeRoot)
	assert.DeepEqual(t, actualRecommendList, lo.Map(expectedRecommendList, func(item recommendListItem, i int) string {
		return item.AppID
	}))
}

func TestBuildCatalog(t *testing.T) {

	defer func() {
		// workaround due to https://github.com/patrickmn/go-cache/issues/166
		docker.Cache = nil
		runtime.GC()
	}()

	storeRoot := t.TempDir()

	// test for invalid storeRoot
	_, err := service.BuildCatalog(storeRoot)
	assert.ErrorType(t, err, new(fs.PathError))

	appsPath := filepath.Join(storeRoot, common.AppsDirectoryName)
	err = file.MkDir(appsPath)
	assert.NilError(t, err)

	// test for empty catalog
	catalog, err := service.BuildCatalog(storeRoot)
	assert.NilError(t, err)
	assert.Equal(t, len(catalog), 0)

	// build test catalog
	err = file.MkDir(filepath.Join(appsPath, "test1"))
	assert.NilError(t, err)

	err = file.WriteToFullPath([]byte(common.SampleComposeAppYAML), filepath.Join(appsPath, "test1", common.ComposeYAMLFileName), 0o644)
	assert.NilError(t, err)
	catalog, err = service.BuildCatalog(storeRoot)
	assert.NilError(t, err)
	assert.Equal(t, len(catalog), 1)
}
