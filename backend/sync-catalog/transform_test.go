package main

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The transforms here are the regression lock for the v0.6.1 bug
// where 241 Umbrel apps emitted in the first weekly sync ALL failed
// to parse in `app-management.service.BuildCatalog` because the
// upstream compose YAML assumed an Umbrel runtime that we don't
// replicate. Two root causes — `services.app_proxy` without an
// image, and `${APP_DATA_DIR}` un-substituted in volume references.
// Each is locked by a dedicated test below; a round-trip test
// asserts the combined transform produces YAML that the compose-go
// validator accepts (see `app-management/service/appstore_catalog_integration_test.go`
// for the loader-level lock, run as a separate CI step).

func TestStripAppProxyService(t *testing.T) {
	in := []byte(`version: '3.7'
services:
  app_proxy:
    environment:
      APP_HOST: foo_app_1
      APP_PORT: 80
  app:
    image: nginx:latest
    restart: on-failure
`)
	out, err := transformUpstreamCompose(in, "foo")
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	if strings.Contains(string(out), "app_proxy") {
		t.Errorf("expected app_proxy service removed, got:\n%s", out)
	}
	// The real service must survive
	if !strings.Contains(string(out), "image: nginx:latest") {
		t.Errorf("expected real service preserved, got:\n%s", out)
	}
}

func TestSubstituteAppDataDirInStringVolumes(t *testing.T) {
	in := []byte(`version: '3.7'
services:
  web:
    image: nginx:latest
    volumes:
      - ${APP_DATA_DIR}/data:/app/data
      - ${APP_DATA_DIR}/config:/app/config:rw
      - /tmp:/tmp
`)
	out, err := transformUpstreamCompose(in, "my-app")
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "${APP_DATA_DIR}") {
		t.Errorf("expected APP_DATA_DIR placeholder substituted, got:\n%s", s)
	}
	if !strings.Contains(s, "/DATA/PowerLabAppData/my-app/data:/app/data") {
		t.Errorf("expected PowerLab AppData path substituted, got:\n%s", s)
	}
	if !strings.Contains(s, "/DATA/PowerLabAppData/my-app/config:/app/config:rw") {
		t.Errorf("expected second volume substituted, got:\n%s", s)
	}
	// Non-templated volumes must be untouched
	if !strings.Contains(s, "/tmp:/tmp") {
		t.Errorf("expected non-templated volume preserved, got:\n%s", s)
	}
}

func TestSubstituteAppDataDirInMapVolume(t *testing.T) {
	// Compose long-form volume entries — less common in Umbrel apps
	// but present in some, and the substitution must work on the
	// `source:` field of long-form entries too.
	in := []byte(`version: '3.7'
services:
  web:
    image: nginx:latest
    volumes:
      - type: bind
        source: ${APP_DATA_DIR}/state
        target: /var/lib/app
`)
	out, err := transformUpstreamCompose(in, "my-app")
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	if strings.Contains(string(out), "${APP_DATA_DIR}") {
		t.Errorf("expected APP_DATA_DIR substituted in map volume source, got:\n%s", out)
	}
	if !strings.Contains(string(out), "/DATA/PowerLabAppData/my-app/state") {
		t.Errorf("expected substituted path in map volume, got:\n%s", out)
	}
}

func TestTransformEnvVarsLeftAlone(t *testing.T) {
	// We deliberately do NOT touch ${APP_*} references in environment
	// vars — compose-go's strict validator only cares about volume
	// references. Leaving env-var placeholders means a future install-
	// time substitution layer can still resolve them with Umbrel-style
	// semantics; touching them now would lose information.
	in := []byte(`version: '3.7'
services:
  web:
    image: nginx:latest
    environment:
      - ALLOWED=http://${DEVICE_DOMAIN_NAME}:${APP_FOO_PORT}
    volumes:
      - ${APP_DATA_DIR}/data:/app/data
`)
	out, err := transformUpstreamCompose(in, "foo")
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "${DEVICE_DOMAIN_NAME}") {
		t.Errorf("env var DEVICE_DOMAIN_NAME should be preserved, got:\n%s", s)
	}
	if !strings.Contains(s, "${APP_FOO_PORT}") {
		t.Errorf("env var APP_FOO_PORT should be preserved, got:\n%s", s)
	}
}

