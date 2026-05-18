package config

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"
)

// Legacy install marker (#437, Sprint 23 / ADR-0039 reframe).
//
// Apps installed BEFORE v0.7.1 came from the pre-ADR-0039 upstream
// Umbrel/CasaOS passthrough model. They continue to work but aren't
// part of the new curated catalog. To make this visible to operators,
// the boot migration scans AppsPath ONCE on the first v0.7.1 boot and
// drops a `.installed-pre-v0.7.1` marker into each existing app dir.
//
// New installs (post-v0.7.1) skip the marker — the install path writes
// nothing here, and the boot scan only runs ONCE thanks to the
// top-level sentinel `.legacy-scan-complete`. Apps that arrive after
// the sentinel exists never get tagged.
//
// The marker is consumed in a follow-up PR that surfaces a "Legacy"
// badge in the apps grid; this PR ships the plumbing only.

const (
	legacyInstallMarkerFilename = ".installed-pre-v0.7.1"
	legacyScanSentinelFilename  = ".legacy-scan-complete"
)

// markPreUpgradeAppsAsLegacy scans appsDir, identifies each subdir
// that looks like an app (any directory), and writes the legacy
// marker file unless one is already present. Already-marked apps are
// not re-touched (mtime preservation matters — the marker's mtime is
// the operator's "installed before this point" reference).
//
// Returns the count of NEWLY marked apps (excludes pre-existing).
// Missing appsDir → noop, returns (0, nil).
func markPreUpgradeAppsAsLegacy(appsDir string) (int, error) {
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	newlyMarked := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		marker := filepath.Join(appsDir, e.Name(), legacyInstallMarkerFilename)
		if _, err := os.Stat(marker); err == nil {
			continue
		}
		body := []byte("Installed before v0.7.1 (ADR-0039 curated-catalog cutover).\n" +
			"This app came from the pre-curated upstream sync model.\n" +
			"Marker written at " + time.Now().UTC().Format(time.RFC3339) + "\n")
		if err := os.WriteFile(marker, body, 0o644); err != nil {
			return newlyMarked, err
		}
		newlyMarked++
	}
	return newlyMarked, nil
}

// migrateLegacyAppMarkerAt runs the scan against a specific dir, gated
// by the top-level sentinel. Idempotent. Exposed unexported for tests
// (the production `MigrateLegacyAppMarker` resolves AppsPath from the
// global config + bails on nil).
func migrateLegacyAppMarkerAt(appsDir string) error {
	sentinel := filepath.Join(appsDir, legacyScanSentinelFilename)
	if _, err := os.Stat(sentinel); err == nil {
		return nil
	}
	if _, err := os.Stat(appsDir); errors.Is(err, os.ErrNotExist) {
		// Nothing to scan + no place to write the sentinel. Treat
		// as "migration not applicable" rather than an error.
		return nil
	}
	newlyMarked, err := markPreUpgradeAppsAsLegacy(appsDir)
	if err != nil {
		return err
	}
	// Write the sentinel so subsequent boots skip the scan even if
	// the app dir was empty on this run (operator may install apps
	// later — they must not get back-tagged).
	body := []byte("Legacy install scan completed at " + time.Now().UTC().Format(time.RFC3339) + ".\n" +
		"Apps present at scan time were marked .installed-pre-v0.7.1.\n" +
		"This file gates the scan to a single run — do not delete unless\n" +
		"intentionally re-tagging an upgraded box.\n")
	if err := os.WriteFile(sentinel, body, 0o644); err != nil {
		return err
	}
	if newlyMarked > 0 {
		log.Printf("[migration] tagged %d legacy app(s) with %s under %q (#437)", newlyMarked, legacyInstallMarkerFilename, appsDir)
	}
	return nil
}

// MigrateLegacyAppMarker is the InitSetup entry point. Resolves AppsPath
// from the global config + bails on nil for early-init / test safety.
// Failures are logged but never abort boot — the marker is a UX nicety,
// not load-bearing.
func MigrateLegacyAppMarker() {
	if AppInfo == nil || AppInfo.AppsPath == "" {
		return
	}
	if err := migrateLegacyAppMarkerAt(AppInfo.AppsPath); err != nil {
		log.Printf("[migration] WARN legacy-app marker scan failed for %q: %v", AppInfo.AppsPath, err)
	}
}
