package config

import (
	"bufio"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// curatedManifestFilename is the file written by scripts/package-linux.sh
// listing the app slugs that ship in the tarball's curated catalog. The
// boot orphan-cleanup migration uses this list as the ground truth for
// what SHOULD be present under <catalogDir>/Apps/ on a clean install.
//
// Format: one app slug per line, optional `#` comments, blank lines
// ignored. Whitespace trimmed.
//
// File is hidden (leading `.`) so it doesn't collide with the rest of
// the catalog dir layout the operator might browse.
const curatedManifestFilename = ".curated-manifest"

// pruneOrphanCuratedApps reads the bundled curated-set manifest at
// `<catalogDir>/.curated-manifest`, lists every directory under
// `<catalogDir>/Apps/`, and removes any directory not present in the
// manifest. Returns the sets it kept and removed for logging.
//
// Behaviour matrix:
//   - Manifest missing       → noop (return nil, nil, nil)
//   - Manifest empty         → noop (ambiguous: could be corruption,
//                                  could be zero-app release; safer to
//                                  skip — install.sh wipe-then-copy is
//                                  the authoritative path)
//   - Apps/ dir missing      → noop
//   - File (not dir) in Apps → ignored, never removed
//   - Dir in manifest        → kept
//   - Dir NOT in manifest    → os.RemoveAll, listed in removed
//
// Pure-ish: I/O is bounded to the catalogDir subtree. No global state.
func pruneOrphanCuratedApps(catalogDir string) (kept []string, removed []string, err error) {
	manifest, err := readCuratedManifest(filepath.Join(catalogDir, curatedManifestFilename))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	if len(manifest) == 0 {
		return nil, nil, nil
	}

	appsDir := filepath.Join(catalogDir, "Apps")
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if _, inManifest := manifest[name]; inManifest {
			kept = append(kept, name)
			continue
		}
		if err := os.RemoveAll(filepath.Join(appsDir, name)); err != nil {
			return kept, removed, err
		}
		removed = append(removed, name)
	}
	return kept, removed, nil
}

// readCuratedManifest parses the manifest file into a set of allowed
// app slugs. Empty lines and `#`-prefixed comments are skipped;
// surrounding whitespace is trimmed. Returns os.ErrNotExist when the
// file is missing so the caller can branch on the noop case.
func readCuratedManifest(path string) (map[string]struct{}, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	set := make(map[string]struct{})
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		set[line] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return set, nil
}

// MigrateOrphanCuratedApps runs the orphan cleanup against the first
// entry in ServerInfo.AppStoreList — that's the PowerLab-bundled
// curated catalog (per app-management.conf.sample `appstore =
// /var/lib/powerlab/community-catalog`). Called from InitSetup once at
// startup so a box upgraded from a PowerLab version that shipped a
// larger curated set converges back to the bundled set without
// requiring a re-install.
//
// Skips if:
//   - AppInfo / ServerInfo not yet populated (early init, test harness)
//   - AppStoreList is empty (no default catalog registered)
//   - The first AppStoreList entry is a URL (not a local path) — we
//     only own the local bundled dir; remote catalogs are operator
//     responsibility
//
// Logging mirrors MigrateAppStoreListLegacyRemoval — `[migration]`
// prefix so operators can grep startup output for migration activity.
func MigrateOrphanCuratedApps() {
	if AppInfo == nil || ServerInfo == nil {
		return
	}
	if len(ServerInfo.AppStoreList) == 0 {
		return
	}
	catalogDir := ServerInfo.AppStoreList[0]
	if catalogDir == "" || strings.HasPrefix(catalogDir, "http://") || strings.HasPrefix(catalogDir, "https://") {
		return
	}
	_, removed, err := pruneOrphanCuratedApps(catalogDir)
	if err != nil {
		log.Printf("[migration] orphan-curated cleanup failed for %q: %v", catalogDir, err)
		return
	}
	for _, name := range removed {
		log.Printf("[migration] removed orphan catalog app %q (not in curated manifest, ADR-0039 / #450)", name)
	}
}