func TestTransformAgentZero(t *testing.T) {
	// Realistic fixture from the first weekly sync — agent-zero is
	// representative of the dominant Umbrel app shape. The transform
	// must produce YAML where the result has neither `app_proxy:`
	// nor any `${APP_DATA_DIR}` literal, and the `app` service's
	// image + volumes survive.
	in := []byte(`version: '3.7'
services:
  app_proxy:
    environment:
      APP_HOST: agent-zero_web_1
      APP_PORT: 80
  web:
    image: agent0ai/agent-zero:v1.13
    restart: on-failure
    volumes:
      - ${APP_DATA_DIR}/data:/a0
    environment:
      - ALLOWED_ORIGINS=http://${DEVICE_DOMAIN_NAME}:${APP_AGENTZERO_PORT}
`)
	out, err := transformUpstreamCompose(in, "agent-zero")
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	s := string(out)

	if strings.Contains(s, "app_proxy") {
		t.Errorf("app_proxy should be dropped, got:\n%s", s)
	}
	if strings.Contains(s, "${APP_DATA_DIR}") {
		t.Errorf("${APP_DATA_DIR} should be substituted, got:\n%s", s)
	}
	if !strings.Contains(s, "/DATA/PowerLabAppData/agent-zero/data:/a0") {
		t.Errorf("expected substituted volume path, got:\n%s", s)
	}
	if !strings.Contains(s, "image: agent0ai/agent-zero:v1.13") {
		t.Errorf("real service image should be preserved, got:\n%s", s)
	}
	// Env var preservation
	if !strings.Contains(s, "${APP_AGENTZERO_PORT}") {
		t.Errorf("env var should be preserved (only volume refs are touched), got:\n%s", s)
	}

	// Verify the output is well-formed YAML by re-parsing
	var verify map[string]any
	if err := yaml.Unmarshal(out, &verify); err != nil {
		t.Fatalf("transformed output is not valid YAML: %v\n%s", err, out)
	}
	services, ok := verify["services"].(map[string]any)
	if !ok {
		t.Fatalf("services key missing or wrong type in:\n%s", out)
	}
	if _, hasProxy := services["app_proxy"]; hasProxy {
		t.Errorf("app_proxy still present in parsed output")
	}
	if _, hasWeb := services["web"]; !hasWeb {
		t.Errorf("web service missing in parsed output")
	}
}

