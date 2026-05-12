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

// TestEmit_PortMapAlignsWithComposePort is the regression lock for
// the v0.6.3 click-through bug: when an app's compose has
// `${APP_FOO_PORT}` placeholders that get substituted to e.g.
// 18000+ by transform.go, but the `x-powerlab.port_map` metadata
// still carries the upstream manifest's `port:` field (e.g. 8788),
// they DISAGREE. The PowerLab UI launchpad uses port_map to build
// the click-through URL (`http://host:8788/`) — that URL hits
// nothing because the container is listening on 18000.
//
// The contract this test locks: after Emit, the host-side port in
// the FIRST `services.<main>.ports` entry MUST equal what's written
// to `x-powerlab.port_map`. If they ever drift apart again, the
// "open app" tile click will silently break for every Umbrel app.
func TestEmit_PortMapAlignsWithComposePort(t *testing.T) {
	root := t.TempDir()

	// Realistic upstream shape: ports section uses an Umbrel-style
	// placeholder + manifest declares the real port.
	upstream := []byte(`services:
  web:
    image: example/app:latest
    ports:
      - "${APP_FOO_PORT}:80"
`)
	const manifestPort = 8788

	composePath, err := Emit(EmitContext{
		OutputRoot:     root,
		UpstreamRepo:   "https://github.com/getumbrel/umbrel-apps",
		UpstreamCommit: "test",
	}, UmbrelManifest{
		ID:   "foo",
		Name: "Foo",
		Port: manifestPort,
	}, upstream, "")
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}

	data, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read emitted file: %v", err)
	}

	// Parse the emitted YAML to inspect both fields.
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("emitted file is not valid YAML: %v", err)
	}

	// 1. x-powerlab.port_map carries the manifest port
	xp, _ := doc["x-powerlab"].(map[string]any)
	portMap, _ := xp["port_map"].(string)
	wantPortMap := "8788"
	if portMap != wantPortMap {
		t.Errorf("x-powerlab.port_map = %q, want %q (the manifest's port)", portMap, wantPortMap)
	}

	// 2. services.web.ports[0] host-side port matches port_map
	services, _ := doc["services"].(map[string]any)
	web, _ := services["web"].(map[string]any)
	ports, _ := web["ports"].([]any)
	if len(ports) == 0 {
		t.Fatal("expected at least one port in services.web.ports")
	}
	firstPort, _ := ports[0].(string)
	if !strings.HasPrefix(firstPort, wantPortMap+":") {
		t.Errorf("services.web.ports[0] = %q, want host-side port %s (matching x-powerlab.port_map). When these disagree, the PowerLab UI launchpad builds the wrong URL for the click-through and the app appears to 'open in new window but never load' — the v0.6.3 ship bug.",
			firstPort, wantPortMap)
	}
}

// TestEmit_ExposesPortWhenUpstreamOnlyUsedAppProxy is the regression
// lock for a v0.6.3 click-through bug visible on apps like `enclosed`:
// Umbrel's `app_proxy` service was the ONLY mechanism exposing the
// inner service's port (no `ports:` section in the real service). When
// transform.go strips app_proxy, the container becomes
// internal-only — the launchpad's click-through URL points at
// `host:port_map` but the host has nothing listening because no port
// mapping survived.
//
// The contract this test locks: after Emit on a compose where the
// real service has NO `ports:` section but app_proxy declared the
// internal target via `APP_PORT`, the emitted compose must add a
// `ports: ["<manifest.Port>:<APP_PORT>"]` mapping to the target
// service so the host port the UI uses actually reaches the container.
func TestEmit_ExposesPortWhenUpstreamOnlyUsedAppProxy(t *testing.T) {
	root := t.TempDir()

	// Realistic enclosed-style shape: real service has NO ports; the
	// only port-routing signal is app_proxy's APP_HOST + APP_PORT.
	upstream := []byte(`services:
  app_proxy:
    environment:
      APP_HOST: enclosed_web_1
      APP_PORT: 8080
  web:
    image: example/enclosed:1.0
    volumes:
      - ${APP_DATA_DIR}/data:/app/data
`)

	composePath, err := Emit(EmitContext{
		OutputRoot:     root,
		UpstreamRepo:   "https://github.com/getumbrel/umbrel-apps",
		UpstreamCommit: "test",
	}, UmbrelManifest{
		ID:   "enclosed",
		Name: "Enclosed",
		Port: 8788, // the manifest's port: field — what x-powerlab.port_map gets
	}, upstream, "")
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}

	data, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}

	services, _ := doc["services"].(map[string]any)
	web, _ := services["web"].(map[string]any)
	ports, _ := web["ports"].([]any)
	if len(ports) == 0 {
		t.Fatalf("expected services.web.ports to be added from app_proxy info, got none — the launchpad click-through URL will point at port_map=8788 but the container will have no host port mapping. Compose:\n%s", data)
	}
	first, _ := ports[0].(string)
	want := "8788:8080"
	if first != want {
		t.Errorf("services.web.ports[0] = %q, want %q (manifest.Port : app_proxy.APP_PORT)", first, want)
	}
}

