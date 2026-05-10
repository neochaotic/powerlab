package service

import (
	"crypto/md5" // nolint: gosec
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neochaotic/powerlab/backend/app-management/codegen"
	"github.com/neochaotic/powerlab/backend/app-management/common"
	"github.com/neochaotic/powerlab/backend/app-management/pkg/config"
	"github.com/neochaotic/powerlab/backend/app-management/pkg/utils/downloadHelper"
	"github.com/neochaotic/powerlab/backend/common/utils/file"
	"github.com/neochaotic/powerlab/backend/common/utils/logger"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// AppStore is one source of catalog data — typically a git
// repository checked out under AppInfo.AppStorePath, but in
// principle a non-git directory works too. Multiple stores are
// queried by AppStoreManagement and merged into a single homepage
// catalog.
type AppStore interface {
	// Catalog returns every ComposeApp in this store, keyed by
	// app id (the directory name under the store root).
	Catalog() (map[string]*ComposeApp, error)
	// CategoryMap returns the store's category index — the icons
	// and counts shown in the homepage filter chips.
	CategoryMap() (map[string]codegen.CategoryInfo, error)
	// ComposeApp returns the single ComposeApp identified by id,
	// or an error if it isn't in this store.
	ComposeApp(id string) (*ComposeApp, error)
	// Recommend returns the editor-curated app id list shown as
	// "Recommended" on the homepage tab.
	Recommend() ([]string, error)
	// UpdateCatalog re-reads the store directory from disk
	// (typically after a git pull) and rebuilds the in-memory
	// catalog. Idempotent.
	UpdateCatalog() error
	// WorkDir returns the on-disk root of the store's catalog —
	// used by callers that want to mutate or inspect raw files.
	WorkDir() (string, error)
}

type appStore struct {
	categoryMap map[string]codegen.CategoryInfo
	catalog     map[string]*ComposeApp
	recommend   []string
	url         string

	lastAPPStoreSize int64
}

var (
	appStoreMap = make(map[string]*appStore)

	ErrNotAppStore             = fmt.Errorf("not an appstore")
	ErrDefaultAppStoreNotFound = fmt.Errorf("default appstore not found")
)

func (s *appStore) CategoryMap() (map[string]codegen.CategoryInfo, error) {
	if s.categoryMap != nil {
		return s.categoryMap, nil
	}

	workdir, err := s.WorkDir()
	if err != nil {
		return nil, err
	}

	storeRoot, err := StoreRoot(workdir)
	if err != nil {
		return nil, err
	}

	categoryMap := LoadCategoryMap(storeRoot)

	s.categoryMap = categoryMap

	return s.categoryMap, nil
}

// isLocalPath returns true if the URL is a local filesystem path (absolute path or file://).
func isLocalPath(rawURL string) bool {
	if strings.HasPrefix(rawURL, "/") {
		return true
	}
	u, err := url.Parse(rawURL)
	return err == nil && u.Scheme == "file"
}

func (s *appStore) UpdateCatalog() error {
	isSuccessful := false

	if _, err := url.Parse(s.url); err != nil {
		return err
	}

	// For local directory stores: skip the HTTP size check and just load directly.
	if isLocalPath(s.url) {
		localPath := s.url
		if strings.HasPrefix(localPath, "file://") {
			localPath = strings.TrimPrefix(localPath, "file://")
		}
		storeRoot, err := StoreRoot(localPath)
		if err != nil {
			return fmt.Errorf("local appstore at %q: %w", localPath, err)
		}
		catalog, err := BuildCatalog(storeRoot)
		if err != nil {
			return err
		}
		s.catalog = catalog
		logger.Info("local appstore catalog loaded", zap.String("path", localPath), zap.Int("apps", len(catalog)))
		return nil
	}

	// check wether the zip package size change
	// if not, skip the update
	{
		// timeout 5s
		http.DefaultClient.Timeout = 5 * time.Second
		res, err := http.Head(s.url)
		if err != nil {
			return err
		}
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to get appstore size, status code: %d", res.StatusCode)
		}
		if res.ContentLength == s.lastAPPStoreSize {
			logger.Info("appstore size not changed", zap.String("url", s.url))
			return nil
		}
		logger.Info("appstore size changed, update app store", zap.String("url", s.url))

		defer func() {
			if isSuccessful {
				s.lastAPPStoreSize = res.ContentLength
			}
		}()
	}

	workdir, err := s.WorkDir()
	if err != nil {
		return err
	}
	tmpDir := workdir + ".tmp"

	defer func() {
		if err := file.RMDir(tmpDir); err != nil {
			logger.Error("failed to remove temp appstore workdir", zap.Error(err), zap.String("tmpDir", tmpDir))
		}
	}()

	if err := downloadHelper.Download(s.url, tmpDir); err != nil {
		return err
	}

	// make a backup of existing workdir
	if file.Exists(workdir) {
		backupDir := workdir + ".backup"

		if err := file.RMDir(backupDir); err != nil {
			return err
		}

		if err := os.Rename(workdir, backupDir); err != nil {
			return err
		}

		defer func() {
			if isSuccessful {
				if err := file.RMDir(backupDir); err != nil {
					logger.Error("failed to remove backup appstore workdir", zap.Error(err), zap.String("backupDir", backupDir))
				}
				return
			}

			if err := file.RMDir(workdir); err != nil {
				logger.Error("failed to remove appstore workdir", zap.Error(err), zap.String("workdir", workdir))
			}

			if err := os.Rename(backupDir, workdir); err != nil {
				logger.Error("failed to restore backup appstore workdir", zap.Error(err), zap.String("backupDir", backupDir), zap.String("workdir", workdir))
			}
		}()
	}

	if err := os.Rename(tmpDir, workdir); err != nil {
		return err
	}

	storeRoot, err := StoreRoot(workdir)
	if err != nil {
		return err
	}

	// Marker file written into each app store dir so the registry
	// service can identify the store provenance from disk. The file
	// is regenerated on every store sync, so renaming the basename
	// from ".casaos-appstore" to ".powerlab-appstore" leaves no
	// stale state behind. Sprint 4 PR1 cosmetic rebrand (#85, audit
	// `docs/audits/sprint-4-app-management-prep.md`).
	placeholderFile := filepath.Join(storeRoot, ".powerlab-appstore")
	if err := file.CreateFileAndWriteContent(placeholderFile, s.url); err != nil {
		return err
	}

	s.catalog, err = BuildCatalog(storeRoot)
	if err != nil {
		return err
	}

	s.categoryMap = LoadCategoryMap(storeRoot)

	s.recommend = LoadRecommend(storeRoot)

	isSuccessful = true

	return nil
}

