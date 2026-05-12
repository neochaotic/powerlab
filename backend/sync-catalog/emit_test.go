package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEmit_WritesAppfileWithProvenance is the load-bearing assertion
// for the user's "debugar de onde é a origem do docker file" requirement:
// every emitted appfile.json carries the source block with enough
// information to trace back to a specific upstream commit.
func TestEmit_WritesAppfileWithProvenance(t *testing.T) {
	root := t.TempDir()

	path, err := Emit(EmitContext{
		OutputRoot:       root,
		UpstreamRepo:     "https://github.com/getumbrel/umbrel-apps",
		UpstreamCommit:   "abc123def456",
		TransformVersion: "1.0",
	}, UmbrelManifest{
		ID:       "nginx-proxy-manager",
		Name:     "Nginx Proxy Manager",
		Tagline:  "Expose your services easily",
		Category: "networking",
		Version:  "2.14.0",
		Port:     4498,
		Repo:     "https://github.com/NginxProxyManager/nginx-proxy-manager",
	}, "A tool for managing Nginx proxy hosts.")

	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if !strings.HasSuffix(path, "/apps/nginx-proxy-manager/appfile.json") {
		t.Errorf("appfile path wrong: %q", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got AppFile
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("emitted file is not valid JSON: %v", err)
	}

	// Headline fields
	if got.ID != "nginx-proxy-manager" {
		t.Errorf("ID = %q", got.ID)
	}
	if got.Icon != "https://getumbrel.github.io/umbrel-apps-gallery/nginx-proxy-manager/icon.svg" {
		t.Errorf("Icon = %q", got.Icon)
	}
	if got.Description == "" {
		t.Error("Description empty")
	}

	// Provenance is the load-bearing bit
	src := got.Source
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
// when a description is provided.
func TestEmit_WritesDescriptionMd(t *testing.T) {
	root := t.TempDir()
	_, err := Emit(EmitContext{
		OutputRoot:     root,
		UpstreamRepo:   "https://github.com/getumbrel/umbrel-apps",
		UpstreamCommit: "sha",
	}, UmbrelManifest{ID: "foo", Name: "Foo"}, "the description body")
	if err != nil {
		t.Fatal(err)
	}

	desc, err := os.ReadFile(filepath.Join(root, "apps", "foo", "description.md"))
	if err != nil {
		t.Fatalf("description.md not written: %v", err)
	}
	if !strings.Contains(string(desc), "the description body") {
		t.Errorf("description.md content unexpected: %q", string(desc))
	}
}

// TestEmit_RespectsMaintainerOverride_OnWrite confirms we never
// overwrite a hand-curated `description-powerlab.md` even if the
// auto-fetched description would have been different.
func TestEmit_RespectsMaintainerOverride_OnWrite(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "apps", "foo")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	overridePath := filepath.Join(appDir, "description-powerlab.md")
	if err := os.WriteFile(overridePath, []byte("MAINTAINER WRITTEN — do not auto-overwrite"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Emit(EmitContext{
		OutputRoot:     root,
		UpstreamRepo:   "https://github.com/getumbrel/umbrel-apps",
		UpstreamCommit: "sha",
	}, UmbrelManifest{ID: "foo", Name: "Foo"}, "would-be auto-generated description")
	if err != nil {
		t.Fatal(err)
	}

	// description.md must NOT be created when override exists
	if _, err := os.Stat(filepath.Join(appDir, "description.md")); err == nil {
		t.Error("description.md was written even though override exists")
	}
	// Override file must be unchanged
	content, _ := os.ReadFile(overridePath)
	if !strings.Contains(string(content), "MAINTAINER WRITTEN") {
		t.Error("maintainer override was overwritten")
	}
}

// TestEmit_NoDescription_SkipsWritingDescriptionFile guards against
// emitting an empty description.md when the upstream README fetch
// failed — better to have no file than a misleading empty one.
func TestEmit_NoDescription_SkipsWritingDescriptionFile(t *testing.T) {
	root := t.TempDir()
	_, err := Emit(EmitContext{
		OutputRoot:     root,
		UpstreamRepo:   "https://github.com/getumbrel/umbrel-apps",
		UpstreamCommit: "sha",
	}, UmbrelManifest{ID: "foo", Name: "Foo"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "apps", "foo", "description.md")); err == nil {
		t.Error("description.md was written for an empty description")
	}
}

// TestIconURL_FollowsConvention locks the upstream icon URL pattern
// confirmed in the Phase 0 audit. If Umbrel reorganises and we need
// to flip to the rehost escape hatch, this test is the first surface
// to fail.
func TestIconURL_FollowsConvention(t *testing.T) {
	got := IconURL("nginx-proxy-manager")
	want := "https://getumbrel.github.io/umbrel-apps-gallery/nginx-proxy-manager/icon.svg"
	if got != want {
		t.Errorf("IconURL = %q, want %q", got, want)
	}
}
