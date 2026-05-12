package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// AppFile is the PowerLab-native catalog entry shape that
// `backend/app-management/service` already knows how to load
// (per ADR-0021).
//
// We deliberately do NOT embed the full upstream compose YAML
// verbatim — that would be importing Umbrel's expressive content.
// Instead we extract the factual fields the runtime needs and emit
// a fresh structure with a `source` block for provenance.
type AppFile struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Tagline     string  `json:"tagline,omitempty"`
	Category    string  `json:"category,omitempty"`
	Version     string  `json:"version,omitempty"`
	Port        int     `json:"port,omitempty"`
	Icon        string  `json:"icon,omitempty"`
	Description string  `json:"description,omitempty"`
	Source      Source  `json:"source"`
}

// Source is the provenance block — answers the user-stated requirement
// "debugar de onde é a origem do docker file". A reader of a generated
// appfile.json can always trace it back to a specific upstream commit
// + file path.
type Source struct {
	Catalog          string `json:"catalog"`                     // "umbrel-apps"
	UpstreamID       string `json:"upstream_id"`                 // the id in the upstream catalog
	UpstreamRepo     string `json:"upstream_repo"`               // GitHub repo URL
	UpstreamCommit   string `json:"upstream_commit,omitempty"`   // SHA of the sync source
	UpstreamPath     string `json:"upstream_path"`               // path-within-repo of the source file
	TransformVersion string `json:"transform_version"`           // version of THIS sync binary's logic
	SyncedAt         string `json:"synced_at"`                   // ISO 8601 UTC
}

// EmitContext bundles the inputs Emit() needs from the orchestrator.
type EmitContext struct {
	// OutputRoot is the directory tree to write into. The convention
	// (per ADR-0024) is `community-catalog/apps/<id>/`.
	OutputRoot string

	// UpstreamRepo + UpstreamCommit feed the Source block. The
	// commit comes from `git rev-parse HEAD` of the cloned upstream.
	UpstreamRepo   string
	UpstreamCommit string

	// TransformVersion is what changes when filter/parser/emit logic
	// changes in a way that re-runs would produce different output.
	// Sync orchestrator hardcodes "1.0" for the initial implementation.
	TransformVersion string
}

// CurrentTransformVersion is bumped by hand whenever the sync logic
// changes in a way that warrants re-running over previously-imported
// apps. Provenance comparison vs this value tells the orchestrator
// "this app's appfile was written by an older transform — re-emit".
const CurrentTransformVersion = "1.0"

// IconURL returns the conventional Umbrel icon location for an app.
// Per Phase 0 audit + user direction: pass through to upstream,
// no re-host.
func IconURL(upstreamID string) string {
	return fmt.Sprintf("https://getumbrel.github.io/umbrel-apps-gallery/%s/icon.svg", upstreamID)
}

// Emit writes the per-app artifacts:
//   - <OutputRoot>/apps/<id>/appfile.json
//   - <OutputRoot>/apps/<id>/description.md (if non-empty and no
//     override exists; we never overwrite a maintainer's
//     description-powerlab.md)
//
// Returns the path written for the appfile, mainly for logging.
func Emit(ctx EmitContext, manifest UmbrelManifest, description string) (string, error) {
	appDir := filepath.Join(ctx.OutputRoot, "apps", manifest.ID)
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", appDir, err)
	}

	af := AppFile{
		ID:          manifest.ID,
		Name:        manifest.Name,
		Tagline:     manifest.Tagline,
		Category:    manifest.Category,
		Version:     manifest.Version,
		Port:        manifest.Port,
		Icon:        IconURL(manifest.ID),
		Description: description,
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

	jsonBytes, err := json.MarshalIndent(af, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal appfile.json: %w", err)
	}
	jsonBytes = append(jsonBytes, '\n')

	appfilePath := filepath.Join(appDir, "appfile.json")
	if err := os.WriteFile(appfilePath, jsonBytes, 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", appfilePath, err)
	}

	// description.md is written ONLY if there's no maintainer override.
	// Overwriting a maintainer's hand-curated description would be
	// data loss; the resolver's precedence (override > upstream)
	// already covers reads, but emit must also respect it on writes.
	if description != "" {
		overridePath := filepath.Join(appDir, "description-powerlab.md")
		if _, err := os.Stat(overridePath); os.IsNotExist(err) {
			descPath := filepath.Join(appDir, "description.md")
			if err := os.WriteFile(descPath, []byte(description+"\n"), 0o644); err != nil {
				return "", fmt.Errorf("write %s: %w", descPath, err)
			}
		}
	}

	return appfilePath, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
