package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bluele/gcache"
	"github.com/docker/docker/client"
	"github.com/neochaotic/powerlab/backend/app-management/codegen"
	"github.com/neochaotic/powerlab/backend/app-management/common"
	"github.com/neochaotic/powerlab/backend/app-management/pkg/config"
	"github.com/neochaotic/powerlab/backend/app-management/pkg/docker"
	"github.com/neochaotic/powerlab/backend/common/utils"
	"github.com/neochaotic/powerlab/backend/common/utils/file"
	"github.com/neochaotic/powerlab/backend/common/utils/logger"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

var ErrAppStoreSourceExists = fmt.Errorf("appstore source already exists")

// AppStoreManagement is the multi-store front for the app catalog.
// Owns the registered store list (admin-configurable via the
// AppStoreList ini key), the per-app upgrade-availability cache,
// and the in-flight upgrade tracking. Its CRUD methods drive the
// admin "App stores" panel.
type AppStoreManagement struct {
	isAppUpgradable      gcache.Cache
	defaultAppStore      AppStore
	isAppUpgrading       sync.Map
	onAppStoreRegister   []func(string) error
	onAppStoreUnregister []func(string) error
}

// AppStoreList returns the registered app-store list as render-
// ready metadata for the admin UI. Stores that fail to load are
// included with a marker so the UI can surface the error.
func (a *AppStoreManagement) AppStoreList() []codegen.AppStoreMetadata {
	return lo.Map(config.ServerInfo.AppStoreList, func(appStoreURL string, id int) codegen.AppStoreMetadata {
		appStore, err := AppStoreByURL(appStoreURL)
		if err != nil {
			logger.Error("failed to construct appstore", zap.Error(err), zap.String("appstoreURL", appStoreURL))
			return codegen.AppStoreMetadata{}
		}

		workDir, err := appStore.WorkDir()
		if err != nil {
			logger.Error("failed to get appstore workdir", zap.Error(err), zap.String("appstoreURL", appStoreURL))
			return codegen.AppStoreMetadata{}
		}

		storeRoot, err := StoreRoot(workDir)
		if err != nil {
			logger.Error("failed to get appstore storeRoot", zap.Error(err), zap.String("appstoreURL", appStoreURL))
			storeRoot = "internal error - store root not found"
		}

		return codegen.AppStoreMetadata{
			ID:        &id,
			URL:       &appStoreURL,
			StoreRoot: &storeRoot,
		}
	})
}

// OnAppStoreRegister appends a callback fired after a successful
// store registration. Used by code that needs to refresh derived
// caches (e.g. the homepage tile list).
func (a *AppStoreManagement) OnAppStoreRegister(fn func(string) error) {
	a.onAppStoreRegister = append(a.onAppStoreRegister, fn)
}

// OnAppStoreUnregister appends a callback fired after a successful
// store unregistration.
func (a *AppStoreManagement) OnAppStoreUnregister(fn func(string) error) {
	a.onAppStoreUnregister = append(a.onAppStoreUnregister, fn)
}

// ChangeGlobal sets a [global] config key to value and persists
// the conf file. Used by the admin UI's secret-management panel
// (OpenAI key, etc.).
func (a *AppStoreManagement) ChangeGlobal(key string, value string) error {
	config.Global[key] = value

	go func() {
		if err := config.SaveGlobal(); err != nil {
			logger.Error("failed to save global env", zap.Error(err), zap.String("key", key), zap.String("value", value))
			return
		}
	}()

	return nil
}

// DeleteGlobal removes a [global] config key from the conf file.
func (a *AppStoreManagement) DeleteGlobal(key string) error {
	for k := range config.Global {
		if k == key {
			delete(config.Global, k)
		}
	}

	go func() {
		if err := config.SaveGlobal(); err != nil {
			logger.Error("failed to delete global env", zap.Error(err), zap.String("key", key))
			return
		}
	}()

	return nil
}

