package service_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neochaotic/powerlab/backend/app-management/service"
	"github.com/neochaotic/powerlab/backend/common/utils/logger"
)

// CI gate: every `community-catalog/Apps/<id>/docker-compose.yml`
// in the repo MUST parse through `NewComposeAppFromYAML` —
// the same loader BuildCatalog uses at runtime. If any app fails,
// CI red-fails the build, the tag can't be pushed, the broken
// catalog never makes it into a release tarball.
//
// This test is the "v0.6.2 lesson": that release shipped a
// Phase-7-fix-binary together with a v0.6.1-emit-catalog tarball.
// Binary could parse Umbrel apps, but the on-disk YAMLs in the
// tarball still carried the un-transformed broken content. On
// upgrade, `install.sh`'s `cp -R community-catalog/` overwrote
// the user's working catalog with the broken tarball state, and
// the catalog returned 162 apps (CasaOS-only) instead of 336.
//
// Why integration_test wasn't enough: it covers a few hand-
// crafted fixtures. This test scans the ACTUAL production
// catalogue in the repo — every PR that touches community-
// catalog/ runs the same loader the user's PowerLab will run.
// Coverage is structural, not sample-based.
//
// Skip rules: if community-catalog/Apps/ is missing or contains
// only .gitkeep (fresh repo before first sync), skip. This is
// a CI gate against regressions, not a "must always have apps"
// test — that would force every fresh checkout to run a sync
// before running tests.

func TestProductionCatalog_AllParseThroughComposeLoader(t *testing.T) {
	logger.LogInitConsoleOnly()

	// Walk up to repo root then into community-catalog/Apps/.
	// The test runs from backend/app-management/service so the
	// relative path is `../../../community-catalog/Apps`.
	appsDir := filepath.Join("..", "..", "..", "community-catalog", "Apps")

	entries, err := os.ReadDir(appsDir)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skipf("no community-catalog/Apps dir — fresh repo without sync output (path: %s)", appsDir)
		}
		t.Fatalf("read community-catalog dir %s: %v", appsDir, err)
	}

	// Filter to actual app dirs (skip .gitkeep, hidden files)
	appDirs := []os.DirEntry{}
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			appDirs = append(appDirs, e)
		}
	}
	if len(appDirs) == 0 {
		t.Skipf("community-catalog/Apps is empty (only .gitkeep?) — fresh repo before first sync")
	}

	failed := 0
	for _, entry := range appDirs {
		appID := entry.Name()
		t.Run(appID, func(t *testing.T) {
			composePath := filepath.Join(appsDir, appID, "docker-compose.yml")
			data, err := os.ReadFile(composePath)
			if err != nil {
				t.Skipf("missing docker-compose.yml for %s — empty app dir?", appID)
				return
			}

			// SAME flags as `BuildCatalog` uses in appstore.go:486.
			// SkipInterpolation=true mirrors production; SkipValidation
			// stays false so we catch the same class of errors BuildCatalog
			// catches.
			composeApp, err := service.NewComposeAppFromYAML(data, true, false)
			if err != nil {
				failed++
				t.Errorf("PRODUCTION catalog YAML for %q FAILED to parse — this WILL ship as a broken app in the next release tarball:\n  error: %v\n  path: %s",
					appID, err, composePath)
				return
			}
			if composeApp == nil {
				t.Errorf("ComposeApp is nil for %q without error", appID)
				return
			}
			if composeApp.Name != appID {
				t.Errorf("compose project name = %q, want %q — BuildCatalog will key under wrong id, app unreachable in UI",
					composeApp.Name, appID)
			}
		})
	}

	if failed > 0 {
		t.Logf("CI gate failed: %d/%d apps in community-catalog/ would not surface in the store. Run `make sync-catalog` to refresh, or fix backend/sync-catalog/transform.go if a new upstream pattern appeared.",
			failed, len(appDirs))
	}
}
