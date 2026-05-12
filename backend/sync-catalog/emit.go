package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Layout: `<OutputRoot>/Apps/<id>/docker-compose.yml`
// (capital A in Apps, kebab-case id from upstream verbatim).
//
// Why this shape: `backend/app-management/service.BuildCatalog`
// walks `<storeRoot>/Apps/<dir>/docker-compose.yml` and reads
// a single top-level `x-powerlab:` (or `x-casaos:` / `x-web:` —
// see ADR-0021 + ADR-0025-renumbered) for store-info metadata
// (title, tagline, icon, category, port_map, …). Phase 1's
// original `appfile.json` shape was a misread of ADR-0021 by me;
// the CasaOS-compatible shape is what the rest of the codebase
// already speaks. Phase 4 corrects this.
const (
	appsDirectoryName    = "Apps"
	composeYAMLFileName  = "docker-compose.yml"
	descriptionFileName  = "description.md"
	descriptionOverride  = "description-powerlab.md"
	xPowerlabBlockHeader = "\nx-powerlab:\n"
)

// XPowerLabStoreInfo mirrors the subset of
// `codegen.ComposeAppStoreInfo` the app-management loader reads
// off a catalog entry. We hand-write rather than import the
// codegen struct to avoid a heavy cross-module dependency from
// the sync binary's own go.mod into the app-management module.
//
// Schema is intentionally short — fields we don't have safe
// sources for are omitted. `architectures`, `scheme`, `tips`,
// `screenshot_link` stay empty until/unless we surface concrete
// per-app data (legal posture: we don't import the upstream
// gallery field, so no screenshots).
type XPowerLabStoreInfo struct {
	StoreAppID  string            `yaml:"store_app_id"`
	Title       map[string]string `yaml:"title"`
	Tagline     map[string]string `yaml:"tagline,omitempty"`
	Description map[string]string `yaml:"description,omitempty"`
	Icon        string            `yaml:"icon,omitempty"`
	Category    string            `yaml:"category,omitempty"`
	PortMap     string            `yaml:"port_map,omitempty"`
	Author      string            `yaml:"author,omitempty"`
	Developer   string            `yaml:"developer,omitempty"`
	Main        string            `yaml:"main,omitempty"`

	// Source is the provenance block — answers the user-stated
	// "debug origem" requirement. Reading the catalog entry on
	// disk tells you which upstream commit produced it.
	Source Source `yaml:"source"`
}

// Source is the provenance block embedded in x-powerlab.
type Source struct {
	Catalog          string `yaml:"catalog"`                     // "umbrel-apps"
	UpstreamID       string `yaml:"upstream_id"`                 // the id in the upstream catalog
	UpstreamRepo     string `yaml:"upstream_repo"`               // GitHub repo URL
	UpstreamCommit   string `yaml:"upstream_commit,omitempty"`   // SHA of the sync source
	UpstreamPath     string `yaml:"upstream_path"`               // path-within-repo of the source file
	TransformVersion string `yaml:"transform_version"`           // version of THIS sync binary's logic
	SyncedAt         string `yaml:"synced_at"`                   // ISO 8601 UTC
}

// EmitContext bundles the inputs Emit() needs from the orchestrator.
type EmitContext struct {
	OutputRoot       string
	UpstreamRepo     string
	UpstreamCommit   string
	TransformVersion string
}

// CurrentTransformVersion is bumped by hand whenever the sync
// logic changes in a way that warrants re-running over previously-
// imported apps.
const CurrentTransformVersion = "1.0"

// IconURL returns the conventional Umbrel icon location for an app.
// Per Phase 0 audit + user direction (2026-05-11): pass through to
// upstream, no re-host.
func IconURL(upstreamID string) string {
	return fmt.Sprintf("https://getumbrel.github.io/umbrel-apps-gallery/%s/icon.svg", upstreamID)
}

