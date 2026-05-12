package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const sampleUpstreamCompose = `services:
  nginx-proxy-manager:
    image: jc21/nginx-proxy-manager:2.14.0
    ports:
      - "81:81"
      - "80:80"
      - "443:443"
    restart: unless-stopped
`

// TestEmit_WritesComposeAtCasaOSCompatPath confirms the file
// lands at <root>/Apps/<id>/docker-compose.yml — the exact path
// `backend/app-management/service.BuildCatalog` walks.
func TestEmit_WritesComposeAtCasaOSCompatPath(t *testing.T) {
	root := t.TempDir()

	path, err := Emit(EmitContext{
		OutputRoot:     root,
		UpstreamRepo:   "https://github.com/getumbrel/umbrel-apps",
		UpstreamCommit: "abc123",
	}, UmbrelManifest{
		ID:       "nginx-proxy-manager",
		Name:     "Nginx Proxy Manager",
		Tagline:  "Expose your services easily",
		Category: "networking",
		Port:     4498,
		Developer: "Jamie Curnow",
	}, []byte(sampleUpstreamCompose), "A tool for managing Nginx proxy hosts.")

	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	want := filepath.Join(root, "Apps", "nginx-proxy-manager", "docker-compose.yml")
	if path != want {
		t.Errorf("compose path = %q, want %q", path, want)
	}
}

// TestEmit_PreservesFunctionalFacts is the legal-posture load-bearer
// (revised from Phase 4's "preserved verbatim" assertion): after
// Phase 7's Umbrel→PowerLab transform (`transform.go`, ship-bug fix
// for v0.6.1), the upstream YAML is round-tripped through
// yaml.Marshal/Unmarshal so we can drop the `app_proxy` Umbrel-runtime
// helper service and substitute `${APP_DATA_DIR}`. Formatting +
// comments are NOT preserved across the round-trip; the ADR-0024
// legal posture still holds because the FACTUAL fields (image refs,
// ports, env names, volume paths) are what's preserved — those are
// uncopyrightable. Expressive content was never imported in the
// first place (no comments to lose).
func TestEmit_PreservesFunctionalFacts(t *testing.T) {
	root := t.TempDir()
	path, err := Emit(EmitContext{
		OutputRoot:     root,
		UpstreamRepo:   "https://github.com/getumbrel/umbrel-apps",
		UpstreamCommit: "sha",
	}, UmbrelManifest{ID: "foo", Name: "Foo"}, []byte(sampleUpstreamCompose), "")
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	s := string(data)
	// Image refs survive — the most load-bearing fact for "did the
	// app actually come from the upstream maintainer's compose?".
	if !strings.Contains(s, "jc21/nginx-proxy-manager:2.14.0") {
		t.Errorf("upstream image ref not preserved, got:\n%s", s)
	}
	// Port mapping survives the transform (the sample's only
	// concrete functional fact beyond the image ref + restart).
	if !strings.Contains(s, "81:81") {
		t.Errorf("port mapping not preserved, got:\n%s", s)
	}
	// Our x-powerlab block is appended at the end
	if !strings.Contains(s, "\nx-powerlab:\n") {
		t.Error("x-powerlab block not appended")
	}
}

// TestEmit_XPowerlabBlockHasProvenance is the "debug origem"
// requirement — the source block in x-powerlab must round-trip
// through YAML and carry the upstream pointer.
func TestEmit_XPowerlabBlockHasProvenance(t *testing.T) {
	root := t.TempDir()
	path, err := Emit(EmitContext{
		OutputRoot:       root,
		UpstreamRepo:     "https://github.com/getumbrel/umbrel-apps",
		UpstreamCommit:   "abc123def456",
		TransformVersion: "1.0",
	}, UmbrelManifest{
		ID: "nginx-proxy-manager", Name: "Nginx Proxy Manager",
		Tagline: "Tag", Category: "networking", Port: 4498,
	}, []byte(sampleUpstreamCompose), "Description body.")
	if err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(path)

	// Decode just the x-powerlab block to verify shape.
	// Compose YAML has its own top-level keys (services, etc.);
	// we extract the x-powerlab section by finding its line.
	xpowerlabSection := extractXPowerlabSection(t, string(raw))

	var doc map[string]XPowerLabStoreInfo
	if err := yaml.Unmarshal([]byte(xpowerlabSection), &doc); err != nil {
		t.Fatalf("emitted x-powerlab is not parseable YAML: %v\n---\n%s", err, xpowerlabSection)
	}
	info, ok := doc["x-powerlab"]
	if !ok {
		t.Fatalf("x-powerlab key missing from emitted block")
	}

	if info.StoreAppID != "nginx-proxy-manager" {
		t.Errorf("StoreAppID = %q", info.StoreAppID)
	}
	if info.Title["en_us"] != "Nginx Proxy Manager" {
		t.Errorf("Title = %v", info.Title)
	}
	if info.Tagline["en_us"] != "Tag" {
		t.Errorf("Tagline = %v", info.Tagline)
	}
	if info.Icon != "https://getumbrel.github.io/umbrel-apps-gallery/nginx-proxy-manager/icon.svg" {
		t.Errorf("Icon = %q", info.Icon)
	}
	if info.Category != "networking" {
		t.Errorf("Category = %q", info.Category)
	}
	if info.PortMap != "4498" {
		t.Errorf("PortMap = %q", info.PortMap)
	}

	src := info.Source
	if src.Catalog != "umbrel-apps" {
		t.Errorf("Source.Catalog = %q", src.Catalog)
	}
	if src.UpstreamCommit != "abc123def456" {
		t.Errorf("Source.UpstreamCommit = %q", src.UpstreamCommit)
	}
	if src.UpstreamPath != "nginx-proxy-manager/umbrel-app.yml" {
		t.Errorf("Source.UpstreamPath = %q", src.UpstreamPath)
	}
	if src.TransformVersion != "1.0" {
		t.Errorf("Source.TransformVersion = %q", src.TransformVersion)
	}
	if src.SyncedAt == "" {
		t.Error("Source.SyncedAt empty")
	}
}

