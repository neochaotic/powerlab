package service_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neochaotic/powerlab/backend/app-management/service"
	"github.com/neochaotic/powerlab/backend/common/utils/logger"
)

// Integration test that locks the v0.6.1 ship bug regression:
// every Umbrel app emitted by `backend/sync-catalog/` must parse
// through `service.NewComposeAppFromYAML` so that `BuildCatalog`
// can surface it. Before Phase 7's transform fix, ALL 241 apps
// from the first weekly sync failed validation (app_proxy without
// image + un-substituted ${APP_DATA_DIR} volumes), the catalog
// returned 0 entries, and the UI silently dropped the Umbrel
// source. This test ensures any future regression — a new pattern
// upstream introduces, or a transform refactor that misses an
// invariant — surfaces in CI rather than in production again.
//
// The fixtures in testdata/umbrel-fixtures/ are representative of
// the dominant Umbrel app shapes (app_proxy + APP_DATA_DIR volume,
// multi-service with multiple substitutions, long-form volume
// declarations). Each docker-compose.yml here is the OUTPUT of the
// sync-catalog binary's transform — what BuildCatalog would
// actually read off the on-disk community-catalog/.

func TestUmbrelFixtures_AllParseThroughComposeLoader(t *testing.T) {
	logger.LogInitConsoleOnly()

	fixturesDir := filepath.Join("testdata", "umbrel-fixtures")
	appsDir := filepath.Join(fixturesDir, "Apps")

	entries, err := os.ReadDir(appsDir)
	if err != nil {
		t.Fatalf("read fixtures dir %s: %v", appsDir, err)
	}
	if len(entries) == 0 {
		t.Fatalf("no fixtures in %s — at least one representative app is required to lock the regression", appsDir)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		appID := entry.Name()
		t.Run(appID, func(t *testing.T) {
			composePath := filepath.Join(appsDir, appID, "docker-compose.yml")
			data, err := os.ReadFile(composePath)
			if err != nil {
				t.Fatalf("read %s: %v", composePath, err)
			}

			// Same flags BuildCatalog uses (skipInterpolation=false,
			// skipValidation=false). Anything that BuildCatalog rejects,
			// this test rejects.
			composeApp, err := service.NewComposeAppFromYAML(data, false, false)
			if err != nil {
				t.Fatalf("compose-go REJECTED post-transform Umbrel YAML for %q (this is exactly the v0.6.1 ship bug — see backend/sync-catalog/transform.go):\n  error: %v\n\n  ====== YAML start ======\n%s\n  ====== YAML end ======",
					appID, err, data)
			}
			if composeApp == nil {
				t.Fatalf("ComposeApp is nil for %q without error — unexpected", appID)
			}

			// Additional invariants — these mirror what the UI cares
			// about so we catch silent metadata loss too.
			services := composeApp.Services
			if len(services) == 0 {
				t.Errorf("compose app %q has zero services after parse — transform may have over-stripped", appID)
			}
			for _, svc := range services {
				if svc.Name == "app_proxy" {
					t.Errorf("compose app %q still contains app_proxy service — transform regression", appID)
				}
			}

			// Project name MUST match the directory name — this is the
			// catalog-key bug from the user's box on 2026-05-12: with-
			// out a top-level `name:` in the YAML, compose-go falls
			// back to the (random) working dir basename, and
			// `BuildCatalog` ends up keying the app under names like
			// "amazing_ubs" instead of "agent-zero". See `transform.go`.
			if composeApp.Name != appID {
				t.Errorf("compose project name = %q, want %q (BuildCatalog uses this as the catalog key — mismatch means the app is unreachable by id)",
					composeApp.Name, appID)
			}

			// No `${APP_DATA_DIR}` should remain in volume sources.
			// Env vars CAN still carry it (intentional — transform
			// only touches volumes), so we check volume bytes only.
			for _, svc := range services {
				for _, vol := range svc.Volumes {
					if strings.Contains(vol.Source, "${APP_DATA_DIR}") {
						t.Errorf("app %q service %q volume source still contains ${APP_DATA_DIR}: %q — transform regression",
							appID, svc.Name, vol.Source)
					}
				}
			}
		})
	}
}