// RegisterAppStore registers a new app-store URL — async variant
// that fans the work out to a goroutine and notifies the caller
// via callbacks. Use the Sync sibling for tests / setup scripts
// that need to block.
func (a *AppStoreManagement) RegisterAppStore(ctx context.Context, appstoreURL string, callbacks ...func(*codegen.AppStoreMetadata)) error {
	// check if appstore already exists
	for _, url := range config.ServerInfo.AppStoreList {
		if strings.EqualFold(url, appstoreURL) {
			return ErrAppStoreSourceExists
		}
	}

	appstore, err := AppStoreByURL(appstoreURL)
	if err != nil {
		return err
	}

	go func() {
		go PublishEventWrapper(ctx, common.EventTypeAppStoreRegisterBegin, nil)

		defer PublishEventWrapper(ctx, common.EventTypeAppStoreRegisterEnd, nil)

		var err error

		defer func() {
			if err == nil {
				return
			}

			PublishEventWrapper(ctx, common.EventTypeAppStoreRegisterError, map[string]string{
				common.PropertyTypeMessage.Name: err.Error(),
			})
		}()

		if err = appstore.UpdateCatalog(); err != nil {
			logger.Error("failed to update appstore catalog", zap.Error(err), zap.String("appstoreURL", appstoreURL))

			return
		}

		// if everything is good, add to the list
		config.ServerInfo.AppStoreList = append(config.ServerInfo.AppStoreList, appstoreURL)

		if err = config.SaveSetup(); err != nil {
			logger.Error("failed to save appstore list", zap.Error(err), zap.String("appstoreURL", appstoreURL))
			return
		}

		for _, fn := range a.onAppStoreRegister {
			if err := fn(appstoreURL); err != nil {
				logger.Error("failed to run onAppStoreRegister", zap.Error(err), zap.String("appstoreURL", appstoreURL))
			}
		}

		appStoreMetadata := &codegen.AppStoreMetadata{
			ID:  utils.Ptr(len(config.ServerInfo.AppStoreList) - 1),
			URL: &appstoreURL,
		}

		for _, callback := range callbacks {
			callback(appStoreMetadata)
		}
	}()

	return nil
}

// TODO: refactor the function and above function
// RegisterAppStoreSync is the synchronous variant — git-clones the
// store, builds the catalog, persists the URL to AppStoreList,
// and fires the OnAppStoreRegister callbacks before returning.
func (a *AppStoreManagement) RegisterAppStoreSync(ctx context.Context, appstoreURL string, callbacks ...func(*codegen.AppStoreMetadata)) error {
	// check if appstore already exists
	for _, url := range config.ServerInfo.AppStoreList {
		if strings.EqualFold(url, appstoreURL) {
			return ErrAppStoreSourceExists
		}
	}

	appstore, err := AppStoreByURL(appstoreURL)
	if err != nil {
		return err
	}

	go PublishEventWrapper(ctx, common.EventTypeAppStoreRegisterBegin, nil)

	defer PublishEventWrapper(ctx, common.EventTypeAppStoreRegisterEnd, nil)

	defer func() {
		if err == nil {
			return
		}

		PublishEventWrapper(ctx, common.EventTypeAppStoreRegisterError, map[string]string{
			common.PropertyTypeMessage.Name: err.Error(),
		})
	}()

	if err = appstore.UpdateCatalog(); err != nil {
		logger.Error("failed to update appstore catalog", zap.Error(err), zap.String("appstoreURL", appstoreURL))

		return err
	}

	// if everything is good, add to the list
	config.ServerInfo.AppStoreList = append(config.ServerInfo.AppStoreList, appstoreURL)

	if err = config.SaveSetup(); err != nil {
		logger.Error("failed to save appstore list", zap.Error(err), zap.String("appstoreURL", appstoreURL))
		return err
	}

	for _, fn := range a.onAppStoreRegister {
		if err := fn(appstoreURL); err != nil {
			logger.Error("failed to run onAppStoreRegister", zap.Error(err), zap.String("appstoreURL", appstoreURL))
		}
	}

	appStoreMetadata := &codegen.AppStoreMetadata{
		ID:  utils.Ptr(len(config.ServerInfo.AppStoreList) - 1),
		URL: &appstoreURL,
	}

	for _, callback := range callbacks {
		callback(appStoreMetadata)
	}

	return nil
}