func (s *appStore) Recommend() ([]string, error) {
	if s.recommend != nil && len(s.recommend) > 0 {
		return s.recommend, nil
	}

	workdir, err := s.WorkDir()
	if err != nil {
		return nil, err
	}

	storeRoot, err := StoreRoot(workdir)
	if err != nil {
		return nil, err
	}

	return LoadRecommend(storeRoot), nil
}

func (s *appStore) Catalog() (map[string]*ComposeApp, error) {
	if s.catalog != nil && len(s.catalog) > 0 {
		return s.catalog, nil
	}

	workdir, err := s.WorkDir()
	if err != nil {
		return nil, err
	}

	storeRoot, err := StoreRoot(workdir)
	if err != nil {
		return nil, err
	}

	catalog, err := BuildCatalog(storeRoot)
	if err != nil {
		return nil, err
	}

	s.catalog = catalog

	return s.catalog, nil
}

func (s *appStore) ComposeApp(appStoreID string) (*ComposeApp, error) {
	catalog, err := s.Catalog()
	if err != nil {
		return nil, err
	}

	if composeApp, ok := catalog[appStoreID]; ok {
		return composeApp, nil
	}

	// Case-insensitive fallback: stores may key the same app with different casing.
	lower := strings.ToLower(appStoreID)
	for id, composeApp := range catalog {
		if strings.ToLower(id) == lower {
			return composeApp, nil
		}
	}

	return nil, nil
}

