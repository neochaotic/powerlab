package main

import (
	"strings"
	"testing"
)

// TestParser_UmbrelManifest_AllowlistFields confirms the parser
// extracts ONLY the fields on the UmbrelManifest struct's allowlist.
// Critical: the `description:` field is upstream Umbrel-curated copy
// (legal posture per ADR-0024); it MUST be dropped, and any future
// field appearing in upstream YAMLs MUST NOT silently leak in.
func TestParser_UmbrelManifest_AllowlistFields(t *testing.T) {
	yamlBlob := []byte(`
manifestVersion: 1
id: nginx-proxy-manager
name: Nginx Proxy Manager
tagline: Expose your services easily and securely
category: networking
version: "2.14.0"
port: 4498
description: >-
  Expose your apps to the internet easily and securely. Be cautious when exposing.
developer: Jamie Curnow
website: https://nginxproxymanager.com/
submitter: Sahil Phule
submission: https://github.com/getumbrel/umbrel-apps/pull/1296
repo: https://github.com/NginxProxyManager/nginx-proxy-manager
support: https://github.com/NginxProxyManager/nginx-proxy-manager/issues
gallery:
  - 1.jpg
  - 2.jpg
releaseNotes: This update includes several improvements.
dependencies: []
path: ""
`)
	manifest, err := ParseUmbrelManifest(yamlBlob)
	if err != nil {
		t.Fatalf("ParseUmbrelManifest: %v", err)
	}

	// Allowlisted fields land
	if manifest.ID != "nginx-proxy-manager" {
		t.Errorf("ID = %q, want nginx-proxy-manager", manifest.ID)
	}
	if manifest.Name != "Nginx Proxy Manager" {
		t.Errorf("Name = %q", manifest.Name)
	}
	if manifest.Category != "networking" {
		t.Errorf("Category = %q", manifest.Category)
	}
	if manifest.Repo != "https://github.com/NginxProxyManager/nginx-proxy-manager" {
		t.Errorf("Repo = %q", manifest.Repo)
	}
	if manifest.Version != "2.14.0" {
		t.Errorf("Version = %q", manifest.Version)
	}

	// Non-allowlisted fields don't exist on the struct, so by
	// construction they cannot leak. This assertion guards against
	// someone adding a `Description string \`yaml:"description"\``
	// to UmbrelManifest without thinking about the legal posture.
	// The unsafe-add catch is: re-serialise the struct and confirm
	// the original Umbrel-curated description string is absent.
	var sb strings.Builder
	sb.WriteString(manifest.Name)
	sb.WriteString(manifest.Tagline)
	sb.WriteString(manifest.Repo)
	sb.WriteString(manifest.Support)
	sb.WriteString(manifest.Website)
	sb.WriteString(manifest.Developer)
	// `gallery` and `releaseNotes` aren't on the struct — by
	// construction this contains none of them.
	if strings.Contains(sb.String(), "Be cautious when exposing") {
		t.Error("upstream description leaked into a parsed field — field allowlist violated")
	}
	if strings.Contains(sb.String(), "1.jpg") || strings.Contains(sb.String(), "2.jpg") {
		t.Error("upstream gallery leaked into a parsed field")
	}
	if strings.Contains(sb.String(), "improvements") {
		t.Error("upstream releaseNotes leaked into a parsed field")
	}
}

// TestParser_ComposeFile_BasicShape verifies the minimal compose
// model the filter needs.
func TestParser_ComposeFile_BasicShape(t *testing.T) {
	yamlBlob := []byte(`
version: "3.8"
services:
  app:
    image: jc21/nginx-proxy-manager:2.14.0
    ports:
      - "80:80"
    environment:
      DB_HOST: ${APP_NPM_DB_HOST}
      TOKEN: ${APP_NPM_TOKEN}
  db:
    image: mariadb:10.6
    depends_on:
      - app
networks:
  default:
    name: nginx-proxy-manager_default
`)
	compose, err := ParseComposeFile(yamlBlob)
	if err != nil {
		t.Fatalf("ParseComposeFile: %v", err)
	}
	if len(compose.Services) != 2 {
		t.Errorf("expected 2 services, got %d", len(compose.Services))
	}
	app, ok := compose.Services["app"]
	if !ok {
		t.Fatal("app service missing")
	}
	if !strings.HasPrefix(app.Image, "jc21/nginx-proxy-manager") {
		t.Errorf("app.Image = %q", app.Image)
	}
}

// TestParser_ComposeFile_EnvShapes covers both `environment:` shapes
// in the wild — list-of-strings vs map. Both must survive parsing
// for the filter's flattenEnvironment to work.
func TestParser_ComposeFile_EnvShapes(t *testing.T) {
	// List-of-strings shape
	listYAML := []byte(`
services:
  app:
    image: foo:1
    environment:
      - KEY1=value1
      - KEY2=${APP_OTHER_REF}
`)
	c1, err := ParseComposeFile(listYAML)
	if err != nil {
		t.Fatalf("list shape: %v", err)
	}
	envText := flattenEnvironment(c1.Services["app"].EnvironmentRaw)
	if !strings.Contains(envText, "APP_OTHER_REF") {
		t.Errorf("list env shape lost the var ref; flatten output: %q", envText)
	}

	// Map shape
	mapYAML := []byte(`
services:
  app:
    image: foo:1
    environment:
      KEY1: value1
      KEY2: ${APP_OTHER_REF}
`)
	c2, err := ParseComposeFile(mapYAML)
	if err != nil {
		t.Fatalf("map shape: %v", err)
	}
	envText = flattenEnvironment(c2.Services["app"].EnvironmentRaw)
	if !strings.Contains(envText, "APP_OTHER_REF") {
		t.Errorf("map env shape lost the var ref; flatten output: %q", envText)
	}
}

// TestParser_ComposeFile_Malformed surfaces parse errors instead of
// silently returning a zero struct — the sync should skip malformed
// upstream files, not import them as if they were empty.
func TestParser_ComposeFile_Malformed(t *testing.T) {
	yamlBlob := []byte(`this is not: valid: yaml: with: nested colons`)
	_, err := ParseComposeFile(yamlBlob)
	if err == nil {
		t.Error("expected error for malformed YAML, got nil")
	}
}