// UnregisterAppStore removes an app-store entry by index in
// AppStoreList, persists the conf, and fires the
// OnAppStoreUnregister callbacks. Note this DOES NOT delete the
// on-disk catalog — admin must clean it up manually.
func (a *AppStoreManagement) UnregisterAppStore(appStoreID uint) error {
	if appStoreID >= uint(len(config.ServerInfo.AppStoreList)) {
		return fmt.Errorf("appstore id %d out of range", appStoreID)
	}

	appStoreURL := config.ServerInfo.AppStoreList[appStoreID]

	// remove appstore from list
	{
		config.ServerInfo.AppStoreList = append(config.ServerInfo.AppStoreList[:appStoreID], config.ServerInfo.AppStoreList[appStoreID+1:]...)

		if err := config.SaveSetup(); err != nil {
			return err
		}
	}

	// remove appstore workdir
	{
		appStore, err := AppStoreByURL(appStoreURL)
		if err != nil {
			return err
		}

		workdir, err := appStore.WorkDir()
		if err != nil {
			logger.Error("error while getting appstore workdir", zap.Error(err), zap.String("url", appStoreURL))
		}

		if len(workdir) != 0 {
			if err := file.RMDir(workdir); err != nil {
				logger.Error("error while removing appstore workdir", zap.Error(err), zap.String("workdir", workdir))
			}
		}
	}

	for _, fn := range a.onAppStoreUnregister {
		if err := fn(appStoreURL); err != nil {
			return err
		}
	}
	return nil
}

// AppStoreMap returns every registered AppStore keyed by URL.
// Lazy-resolves stores on first call.
func (a *AppStoreManagement) AppStoreMap() (map[string]AppStore, error) {
	appStoreMap := lo.SliceToMap(config.ServerInfo.AppStoreList, func(appStoreURL string) (string, AppStore) {
		appStore, err := AppStoreByURL(appStoreURL)
		if err != nil {
			return "", nil
		}
		return appStoreURL, appStore
	})

	delete(appStoreMap, "")

	return appStoreMap, nil
}

// AppStore interface
// CategoryMap returns the merged category index across every
// registered store. The default store wins on key collisions.
func (a *AppStoreManagement) CategoryMap() (map[string]codegen.CategoryInfo, error) {
	appStoreMap, err := a.AppStoreMap()
	if err != nil {
		return nil, err
	}

	allFailed := true

	categoryMap := map[string]codegen.CategoryInfo{}
	for _, appStore := range appStoreMap {
		c, err := appStore.CategoryMap()
		if err != nil {
			logger.Error("error while loading category map", zap.Error(err))
			continue
		}

		allFailed = false

		for name, category := range c {
			categoryMap[name] = category
		}
	}

	if allFailed {
		logger.Info("all appstores failed to load category map, using default")

		categoryMap, err = a.defaultAppStore.CategoryMap()
		if err != nil {
			return nil, err
		}
	}

	for name, category := range categoryMap {
		category.Count = utils.Ptr(0)
		categoryMap[name] = category
	}

	catalog, err := a.Catalog()
	if err != nil {
		return nil, err
	}

	for _, app := range catalog {
		storeInfo, err := app.StoreInfo(false)
		if err != nil {
			continue
		}

		category, ok := categoryMap[storeInfo.Category]
		if !ok {
			continue
		}

		category.Count = lo.ToPtr(*category.Count + 1)

		categoryMap[storeInfo.Category] = category
	}

	return categoryMap, nil
}

// Recommend returns the merged "Recommended" app-id list across
// every registered store.
func (a *AppStoreManagement) Recommend() ([]string, error) {
	appStoreMap, err := a.AppStoreMap()
	if err != nil {
		logger.Error("error while loading appstore map", zap.Error(err))
		return nil, err
	}

	allFailed := true

	recommend := []string{}
	for _, appStore := range appStoreMap {
		r, err := appStore.Recommend()
		if err != nil {
			logger.Error("error while getting appstore recommend", zap.Error(err))
			continue
		}

		allFailed = false
		recommend = lo.Union(recommend, r)
	}

	if !allFailed {
		return recommend, nil
	}

	logger.Info("No appstore registered")
	if a.defaultAppStore == nil {
		logger.Info("WARNING - no default appstore")
		return nil, nil
	}

	logger.Info("Using default appstore")
	recommend, err = a.defaultAppStore.Recommend()
	if err != nil {
		logger.Error("error while getting default appstore recommend list", zap.Error(err))
		return nil, err
	}

	return recommend, nil
}

