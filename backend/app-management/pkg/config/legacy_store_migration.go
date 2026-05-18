package config

import (
	"crypto/md5"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// legacyCatalogStores lists URLs that PowerLab no longer supports as
// default app-store sources, per ADR-0038. The migration in
// `pruneLegacyAppStores` removes any of these from the loaded config
// at startup and persists the cleaned list.
//
// Match is case-insensitive substring — covers minor URL variations
// (http vs https, trailing slashes, jsdelivr CDN variants) without
// a regex-engine dependency for what is essentially an allowlist
// inverter on three known strings.
var legacyCatalogStores = []string{
	"cdn.jsdelivr.net/gh/icewhaletech/casaos-appstore",
	"github.com/icewhaletech/casaos-appstore",
	"github.com/bigbeartechworld/big-bear-casaos",
}

// pruneLegacyAppStores filters known-legacy URLs out of a store-URL
// list. Returns the filtered list AND the list of removed entries
// (for logging). The input slice is not mutated.
//
// Pure function — no I/O. Caller is responsible for persisting the
// cleaned list back to the config file when the removed slice is
// non-empty.
func pruneLegacyAppStores(input []string) (kept []string, removed []string) {
	kept = make([]string, 0, len(input))
	for _, url := range input {
		if isLegacyCatalogStore(url) {
			removed = append(removed, url)
			continue
		}
		kept = append(kept, url)
	}
	return kept, removed
}

// isLegacyCatalogStore reports whether a URL matches any entry in
// the legacyCatalogStores allowlist (case-insensitive substring).
func isLegacyCatalogStore(url string) bool {
	lower := strings.ToLower(url)
	for _, legacy := range legacyCatalogStores {
		if strings.Contains(lower, legacy) {
			return true
		}
	}
	return false
}

// MigrateAppStoreListLegacyRemoval mutates ServerInfo.AppStoreList in
// place to drop legacy catalog sources (ADR-0038), then persists the
// cleaned config if anything changed. Safe to call multiple times
// (idempotent — a second run finds nothing to remove).
//
// Logged with `[migration]` prefix so operators can grep startup
// output. Save failure is logged but doesn't panic — the in-memory
// list is correct for this process; next clean run will re-attempt
// the persist.
func MigrateAppStoreListLegacyRemoval() {
	if ServerInfo == nil {
		return
	}
	kept, removed := pruneLegacyAppStores(ServerInfo.AppStoreList)
	if len(removed) == 0 {
		return
	}
	for _, url := range removed {
		log.Printf("[migration] removed legacy catalog source: %s (ADR-0038)", url)
	}
	ServerInfo.AppStoreList = kept
	if Cfg == nil {
		// Tests may call this directly without going through InitSetup;
		// skip the persist in that case.
		return
	}
	if err := SaveSetup(); err != nil {
		log.Printf("[migration] WARN failed to persist cleaned AppStoreList: %v (will retry next start)", err)
	}
	// Parity with UnregisterAppStore in service/appstore_management.go:
	// removing the URL from the registered list ALSO needs to remove
	// the on-disk workdir so the cloned catalog content doesn't linger
	// at /var/lib/powerlab/appstore/<host>/<md5>/. Without this, an
	// operator listing the appstore dir after migration would still see
	// the old CasaOS/big-bear directories. (#450)
	if AppInfo != nil && AppInfo.AppStorePath != "" {
		removeLegacyWorkDirs(AppInfo.AppStorePath, removed)
	}
}

// legacyURLWorkDir computes the on-disk workdir path the appstore
// service uses for a registered URL — mirrors service/appstore.go
// `(*appStore).WorkDir()` for the URL branch. Returns "" for entries
// that have no separate workdir (local paths, file:// URLs, parse
// failures) so callers can skip them.
//
// Kept in the config package so MigrateAppStoreListLegacyRemoval can
// reach it without depending on the service package (which would
// create an init-order cycle — config is loaded before service is set
// up).
func legacyURLWorkDir(rawURL, appStorePath string) string {
	if rawURL == "" || appStorePath == "" {
		return ""
	}
	if strings.HasPrefix(rawURL, "file://") {
		return ""
	}
	// isLocalPath is borrowed from the service package's heuristic —
	// inline here to avoid the import cycle. A "local path" is anything
	// that starts with / or ./ or doesn't parse as an http(s) URL.
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	if parsed.Host == "" {
		return ""
	}
	appstoreKey := strings.ToLower(parsed.Path)
	hash := fmt.Sprintf("%x", md5.Sum([]byte(appstoreKey))) //nolint: gosec
	return filepath.Join(appStorePath, parsed.Host, hash)
}

// removeLegacyWorkDirs deletes the workdirs for each removed URL. Best
// effort: missing dirs are silently skipped, RemoveAll errors are
// logged but don't abort the migration. Pure I/O; no global state.
func removeLegacyWorkDirs(appStorePath string, removed []string) {
	for _, rawURL := range removed {
		dir := legacyURLWorkDir(rawURL, appStorePath)
		if dir == "" {
			continue
		}
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			log.Printf("[migration] WARN failed to remove legacy workdir %q for %q: %v", dir, rawURL, err)
			continue
		}
		log.Printf("[migration] removed legacy workdir %q for %q (ADR-0038 / #450)", dir, rawURL)
	}
}