func TestTransformIdempotent(t *testing.T) {
	// Running the transform twice should produce the same result —
	// useful as a self-check for the sync workflow that may re-emit
	// the same app multiple times across runs.
	in := []byte(`version: '3.7'
services:
  web:
    image: nginx:latest
    volumes:
      - ${APP_DATA_DIR}/data:/app/data
`)
	first, err := transformUpstreamCompose(in, "foo")
	if err != nil {
		t.Fatalf("first transform: %v", err)
	}
	second, err := transformUpstreamCompose(first, "foo")
	if err != nil {
		t.Fatalf("second transform: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("transform is not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// ─── Generalized volume placeholder substitution ───
// Beyond the original `${APP_DATA_DIR}` case, the production catalog
// surfaces TWO more placeholder families that need the same treatment.
// The v0.6.3 CI gate caught both — see production_catalog_test.go.

func TestSubstituteSiblingAppDataDir(t *testing.T) {
	// A Lightning-Network UI references the Lightning Node's data dir
	// via `${APP_LIGHTNING_NODE_DATA_DIR}` (Umbrel sibling-app convention).
	// On PowerLab the sibling won't exist, but we still substitute with
	// a valid path so the catalog parses.
	in := []byte(`version: '3.7'
services:
  web:
    image: agora:latest
    volumes:
      - ${APP_LIGHTNING_NODE_DATA_DIR}:/data/lnd:ro
`)
	out, err := transformUpstreamCompose(in, "agora")
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "${APP_LIGHTNING_NODE_DATA_DIR}") {
		t.Errorf("sibling-app data dir placeholder not substituted, got:\n%s", s)
	}
	if !strings.Contains(s, "/DATA/PowerLabAppData/agora") {
		t.Errorf("expected substituted to AppData path, got:\n%s", s)
	}
}

func TestSubstituteUmbrelRoot(t *testing.T) {
	// Apps that read from Umbrel's shared storage tree
	// (`/data/storage/downloads`, `/data/storage/music`, …) prefix the
	// path with `${UMBREL_ROOT}`. We map that to `/DATA` (PowerLab's
	// shared storage root).
	in := []byte(`version: '3.7'
services:
  server:
    image: deluge:latest
    volumes:
      - ${UMBREL_ROOT}/data/storage/downloads:/downloads
`)
	out, err := transformUpstreamCompose(in, "deluge")
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "${UMBREL_ROOT}") {
		t.Errorf("${UMBREL_ROOT} not substituted, got:\n%s", s)
	}
	if !strings.Contains(s, "/DATA/data/storage/downloads:/downloads") {
		t.Errorf("expected /DATA mapping, got:\n%s", s)
	}
}

func TestSubstituteMultipleVarKindsInOneCompose(t *testing.T) {
	// Realistic shape: app that uses its OWN data dir AND reads from
	// Umbrel's downloads tree. Both must be substituted.
	in := []byte(`version: '3.7'
services:
  web:
    image: sonarr:latest
    volumes:
      - ${APP_DATA_DIR}/config:/config
      - ${UMBREL_ROOT}/data/storage/downloads:/downloads
      - ${APP_LIGHTNING_NODE_DATA_DIR}:/lightning:ro
`)
	out, err := transformUpstreamCompose(in, "sonarr")
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	if strings.Contains(string(out), "${") {
		t.Errorf("all placeholders should be substituted, got:\n%s", out)
	}
}

// ─── Top-level `name:` injection — see transform.go for the bug story ───

func TestTransformInjectsNameField(t *testing.T) {
	in := []byte(`version: '3.7'
services:
  web:
    image: nginx:latest
`)
	out, err := transformUpstreamCompose(in, "agent-zero")
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	if !strings.Contains(string(out), "name: agent-zero") {
		t.Errorf("expected top-level `name: agent-zero`, got:\n%s", out)
	}
}

func TestTransformOverridesExistingName(t *testing.T) {
	// If the upstream compose already has a name field, it must be
	// overwritten with our store_app_id — otherwise BuildCatalog
	// would key by the upstream's name (which may not match our id).
	in := []byte(`version: '3.7'
name: some-other-name
services:
  web:
    image: nginx:latest
`)
	out, err := transformUpstreamCompose(in, "my-id")
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "name: some-other-name") {
		t.Errorf("existing name field should be overridden, got:\n%s", s)
	}
	if !strings.Contains(s, "name: my-id") {
		t.Errorf("name should be our store_app_id, got:\n%s", s)
	}
}

// ─── env_file dropping + port placeholder substitution ───
// These cover the remaining v0.6.2 catalog-read failures after the
// initial volume + app_proxy fix unblocked 204/241 apps; these two
// transforms close the gap to ~all-241 by handling the patterns
// the production sync surfaced on the user's box on 2026-05-12.

func TestDropEnvFileFromServices(t *testing.T) {
	in := []byte(`version: '3.7'
services:
  web:
    image: nginx:latest
    env_file:
      - ${APP_DATA_DIR}/settings.env
      - ${APP_DATA_DIR}/keys.env
    environment:
      - LOG_LEVEL=info
`)
	out, err := transformUpstreamCompose(in, "foo")
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "env_file") {
		t.Errorf("env_file directive should be dropped, got:\n%s", s)
	}
	// `environment:` (inline env vars) must survive — only env_file is dropped
	if !strings.Contains(s, "LOG_LEVEL=info") {
		t.Errorf("inline environment list should be preserved, got:\n%s", s)
	}
}