// Emit writes the per-app artifacts:
//   - <OutputRoot>/Apps/<id>/docker-compose.yml — upstream YAML
//     verbatim, plus a single top-level `x-powerlab:` block
//     containing store-info + source provenance
//   - <OutputRoot>/Apps/<id>/description.md — sidecar (UI may
//     show; loader doesn't need); skipped if a
//     description-powerlab.md maintainer override exists
//
// The upstream docker-compose.yml is appended verbatim because
// compose YAML is functional config (image refs, ports, env
// names, volume paths — all factual, not expressive). The legal
// posture from ADR-0024 still holds; the only expressive content
// (descriptions, screenshots) was already dropped at the parser
// stage.
//
// Returns the compose file path, mainly for logging.
func Emit(ctx EmitContext, manifest UmbrelManifest, upstreamCompose []byte, description string) (string, error) {
	appDir := filepath.Join(ctx.OutputRoot, appsDirectoryName, manifest.ID)
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", appDir, err)
	}

	info := XPowerLabStoreInfo{
		StoreAppID: manifest.ID,
		Title:      map[string]string{"en_us": manifest.Name},
		Tagline:    nilIfEmptyMap("en_us", manifest.Tagline),
		Icon:       IconURL(manifest.ID),
		Category:   manifest.Category,
		Author:     manifest.Developer,
		Developer:  manifest.Developer,
		Source: Source{
			Catalog:          "umbrel-apps",
			UpstreamID:       manifest.ID,
			UpstreamRepo:     ctx.UpstreamRepo,
			UpstreamCommit:   ctx.UpstreamCommit,
			UpstreamPath:     fmt.Sprintf("%s/umbrel-app.yml", manifest.ID),
			TransformVersion: firstNonEmpty(ctx.TransformVersion, CurrentTransformVersion),
			SyncedAt:         time.Now().UTC().Format(time.RFC3339),
		},
	}
	if manifest.Port > 0 {
		info.PortMap = fmt.Sprintf("%d", manifest.Port)
	}
	if description != "" {
		info.Description = map[string]string{"en_us": description}
	}

	// Apply Umbrel→PowerLab compose transform before stamping the
	// x-powerlab block. Without this, the upstream YAML carries an
	// `app_proxy` service with no image and `${APP_DATA_DIR}`
	// volume references that compose-go's validator rejects — the
	// catalog walker would silently drop every imported app.
	// See `transform.go` for the rationale + `transform_test.go`
	// for the regression locks; this was the v0.6.1 ship bug
	// caught when the first weekly sync produced 241 unparseable
	// composes (CHANGELOG fragment fix-307-umbrel-compose-validity).
	transformed, err := transformUpstreamCompose(upstreamCompose, manifest.ID, manifest.Port)
	if err != nil {
		return "", fmt.Errorf("transform upstream compose for %s: %w", manifest.ID, err)
	}

	// Marshal the x-powerlab block separately and append. The
	// upstream compose has been YAML-round-tripped by transform
	// (formatting + comments lost — acceptable since the file is
	// machine-emitted and machine-read; maintainer-curated content
	// goes in `description-powerlab.md` sidecars).
	blockYAML, err := yaml.Marshal(map[string]XPowerLabStoreInfo{"x-powerlab": info})
	if err != nil {
		return "", fmt.Errorf("marshal x-powerlab: %w", err)
	}

	out := bytesEnsureTrailingNewline(transformed)
	out = append(out, '\n')
	out = append(out, blockYAML...)

	composePath := filepath.Join(appDir, composeYAMLFileName)
	if err := os.WriteFile(composePath, out, 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", composePath, err)
	}

	// description.md is written ONLY if there's no maintainer
	// override. Overwriting hand-curated copy would be data
	// loss; the description-powerlab.md override is read by the
	// resolver and takes precedence.
	if description != "" {
		overridePath := filepath.Join(appDir, descriptionOverride)
		if _, err := os.Stat(overridePath); os.IsNotExist(err) {
			descPath := filepath.Join(appDir, descriptionFileName)
			if err := os.WriteFile(descPath, []byte(description+"\n"), 0o644); err != nil {
				return "", fmt.Errorf("write %s: %w", descPath, err)
			}
		}
	}

	return composePath, nil
}

// bytesEnsureTrailingNewline returns b with at least one trailing
// '\n' so the appended `x-powerlab:` block lands on its own line.
func bytesEnsureTrailingNewline(b []byte) []byte {
	if len(b) == 0 {
		return []byte{'\n'}
	}
	if b[len(b)-1] != '\n' {
		return append(b, '\n')
	}
	return b
}

// nilIfEmptyMap returns nil when value is empty; otherwise a
// single-entry map keyed by locale. Keeps the emitted YAML from
// carrying empty `tagline:` keys.
func nilIfEmptyMap(locale, value string) map[string]string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return map[string]string{locale: value}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
