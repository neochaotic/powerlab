package service_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/neochaotic/powerlab/backend/app-management/service"
	"github.com/neochaotic/powerlab/backend/common/utils/logger"
	"gopkg.in/yaml.v3"
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

// shellVarRE matches `${VAR}` AND brace-less `$VAR` shell placeholders.
// Used by the "no unknown placeholder" guard below to detect upstream
// patterns we don't yet substitute. If Umbrel introduces a new
// `${UMBREL_FOO_BAR}` kind variable that lands in a volume/port spec,
// this regex catches it and the test surfaces it BEFORE the user does.
var shellVarRE = regexp.MustCompile(`\$\{[A-Z_][A-Z0-9_]*\}|\$[A-Z_][A-Z0-9_]*`)

// TestProductionCatalog_NoUnknownPlaceholdersInDangerousPositions is a
// PROACTIVE gate against Umbrel adding a new placeholder kind we don't
// know about yet. compose-go's parser will reject many such cases (and
// `TestProductionCatalog_AllParseThroughComposeLoader` will flag those),
// but some placeholder shapes pass the parser only to break later (e.g.
// a literal `$APP_FOO_DIR` left in a volume bind path — compose-go
// happily accepts it as a relative path string, but the bind mount
// fails at docker run time, opaque to CI).
//
// This test scans every emitted compose's volumes + ports positions —
// the load-bearing fields the launchpad uses — and fails if ANY shell
// placeholder remains. After the v0.6.3 round of transforms we should
// have ZERO surviving `${...}` or `$VAR` in those positions.
//
// When this test fails: it surfaces the EXACT pattern Umbrel surfaced.
// Add a substitution case to `backend/sync-catalog/transform.go`,
// re-emit (`make sync-catalog`), re-run the test. Loop closes.
func TestProductionCatalog_NoUnknownPlaceholdersInDangerousPositions(t *testing.T) {
	appsDir := filepath.Join("..", "..", "..", "community-catalog", "Apps")
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skipf("no community-catalog/Apps dir — fresh repo without sync output")
		}
		t.Fatalf("read community-catalog dir: %v", err)
	}

	// Aggregate unknowns so the failure message lists ALL of them at once.
	type unknown struct {
		app, service, position, value string
	}
	var unknowns []unknown

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		appID := entry.Name()
		composePath := filepath.Join(appsDir, appID, "docker-compose.yml")
		data, err := os.ReadFile(composePath)
		if err != nil {
			continue
		}
		var doc map[string]any
		if err := yaml.Unmarshal(data, &doc); err != nil {
			continue // covered by the other gate
		}
		services, _ := doc["services"].(map[string]any)
		for svcName, svcAny := range services {
			svc, _ := svcAny.(map[string]any)
			if svc == nil {
				continue
			}

			// volumes
			if vols, ok := svc["volumes"].([]any); ok {
				for _, v := range vols {
					switch vv := v.(type) {
					case string:
						for _, match := range shellVarRE.FindAllString(vv, -1) {
							unknowns = append(unknowns, unknown{appID, svcName, "volumes", match})
						}
					case map[string]any:
						if src, _ := vv["source"].(string); src != "" {
							for _, match := range shellVarRE.FindAllString(src, -1) {
								unknowns = append(unknowns, unknown{appID, svcName, "volumes.source", match})
							}
						}
					}
				}
			}

			// ports — strings or map-form
			if ports, ok := svc["ports"].([]any); ok {
				for _, p := range ports {
					switch pp := p.(type) {
					case string:
						for _, match := range shellVarRE.FindAllString(pp, -1) {
							unknowns = append(unknowns, unknown{appID, svcName, "ports", match})
						}
					case map[string]any:
						if pub, _ := pp["published"].(string); pub != "" {
							for _, match := range shellVarRE.FindAllString(pub, -1) {
								unknowns = append(unknowns, unknown{appID, svcName, "ports.published", match})
							}
						}
						if tgt, _ := pp["target"].(string); tgt != "" {
							for _, match := range shellVarRE.FindAllString(tgt, -1) {
								unknowns = append(unknowns, unknown{appID, svcName, "ports.target", match})
							}
						}
					}
				}
			}
		}
	}

	if len(unknowns) > 0 {
		// Group + report so a future maintainer sees the unique patterns
		// upstream introduced. Add a substitution rule to transform.go.
		uniqueVars := map[string]int{}
		for _, u := range unknowns {
			uniqueVars[u.value]++
		}
		summary := []string{}
		for v, n := range uniqueVars {
			summary = append(summary, fmt.Sprintf("  %s (%d occurrences)", v, n))
		}
		t.Errorf(`Found %d shell-var placeholder(s) in volume/port positions across %d unique pattern(s).
This is the EARLY-WARNING gate for "Umbrel introduced a new variable kind we don't substitute".
Add the missing case to backend/sync-catalog/transform.go (look at substituteAppDataDir / substitutePortPlaceholders for the pattern) and re-emit with 'make sync-catalog'.

Unique patterns found:
%s

First 5 locations:
%v`,
			len(unknowns), len(uniqueVars), strings.Join(summary, "\n"), unknowns[:min(5, len(unknowns))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