func TestSubstitutePortPlaceholdersStringForm(t *testing.T) {
	in := []byte(`version: '3.7'
services:
  web:
    image: nginx:latest
    ports:
      - "22:${APP_SSH_PORT}"
      - "${APP_HTTP_PORT}:80"
`)
	out, err := transformUpstreamCompose(in, "foo")
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "${APP_SSH_PORT}") || strings.Contains(s, "${APP_HTTP_PORT}") {
		t.Errorf("port placeholders should be substituted, got:\n%s", s)
	}
	// Should contain the placeholder integer (18000-range)
	if !strings.Contains(s, "18000") && !strings.Contains(s, "18001") {
		t.Errorf("expected substituted port integers (18000+), got:\n%s", s)
	}
}

func TestSubstitutePortPlaceholdersMapForm(t *testing.T) {
	in := []byte(`version: '3.7'
services:
  web:
    image: nginx:latest
    ports:
      - target: 80
        published: ${APP_HTTP_PORT}
        protocol: tcp
`)
	out, err := transformUpstreamCompose(in, "foo")
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	if strings.Contains(string(out), "${APP_HTTP_PORT}") {
		t.Errorf("long-form port placeholder should be substituted, got:\n%s", out)
	}
}

func TestPortPlaceholderDistinctAcrossServices(t *testing.T) {
	// If 2 services each use a port placeholder, they must end up with
	// DIFFERENT integers so compose-go's host-port collision check
	// doesn't reject the project.
	in := []byte(`version: '3.7'
services:
  web:
    image: nginx:latest
    ports:
      - "${APP_WEB_PORT}:80"
  ssh:
    image: linuxserver/openssh:latest
    ports:
      - "${APP_SSH_PORT}:22"
`)
	out, err := transformUpstreamCompose(in, "foo")
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "${") {
		t.Errorf("all placeholders should be substituted, got:\n%s", s)
	}
	// Both 18000 AND 18001 should appear — distinct allocations per
	// service ensure no host-port collision.
	if !strings.Contains(s, "18000") || !strings.Contains(s, "18001") {
		t.Errorf("expected 18000 + 18001 for distinct services, got:\n%s", s)
	}
}

func TestNonPlaceholderPortsUntouched(t *testing.T) {
	in := []byte(`version: '3.7'
services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"
      - "8443:443"
`)
	out, err := transformUpstreamCompose(in, "foo")
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "8080:80") {
		t.Errorf("literal ports should be preserved, got:\n%s", s)
	}
	if !strings.Contains(s, "8443:443") {
		t.Errorf("second literal port should be preserved, got:\n%s", s)
	}
}

// ─── Defensive edge cases — must not panic, return reasonable output ───

func TestTransformEmptyInput(t *testing.T) {
	out, err := transformUpstreamCompose([]byte(""), "foo")
	if err != nil {
		t.Fatalf("empty input: expected nil err, got %v", err)
	}
	// Empty yaml unmarshals to nil — re-marshal produces "null\n".
	// We don't care about the exact form, just that we return SOMETHING
	// without panicking. Downstream emit.go appends x-powerlab block so
	// the final file is still a valid catalog entry.
	if len(out) == 0 {
		t.Errorf("expected non-empty marshal output, got empty")
	}
}

func TestTransformNoServicesKey(t *testing.T) {
	in := []byte(`version: '3.7'
networks:
  default:
    external: true
`)
	out, err := transformUpstreamCompose(in, "foo")
	if err != nil {
		t.Fatalf("no services key: %v", err)
	}
	if !strings.Contains(string(out), "version") {
		t.Errorf("version key should survive transform, got:\n%s", out)
	}
}

