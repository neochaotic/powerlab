package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/neochaotic/powerlab/backend/app-management/common"
)

// AppData migration logic for ADR-0021. The pre-PR layout shared
// <StoragePath>/AppData/<app>/ between PowerLab and any other product
// using the same convention (notably CasaOS, which we're forking
// from). The post-PR layout is <StoragePath>/PowerLabAppData/<app>/.
//
// The migration runs once at app-management startup. It is
// intentionally conservative: only directories whose name matches a
// PowerLab compose project (i.e. <appsPath>/<name>/docker-compose.yml
// exists) get moved. Any directory in AppData/ that PowerLab has no
// compose project for is left alone — it might belong to CasaOS, an
// abandoned PowerLab install of an app since uninstalled, or
// something the user placed there by hand. Manual review is safer
// than automatic move.
//
// Source preservation: the legacy directory is `mv`'d, not `cp`'d.
// This mirrors how systemd `mv` is atomic on same-filesystem moves
// (and AppData is always on /DATA, same FS). Atomic move means no
// "half-migrated" intermediate state — either the migration ran for
// app X or it didn't. After a successful move the legacy path no
// longer exists, so subsequent runs of MigrateAppData are no-ops
// (the discovery phase finds nothing to migrate).
//
// If a destination already exists (the canonical PowerLabAppData/X
// is already populated, e.g. user manually moved data), the legacy
// is preserved as <legacy>.bak.<unix-ts> and a non-fatal error
// returned in MigrationResult. Operator can then compare and clean
// up; we never auto-overwrite user data.

// AppDataMigrationResult is one result-per-app entry returned by
// MigrateAppData. Action is one of: "migrated", "skipped-no-project",
// "skipped-canonical-exists", "skipped-no-legacy", "error".
type AppDataMigrationResult struct {
	AppName  string
	Action   string
	Legacy   string
	Canonical string
	Backup   string // populated only when Action == "skipped-canonical-exists"
	Err      error  // populated only when Action == "error"
}

// MigrateAppData runs the on-boot AppData migration. Discovers
// directories under <storagePath>/AppData/ that have a matching
// PowerLab compose project under <appsPath>/, then moves each to
// <storagePath>/PowerLabAppData/.
//
// Returns one AppDataMigrationResult per directory considered (NOT
// per directory moved — skips are first-class, observable results).
// The migration NEVER returns a top-level error: every per-app
// failure is reported as Action: "error" so a single broken app
// doesn't block migration of the rest. Caller should log the
// returned results.
//
// nil-safe behaviour: missing storagePath or missing AppData/ both
// return an empty result (nothing to migrate, no error).
func MigrateAppData(storagePath, appsPath string) []AppDataMigrationResult {
	legacyRoot := filepath.Join(storagePath, common.LegacyAppDataDirName)
	canonicalRoot := filepath.Join(storagePath, common.AppDataDirName)

	entries, err := os.ReadDir(legacyRoot)
	if err != nil {
		// no legacy dir → nothing to do (fresh install or already
		// migrated)
		return nil
	}

	var results []AppDataMigrationResult
	for _, entry := range entries {
		if !entry.IsDir() {
			continue // skip stray files in AppData/
		}
		name := entry.Name()
		legacy := filepath.Join(legacyRoot, name)
		canonical := filepath.Join(canonicalRoot, name)

		// Heuristic: PowerLab manages this app iff a compose project
		// exists at <appsPath>/<name>/docker-compose.yml or .yaml.
		if !powerlabManagesApp(appsPath, name) {
			results = append(results, AppDataMigrationResult{
				AppName: name, Action: "skipped-no-project",
				Legacy: legacy, Canonical: canonical,
			})
			continue
		}

		// If canonical already exists with content, don't auto-merge.
		// Move legacy aside as .bak.<ts> so operator can review.
		if pathExistsNonEmpty(canonical) {
			backup := fmt.Sprintf("%s.bak.%d", legacy, nowUnix())
			if err := os.Rename(legacy, backup); err != nil {
				results = append(results, AppDataMigrationResult{
					AppName: name, Action: "error",
					Legacy: legacy, Canonical: canonical,
					Err: fmt.Errorf("canonical exists; failed to back up legacy: %w", err),
				})
				continue
			}
			results = append(results, AppDataMigrationResult{
				AppName: name, Action: "skipped-canonical-exists",
				Legacy: legacy, Canonical: canonical, Backup: backup,
			})
			continue
		}

		// Make sure canonical parent exists; mkdir -p semantics.
		if err := os.MkdirAll(canonicalRoot, 0o755); err != nil {
			results = append(results, AppDataMigrationResult{
				AppName: name, Action: "error",
				Legacy: legacy, Canonical: canonical,
				Err: fmt.Errorf("create canonical root: %w", err),
			})
			continue
		}

		if err := os.Rename(legacy, canonical); err != nil {
			results = append(results, AppDataMigrationResult{
				AppName: name, Action: "error",
				Legacy: legacy, Canonical: canonical,
				Err: fmt.Errorf("move legacy → canonical: %w", err),
			})
			continue
		}

		results = append(results, AppDataMigrationResult{
			AppName: name, Action: "migrated",
			Legacy: legacy, Canonical: canonical,
		})
	}
	return results
}

// powerlabManagesApp returns true if <appsPath>/<name>/docker-compose.{yml,yaml}
// exists. The two extensions cover both upstream-AppStore convention
// (.yml) and a few apps that ship .yaml.
func powerlabManagesApp(appsPath, name string) bool {
	for _, ext := range []string{".yml", ".yaml"} {
		composeFile := filepath.Join(appsPath, name, "docker-compose"+ext)
		st, err := os.Stat(composeFile)
		if err == nil && !st.IsDir() {
			return true
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			// permission issue or worse — treat as "we can't tell";
			// safer to skip than to migrate the wrong dir
			return false
		}
	}
	return false
}

// pathExistsNonEmpty returns true if path exists and (for directories)
// has at least one entry. Empty directories at canonical are treated
// as "not really there" — the migration overwrites them.
func pathExistsNonEmpty(path string) bool {
	st, err := os.Stat(path)
	if err != nil {
		return false
	}
	if !st.IsDir() {
		return st.Size() > 0
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return true // exists but unreadable → conservative: treat as occupied
	}
	return len(entries) > 0
}

// nowUnix is a seam for tests; defaults to real time.Now().Unix().
var nowUnix = func() int64 { return realNowUnix() }