func (s *appStore) WorkDir() (string, error) {
	if s.url == "default" {
		return filepath.Join(config.AppInfo.AppStorePath, s.url), nil
	}

	// Local filesystem stores serve their compose YAMLs directly out of
	// the configured directory — there is no download/extract step.
	// Without this branch, WorkDir for a local URL would resolve to
	// AppStorePath/<md5hash>/, which doesn't exist, and AppStoreList
	// would render `store_root: "internal error - store root not found"`
	// even though UpdateCatalog had already loaded the catalog from
	// the real local path. Stay consistent with the local-path branch
	// in UpdateCatalog above.
	if isLocalPath(s.url) {
		localPath := s.url
		if strings.HasPrefix(localPath, "file://") {
			localPath = strings.TrimPrefix(localPath, "file://")
		}
		return localPath, nil
	}

	parsedURL, err := url.Parse(s.url)
	if err != nil {
		return "", err
	}

	appstoreKey := strings.ToLower(parsedURL.Path)

	hash := fmt.Sprintf("%x", md5.Sum([]byte(appstoreKey))) //nolint: gosec

	return filepath.Join(config.AppInfo.AppStorePath, parsedURL.Host, hash), nil
}

// AppStoreByURL returns the cached AppStore for the given
// appstoreURL, lazy-creating one if it doesn't exist yet. The URL
// can be a git remote ("https://...git") or a local path.
func AppStoreByURL(appstoreURL string) (AppStore, error) {
	_, err := url.Parse(appstoreURL)
	if err != nil {
		return nil, err
	}

	// a appstoreKey is a normalized appstore url where everything is in lowercase
	appstoreKey := strings.ToLower(appstoreURL)
	if appstore, ok := appStoreMap[appstoreKey]; ok {
		return appstore, nil
	}

	appStoreMap[appstoreKey] = &appStore{
		url:     appstoreURL,
		catalog: map[string]*ComposeApp{},
	}

	return appStoreMap[appstoreKey], nil
}

// NewDefaultAppStore returns the AppStore for the configured
// default catalog (the bundled "Awesome PowerLab Apps" remote).
// Returns ErrDefaultAppStoreNotFound if the catalog hasn't been
// cloned/checked out yet.
func NewDefaultAppStore() (AppStore, error) {
	storeRoot := filepath.Join(config.AppInfo.AppStorePath, "default")

	if !file.Exists(storeRoot) {
		return nil, ErrDefaultAppStoreNotFound
	}

	categoryMap := LoadCategoryMap(storeRoot)

	catalog, err := BuildCatalog(storeRoot)
	if err != nil {
		return nil, err
	}

	recommend := LoadRecommend(storeRoot)

	return &appStore{
		url:         "default",
		categoryMap: categoryMap,
		catalog:     catalog,
		recommend:   recommend,
	}, nil
}

// LoadCategoryMap reads category.list.json under storeRoot and
// returns the parsed category index. Empty map (not nil) on a
// missing/malformed file so callers can iterate safely.
func LoadCategoryMap(storeRoot string) map[string]codegen.CategoryInfo {
	categoryListFile := filepath.Join(storeRoot, common.CategoryListFileName)

	// unmarsal category list
	categoryList := []codegen.CategoryInfo{}

	if !file.Exists(categoryListFile) {
		return map[string]codegen.CategoryInfo{}
	}

	buf := file.ReadFullFile(categoryListFile)

	if err := json.Unmarshal(buf, &categoryList); err != nil {
		logger.Error("failed to unmarshal category list", zap.Error(err), zap.String("categoryListFile", categoryListFile))
		return map[string]codegen.CategoryInfo{}
	}

	categoryList = lo.Filter(categoryList, func(category codegen.CategoryInfo, i int) bool {
		return category.Name != nil && *category.Name != ""
	})

	categoryList = lo.Map(categoryList, func(category codegen.CategoryInfo, i int) codegen.CategoryInfo {
		if category.Font == nil || *category.Font == "" {
			category.Font = lo.ToPtr(common.DefaultCategoryFont)
		}

		if category.Description == nil {
			category.Description = lo.ToPtr("")
		}

		return category
	})

	return lo.SliceToMap(categoryList, func(category codegen.CategoryInfo) (string, codegen.CategoryInfo) {
		return *category.Name, category
	})
}