func TestTransformEmptyServices(t *testing.T) {
	in := []byte(`version: '3.7'
services: {}
`)
	out, err := transformUpstreamCompose(in, "foo")
	if err != nil {
		t.Fatalf("empty services: %v", err)
	}
	if !strings.Contains(string(out), "services") {
		t.Errorf("services key should survive, got:\n%s", out)
	}
}

func TestTransformOnlyAppProxy(t *testing.T) {
	// Degenerate case: an upstream app that has ONLY app_proxy. The
	// Phase 1 filter should reject this earlier — there's no real
	// service to install. If it slips through, the transform should
	// produce an empty services map rather than crashing.
	in := []byte(`version: '3.7'
services:
  app_proxy:
    environment:
      APP_HOST: foo
      APP_PORT: 80
`)
	out, err := transformUpstreamCompose(in, "foo")
	if err != nil {
		t.Fatalf("only app_proxy: %v", err)
	}
	if strings.Contains(string(out), "app_proxy") {
		t.Errorf("app_proxy should be removed even when sole service")
	}
}

func TestTransformVolumesWithIntegerValue(t *testing.T) {
	// Defensive: a volume entry that's neither a string nor a map
	// (e.g., a typo where someone put a plain integer) shouldn't
	// crash. compose-go will reject it later but the transform must
	// pass through.
	in := []byte(`version: '3.7'
services:
  web:
    image: nginx:latest
    volumes:
      - 12345
      - ${APP_DATA_DIR}/data:/app/data
`)
	out, err := transformUpstreamCompose(in, "foo")
	if err != nil {
		t.Fatalf("integer volume entry: %v", err)
	}
	// Substitution still applies to the string entry
	if strings.Contains(string(out), "${APP_DATA_DIR}") {
		t.Errorf("APP_DATA_DIR should be substituted in string entries even when other entries are non-string, got:\n%s", out)
	}
}

func TestTransformVolumesWithNullEntry(t *testing.T) {
	// nil volume entry — yaml.v3 will produce `nil` for `- ~` or `-`
	in := []byte(`version: '3.7'
services:
  web:
    image: nginx:latest
    volumes:
      - ~
      - ${APP_DATA_DIR}/foo:/foo
`)
	out, err := transformUpstreamCompose(in, "foo")
	if err != nil {
		t.Fatalf("null volume entry: %v", err)
	}
	if strings.Contains(string(out), "${APP_DATA_DIR}") {
		t.Errorf("substitution should still apply to other entries when one is null, got:\n%s", out)
	}
}

func TestTransformServiceWithNoVolumes(t *testing.T) {
	in := []byte(`version: '3.7'
services:
  web:
    image: nginx:latest
    environment:
      - FOO=bar
`)
	out, err := transformUpstreamCompose(in, "foo")
	if err != nil {
		t.Fatalf("service with no volumes: %v", err)
	}
	if !strings.Contains(string(out), "image: nginx:latest") {
		t.Errorf("service image should survive, got:\n%s", out)
	}
}