// Catalog returns the merged catalog across every registered
// store, keyed by app id. App-id collisions are resolved by store
// registration order — first store wins.
func (a *AppStoreManagement) Catalog() (map[string]*ComposeApp, error) {
	catalog := map[string]*ComposeApp{}
	// Track which IDs have been added (case-insensitively) so the same app from
	// a lower-priority store (e.g. CasaOS "2fauth") does not shadow the higher-priority
	// store's entry (e.g. local "2FAuth") and does not produce duplicates in the response.
	seen := map[string]bool{}

	appStoreMap, err := a.AppStoreMap()
	if err != nil {
		return nil, err
	}

	allFailed := true

	// Iterate in config order so earlier stores (e.g. local) take priority over later ones.
	for _, appStoreURL := range config.ServerInfo.AppStoreList {
		appStore, ok := appStoreMap[appStoreURL]
		if !ok {
			continue
		}

		c, err := appStore.Catalog()
		if err != nil {
			logger.Error("error while getting appstore catalog", zap.Error(err))
			continue
		}

		allFailed = false
		for storeAppID, composeApp := range c {
			lower := strings.ToLower(storeAppID)
			if !seen[lower] {
				seen[lower] = true
				catalog[storeAppID] = composeApp
			}
		}
	}

	if !allFailed {
		return catalog, nil
	}

	logger.Info("No appstore registered")
	if a.defaultAppStore == nil {
		logger.Info("WARNING - no default appstore")
		return map[string]*ComposeApp{}, nil
	}

	logger.Info("Using default appstore")
	catalog, err = a.defaultAppStore.Catalog()
	if err != nil {
		return map[string]*ComposeApp{}, err
	}

	return catalog, nil
}

// UpdateCatalog re-runs UpdateCatalog on every registered store
// (typically after a `git pull` on the catalog repos). Errors
// from individual stores are logged + swallowed so a single
// store failure doesn't kill the refresh.
func (a *AppStoreManagement) UpdateCatalog() error {
	// reload config.
	// the appstore may be change in runtime.
	config.ReloadConfig()

	appStoreMap, err := a.AppStoreMap()
	if err != nil {
		return err
	}

	for url, appStore := range appStoreMap {
		if err := appStore.UpdateCatalog(); err != nil {
			logger.Error("error while updating catalog for app store", zap.Error(err), zap.String("url", url))
		}
	}

	// clean cache
	a.isAppUpgradable.Purge()

	return nil
}

// ComposeApp resolves a single ComposeApp by id across every
// registered store — first hit wins.
func (a *AppStoreManagement) ComposeApp(id string) (*ComposeApp, error) {
	appStoreMap, err := a.AppStoreMap()
	if err != nil {
		return nil, err
	}

	for _, appStore := range appStoreMap {
		composeApp, appErr := appStore.ComposeApp(id)
		if appErr != nil {
			logger.Error("error while getting appstore compose app", zap.Error(appErr))
			continue
		}

		if composeApp != nil {
			return composeApp, nil
		}
	}

	logger.Info("app not found in any appstore", zap.String("id", id))

	if a.defaultAppStore == nil {
		logger.Info("WARNING - no default appstore")
		return nil, nil
	}

	logger.Info("Using default appstore")

	composeApp, err := a.defaultAppStore.ComposeApp(id)
	if err != nil {
		return nil, err
	}

	return composeApp, nil
}

// WorkDir returns the on-disk root used to host catalog clones —
// the configured AppInfo.AppStorePath, ensured to exist.
func (a *AppStoreManagement) WorkDir() (string, error) {
	panic("not implemented and will never be implemented - this is a virtual appstore")
}

// IsUpdateAvailable reports whether composeApp has a newer image
// in any registered store. Cached for the upgrade-availability
// TTL so the homepage doesn't hammer registries on every load.
func (a *AppStoreManagement) IsUpdateAvailable(composeApp *ComposeApp) bool {
	storeID := composeApp.Name
	if value, err := a.isAppUpgradable.Get(storeID); err == nil {
		switch value := value.(type) {
		case bool:
			return value
		default:
			logger.Error("invalid type in cache", zap.String("storeID", storeID), zap.Any("value", value))
			return false
		}
	}

	isUpdate, err := a.isUpdateAvailable(composeApp)
	if err != nil {
		logger.Error("failed to check if update is available", zap.Error(err))
		return false
	}
	_ = a.isAppUpgradable.Set(storeID, isUpdate)
	return isUpdate
}