// TestEmit_ExposesPortWhenAppHostMatchesHostname covers the case
// where Umbrel's `app_proxy.APP_HOST` uses the target service's
// `hostname:` field (e.g. cloudflared's `APP_HOST: cloudflared-web`
// matching `services.web.hostname: cloudflared-web`) rather than the
// default `<storeAppID>_<svc>_<replica>` convention. Without this,
// the audit on 2026-05-12 surfaced cloudflared + searxng as silently
// portless after Phase 8 — visible in the catalog, click-through
// pointing at nothing.
func TestEmit_ExposesPortWhenAppHostMatchesHostname(t *testing.T) {
	root := t.TempDir()
	upstream := []byte(`services:
  app_proxy:
    environment:
      APP_HOST: cloudflared-web
      APP_PORT: 3000
  web:
    image: example/cloudflared:1.0
    hostname: cloudflared-web
`)
	composePath, err := Emit(EmitContext{OutputRoot: root, UpstreamRepo: "x", UpstreamCommit: "y"},
		UmbrelManifest{ID: "cloudflared", Name: "Cloudflared", Port: 4499}, upstream, "")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(composePath)
	var doc map[string]any
	yaml.Unmarshal(data, &doc)
	services, _ := doc["services"].(map[string]any)
	web, _ := services["web"].(map[string]any)
	ports, _ := web["ports"].([]any)
	if len(ports) == 0 || ports[0] != "4499:3000" {
		t.Errorf("expected services.web.ports[0]='4499:3000', got %v. APP_HOST=hostname match was not honored.", ports)
	}
}

// TestEmit_ExposesPortWhenAppHostMatchesContainerName covers
// `container_name:` resolution (searxng-style — APP_HOST resolves
// against `services.web.container_name: searxng-web`).
func TestEmit_ExposesPortWhenAppHostMatchesContainerName(t *testing.T) {
	root := t.TempDir()
	upstream := []byte(`services:
  app_proxy:
    environment:
      APP_HOST: searxng-web
      APP_PORT: 8080
  web:
    image: example/searxng:1
    container_name: searxng-web
`)
	composePath, err := Emit(EmitContext{OutputRoot: root, UpstreamRepo: "x", UpstreamCommit: "y"},
		UmbrelManifest{ID: "searxng", Name: "Searxng", Port: 8182}, upstream, "")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(composePath)
	var doc map[string]any
	yaml.Unmarshal(data, &doc)
	services, _ := doc["services"].(map[string]any)
	web, _ := services["web"].(map[string]any)
	ports, _ := web["ports"].([]any)
	if len(ports) == 0 || ports[0] != "8182:8080" {
		t.Errorf("container_name match should produce ports['8182:8080'], got %v", ports)
	}
}

// TestEmit_ExposesPortWhenAppHostIsShellVar covers Umbrel's
// `APP_HOST: $APP_AGORA_IP` form — a shell var that resolves at
// install time. We can't resolve it at sync time but we can fall
// back to the first non-proxy service (most Umbrel apps with this
// pattern have a single "main" service anyway).
func TestEmit_ExposesPortWhenAppHostIsShellVar(t *testing.T) {
	root := t.TempDir()
	upstream := []byte(`services:
  app_proxy:
    environment:
      APP_HOST: $APP_AGORA_IP
      APP_PORT: 80
  agora:
    image: example/agora:1
`)
	composePath, err := Emit(EmitContext{OutputRoot: root, UpstreamRepo: "x", UpstreamCommit: "y"},
		UmbrelManifest{ID: "agora", Name: "Agora", Port: 12080}, upstream, "")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(composePath)
	var doc map[string]any
	yaml.Unmarshal(data, &doc)
	services, _ := doc["services"].(map[string]any)
	agora, _ := services["agora"].(map[string]any)
	ports, _ := agora["ports"].([]any)
	if len(ports) == 0 || ports[0] != "12080:80" {
		t.Errorf("shell-var APP_HOST should fall back to first non-proxy svc; expected ports['12080:80'], got %v", ports)
	}
}

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
