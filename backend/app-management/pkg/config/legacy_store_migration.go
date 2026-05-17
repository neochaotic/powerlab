package config

import (
	"log"
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
}