// LoadRecommend reads recommend.list.json under storeRoot and
// returns the editor-curated app id list shown on the Recommended
// tab. Empty slice (not nil) on a missing/malformed file.
func LoadRecommend(storeRoot string) []string {
	recommendListFile := filepath.Join(storeRoot, common.RecommendListFileName)

	// unmarsal recommend list
	recommendList := []interface{}{}

	if !file.Exists(recommendListFile) {
		logger.Info("recommend list file not found", zap.String("recommendListFile", recommendListFile))
		return []string{}
	}

	buf := file.ReadFullFile(recommendListFile)
	if err := json.Unmarshal(buf, &recommendList); err != nil {
		logger.Error("failed to unmarshal recommend list", zap.Error(err), zap.String("recommendListFile", recommendListFile))
		return []string{}
	}

	result := lo.Map(recommendList, func(item interface{}, i int) string {
		recommendItem, ok := item.(map[string]interface{})
		if !ok {
			return ""
		}

		storeAppID, ok := recommendItem["appid"]
		if !ok {
			return ""
		}

		return storeAppID.(string)
	})

	return result
}

// BuildCatalog walks storeRoot/Apps/* and returns one ComposeApp
// per directory whose docker-compose.yml loads cleanly. Apps with
// load errors are logged and skipped — partial catalog is better
// than no catalog.
func BuildCatalog(storeRoot string) (map[string]*ComposeApp, error) {
	catalog := map[string]*ComposeApp{}

	// walk through each folder under storeRoot/Apps and build the catalog
	if err := filepath.WalkDir(filepath.Join(storeRoot, common.AppsDirectoryName), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			return nil
		}

		composeFile := filepath.Join(path, common.ComposeYAMLFileName)
		if !file.Exists(composeFile) {
			// retry with ".yaml" extension
			composeFile = strings.TrimSuffix(composeFile, ".yml") + ".yaml"
			if !file.Exists(composeFile) {
				return nil
			}
		}

		composeYAML := file.ReadFullFile(composeFile)
		if len(composeYAML) == 0 {
			return nil
		}

		composeApp, err := NewComposeAppFromYAML(composeYAML, true, false)
		if err != nil {
			logger.Info("failed to parse compose app - contact the contributor of this app to fix it", zap.Error(err), zap.String("composeFile", composeFile))
			return fs.SkipDir // skip invalid compose app
		}

		catalog[composeApp.Name] = composeApp

		return nil
	}); err != nil {
		return nil, err
	}

	return catalog, nil
}

// StoreRoot resolves the catalog-root directory under workdir.
// The bundled catalog uses the layout
// <workdir>/Apps + category.list.json + recommend.list.json;
// for other layouts callers can wrap their own AppStore.
func StoreRoot(workdir string) (string, error) {
	storeRoot := ""

	// locate the path that contains the Apps directory
	if err := filepath.WalkDir(workdir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() && d.Name() == common.AppsDirectoryName {
			storeRoot = filepath.Dir(path)
			return filepath.SkipDir
		}

		return nil
	}); err != nil {
		return "", err
	}

	if storeRoot != "" {
		return storeRoot, nil
	}

	return "", ErrNotAppStore
}