func TestTransformMultipleServicesAllSubstituted(t *testing.T) {
	in := []byte(`version: '3.7'
services:
  app_proxy:
    environment:
      APP_HOST: foo
  web:
    image: nginx:latest
    volumes:
      - ${APP_DATA_DIR}/web:/var/www
  worker:
    image: redis:7
    volumes:
      - ${APP_DATA_DIR}/redis:/data
  db:
    image: postgres:16
    volumes:
      - ${APP_DATA_DIR}/db:/var/lib/postgresql/data
`)
	out, err := transformUpstreamCompose(in, "myapp")
	if err != nil {
		t.Fatalf("multi-service: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "${APP_DATA_DIR}") {
		t.Errorf("all APP_DATA_DIR refs should be substituted across services, got:\n%s", s)
	}
	if strings.Count(s, "/DATA/PowerLabAppData/myapp") < 3 {
		t.Errorf("expected at least 3 substituted paths (web/worker/db), got:\n%s", s)
	}
	if strings.Contains(s, "app_proxy") {
		t.Errorf("app_proxy should be dropped, got:\n%s", s)
	}
}

func TestTransformPreservesNetworksAndConfigs(t *testing.T) {
	// Top-level networks, configs, secrets must survive. These are
	// out of scope for the Umbrel transform (they're not the bug class
	// causing parse failures) but we shouldn't strip them by accident.
	in := []byte(`version: '3.7'
services:
  web:
    image: nginx:latest
networks:
  default:
    driver: bridge
configs:
  myconf:
    external: true
`)
	out, err := transformUpstreamCompose(in, "foo")
	if err != nil {
		t.Fatalf("with networks+configs: %v", err)
	}
	s := string(out)
	for _, want := range []string{"networks:", "configs:", "default:", "myconf:"} {
		if !strings.Contains(s, want) {
			t.Errorf("expected %q preserved, got:\n%s", want, s)
		}
	}
}

func TestTransformSubstringNotMatch(t *testing.T) {
	// Edge case: a volume name that happens to CONTAIN the placeholder
	// substring inside a longer identifier — e.g., literal text in a
	// label or comment. Only `${APP_DATA_DIR}` (with the braces) should
	// match; `APP_DATA_DIR` without braces or `${APP_DATA_DIRECTORY}`
	// (different name) must NOT.
	in := []byte(`version: '3.7'
services:
  web:
    image: nginx:latest
    environment:
      - DESCRIPTION=The ${APP_DATA_DIR} placeholder is replaced in volumes only
      - SIMILAR=${APP_DATA_DIRECTORY}/nope
    volumes:
      - ${APP_DATA_DIR}/data:/app
`)
	out, err := transformUpstreamCompose(in, "foo")
	if err != nil {
		t.Fatalf("substring test: %v", err)
	}
	s := string(out)
	// env DESCRIPTION should keep ${APP_DATA_DIR} (env vars untouched)
	if !strings.Contains(s, "${APP_DATA_DIR} placeholder") {
		t.Errorf("env var with APP_DATA_DIR should be preserved verbatim, got:\n%s", s)
	}
	// SIMILAR keeps APP_DATA_DIRECTORY (different name)
	if !strings.Contains(s, "${APP_DATA_DIRECTORY}") {
		t.Errorf("APP_DATA_DIRECTORY (different name) should be preserved, got:\n%s", s)
	}
	// volume IS substituted
	if !strings.Contains(s, "/DATA/PowerLabAppData/foo/data") {
		t.Errorf("volume reference should be substituted, got:\n%s", s)
	}
}

func TestTransformDeeplyNestedDoesntPanic(t *testing.T) {
	// Defensive: a deeply-nested compose that exercises map walks
	// shouldn't panic even if shapes are unexpected.
	in := []byte(`version: '3.7'
services:
  web:
    image: nginx:latest
    deploy:
      resources:
        limits:
          memory: 512M
        reservations:
          memory: 256M
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost"]
      interval: 30s
    volumes:
      - ${APP_DATA_DIR}/data:/app/data
    labels:
      io.powerlab.test: "true"
      nested.label.deep: "value"
`)
	out, err := transformUpstreamCompose(in, "foo")
	if err != nil {
		t.Fatalf("deeply nested: %v", err)
	}
	if strings.Contains(string(out), "${APP_DATA_DIR}") {
		t.Errorf("APP_DATA_DIR should still be substituted despite deep nesting, got:\n%s", out)
	}
	// Healthcheck must survive intact
	if !strings.Contains(string(out), "healthcheck") {
		t.Errorf("healthcheck section should be preserved, got:\n%s", out)
	}
}

func TestTransformMalformedYAMLReturnsError(t *testing.T) {
	// yaml.v3 is permissive — most "wrong-looking" inputs still parse.
	// We use truly invalid YAML (unclosed quote at top level) to lock
	// that the error path returns a wrapped error rather than panicking.
	in := []byte(`version: '3.7
services:
  web: [`)
	_, err := transformUpstreamCompose(in, "foo")
	if err == nil {
		t.Errorf("expected error on malformed YAML, got nil")
	}
}