// TestEmit_WritesDescriptionMd verifies the side-file is produced
// when a description is provided AND no override exists.
func TestEmit_WritesDescriptionMd(t *testing.T) {
	root := t.TempDir()
	_, err := Emit(EmitContext{
		OutputRoot: root, UpstreamRepo: "x", UpstreamCommit: "sha",
	}, UmbrelManifest{ID: "foo", Name: "Foo"}, []byte("services: {}\n"), "the description body")
	if err != nil {
		t.Fatal(err)
	}
	desc, err := os.ReadFile(filepath.Join(root, "Apps", "foo", "description.md"))
	if err != nil {
		t.Fatalf("description.md not written: %v", err)
	}
	if !strings.Contains(string(desc), "the description body") {
		t.Errorf("description.md content unexpected: %q", string(desc))
	}
}

// TestEmit_RespectsMaintainerOverride_OnWrite confirms emit
// never overwrites a hand-curated `description-powerlab.md`.
func TestEmit_RespectsMaintainerOverride_OnWrite(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "Apps", "foo")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	overridePath := filepath.Join(appDir, "description-powerlab.md")
	if err := os.WriteFile(overridePath, []byte("MAINTAINER WRITTEN"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Emit(EmitContext{
		OutputRoot: root, UpstreamRepo: "x", UpstreamCommit: "sha",
	}, UmbrelManifest{ID: "foo", Name: "Foo"}, []byte("services: {}\n"), "auto-generated")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(appDir, "description.md")); err == nil {
		t.Error("description.md was written even though override exists")
	}
	content, _ := os.ReadFile(overridePath)
	if !strings.Contains(string(content), "MAINTAINER WRITTEN") {
		t.Error("maintainer override was overwritten")
	}
}

// TestEmit_NoDescription_SkipsWritingDescriptionFile prevents
// emitting an empty description.md when the upstream README
// fetch failed.
func TestEmit_NoDescription_SkipsWritingDescriptionFile(t *testing.T) {
	root := t.TempDir()
	_, err := Emit(EmitContext{
		OutputRoot: root, UpstreamRepo: "x", UpstreamCommit: "sha",
	}, UmbrelManifest{ID: "foo", Name: "Foo"}, []byte("services: {}\n"), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "Apps", "foo", "description.md")); err == nil {
		t.Error("description.md was written for an empty description")
	}
}

// TestIconURL_FollowsConvention locks the upstream icon URL
// pattern confirmed in the Phase 0 audit.
func TestIconURL_FollowsConvention(t *testing.T) {
	got := IconURL("nginx-proxy-manager")
	want := "https://getumbrel.github.io/umbrel-apps-gallery/nginx-proxy-manager/icon.svg"
	if got != want {
		t.Errorf("IconURL = %q, want %q", got, want)
	}
}

// extractXPowerlabSection returns the YAML chunk starting at the
// top-level `x-powerlab:` line to the end of the file. Used by
// the provenance-block test instead of unmarshaling the full
// compose YAML (which has its own keys we don't care about here).
func extractXPowerlabSection(t *testing.T, all string) string {
	t.Helper()
	idx := strings.Index(all, "\nx-powerlab:\n")
	if idx < 0 {
		t.Fatalf("no x-powerlab section found in:\n%s", all)
	}
	return all[idx+1:]
}