func (a *AppStoreManagement) isUpdateAvailable(composeApp *ComposeApp) (bool, error) {
	// handle no tag logic and for easy to test
	storeInfo, err := composeApp.StoreInfo(false)
	if err != nil {
		logger.Error("failed to get store info of compose app, thus no update available", zap.Error(err))
		return false, nil
	}

	// if app is uncontrolled, no update available
	if storeInfo.IsUncontrolled != nil && *storeInfo.IsUncontrolled {
		return false, nil
	}

	if storeInfo == nil || storeInfo.StoreAppID == nil || *storeInfo.StoreAppID == "" {
		return false, err
	}

	storeComposeApp, err := a.ComposeApp(*storeInfo.StoreAppID)
	if err != nil {
		logger.Error("failed to get store compose app, thus no update available", zap.Error(err))
		return false, err
	}

	if storeComposeApp == nil {
		logger.Error("store compose app not found, thus no update available", zap.String("storeAppID", *storeInfo.StoreAppID))
		return false, nil
	}

	return a.IsUpdateAvailableWith(composeApp, storeComposeApp)
}

// the patch is have no choice
// the digest compare is not work for these images
// I don't know why, but I have to do this
// I will remove the patch after I rewrite the digest compare
var NoUpdateBlacklist = []string{
	"johnguan/stable-diffusion-webui:latest",
}

// IsUpdateAvailableWith compares an installed composeApp against
// a specific storeComposeApp — used by IsUpdateAvailable as the
// inner check when iterating stores.
func (a *AppStoreManagement) IsUpdateAvailableWith(composeApp *ComposeApp, storeComposeApp *ComposeApp) (bool, error) {
	currentTag, err := composeApp.MainTag()
	if err != nil {
		logger.Error("failed to get current tag", zap.Error(err))
		return false, err
	}
	mainService, err := composeApp.MainService()
	if err != nil {
		logger.Error("failed to get main service", zap.Error(err))
		return false, err
	}
	if lo.Contains(common.NeedCheckDigestTags, currentTag) {
		ctx := context.Background()
		cli, clientErr := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if clientErr != nil {
			logger.Error("failed to create docker client", zap.Error(clientErr))
			return false, clientErr
		}
		defer cli.Close()

		if lo.Contains(NoUpdateBlacklist, mainService.Image) {
			return false, nil
		}

		image, _ := docker.ExtractImageAndTag(mainService.Image)

		imageInfo, _, clientErr := cli.ImageInspectWithRaw(ctx, image)
		if clientErr != nil {
			logger.Error("failed to inspect image", zap.Error(clientErr))
			return false, clientErr
		}

		match, clientErr := docker.CompareDigest(mainService.Image, imageInfo.RepoDigests)
		if clientErr != nil {
			logger.Error("failed to compare digest", zap.Error(clientErr))
			return false, clientErr
		}
		// match means no update available
		return !match, nil
	}
	storeTag, err := storeComposeApp.MainTag()
	return currentTag != storeTag, err
}

// IsUpdating reports whether an in-place upgrade is currently
// running for appID. Used to gate the "Update" button + show
// progress.
func (a *AppStoreManagement) IsUpdating(appID string) bool {
	_, ok := a.isAppUpgrading.Load(appID)
	return ok
}

// StartUpgrade marks appID as upgrading. Pair with FinishUpgrade.
func (a *AppStoreManagement) StartUpgrade(appID string) {
	a.isAppUpgrading.Store(appID, struct{}{})
}

// FinishUpgrade clears the upgrading flag for appID + invalidates
// the upgrade-availability cache entry so the new state is read
// on next homepage load.
func (a *AppStoreManagement) FinishUpgrade(appID string) {
	a.isAppUpgrading.Delete(appID)
	a.isAppUpgradable.Remove(appID)
}

// NewAppStoreManagement returns the process-wide AppStoreManagement
// singleton. Bootstraps the upgrade-availability cache (LRU) and
// the in-flight upgrade map.
func NewAppStoreManagement() *AppStoreManagement {
	defaultAppStore, err := NewDefaultAppStore()
	if err != nil {
		fmt.Printf("error while loading default appstore: %s\n", err.Error())
	}

	appStoreManagement := &AppStoreManagement{
		defaultAppStore: defaultAppStore,
		isAppUpgradable: gcache.New(100).LRU().Expiration(1 * time.Hour).Build(),
		isAppUpgrading:  sync.Map{},
	}

	return appStoreManagement
}
