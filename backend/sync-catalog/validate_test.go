package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// validCompose is the minimum well-formed catalog entry — used as a
// baseline that each negative test mutates exactly one field.
const validCompose = `services:
  app:
    image: nginx:latest
    ports: ["80:80"]
x-powerlab:
  store_app_id: nginx-proxy-manager
  title: { en_us: "Nginx Proxy Manager" }
  icon: https://getumbrel.github.io/umbrel-apps-gallery/nginx-proxy-manager/icon.svg
  source:
    catalog: umbrel-apps
    upstream_commit: abc123
`

// helper: write `<root>/Apps/<id>/docker-compose.yml` with the given content.
func writeApp(t *testing.T, root, appID, content string) {
	t.Helper()
	dir := filepath.Join(root, "Apps", appID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestValidate_ValidCatalog_NoErrors locks the happy path.
// Every emitted appfile from Phase 4 should pass this validator
// out of the box.
func TestValidate_ValidCatalog_NoErrors(t *testing.T) {
	root := t.TempDir()
	writeApp(t, root, "nginx-proxy-manager", validCompose)

	errs, err := ValidateCatalogTree(root)
	if err != nil {
		t.Fatalf("ValidateCatalogTree: %v", err)
	}
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d:\n%v", len(errs), errs)
	}
}

// TestValidate_EmptyCatalog_NoErrors — the catalog dir may exist
// but be empty (no apps synced yet). That's fine.
func TestValidate_EmptyCatalog_NoErrors(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Apps"), 0o755); err != nil {
		t.Fatal(err)
	}
	errs, err := ValidateCatalogTree(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(errs) != 0 {
		t.Errorf("expected no errors for empty Apps dir, got %d", len(errs))
	}
}

// TestValidate_MissingAppsDir_NoErrors — Phase 4.5 hasn't seeded
// the directory yet; validator must be a no-op rather than fail.
func TestValidate_MissingAppsDir_NoErrors(t *testing.T) {
	root := t.TempDir()
	errs, err := ValidateCatalogTree(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(errs) != 0 {
		t.Errorf("expected no errors when Apps dir is absent, got %d", len(errs))
	}
}

// TestValidate_MalformedYAML_FlagsParseError surfaces a clear
// rule name so CI logs can be grepped for failure class.
func TestValidate_MalformedYAML_FlagsParseError(t *testing.T) {
	root := t.TempDir()
	writeApp(t, root, "broken-app", "this: is: not: valid: yaml")

	errs, _ := ValidateCatalogTree(root)
	if len(errs) == 0 {
		t.Fatal("expected errors for malformed YAML")
	}
	if errs[0].Rule != "yaml.parse" {
		t.Errorf("expected yaml.parse rule, got %q (%s)", errs[0].Rule, errs[0].Message)
	}
}

// TestValidate_MissingServicesBlock — a catalog entry without
// `services:` would deploy as nothing; flag it.
func TestValidate_MissingServicesBlock(t *testing.T) {
	root := t.TempDir()
	writeApp(t, root, "no-services", `x-powerlab:
  store_app_id: foo
  title: { en_us: "Foo" }
  source: { catalog: umbrel-apps }
`)
	errs, _ := ValidateCatalogTree(root)
	if !hasRule(errs, "yaml.services") {
		t.Errorf("expected yaml.services rule, got %v", errs)
	}
}

// TestValidate_MissingXPowerlab — every community-catalog entry
// MUST carry the x-powerlab extension (provenance + store-info).
func TestValidate_MissingXPowerlab(t *testing.T) {
	root := t.TempDir()
	writeApp(t, root, "no-extension", `services:
  app:
    image: nginx
`)
	errs, _ := ValidateCatalogTree(root)
	if !hasRule(errs, "yaml.x-powerlab") {
		t.Errorf("expected yaml.x-powerlab rule, got %v", errs)
	}
}

// TestValidate_MissingStoreAppID flags the required identifier.
func TestValidate_MissingStoreAppID(t *testing.T) {
	root := t.TempDir()
	writeApp(t, root, "no-id", `services:
  app: { image: nginx }
x-powerlab:
  title: { en_us: "Foo" }
  source: { catalog: umbrel-apps }
`)
	errs, _ := ValidateCatalogTree(root)
	if !hasRule(errs, "field.store_app_id") {
		t.Errorf("expected field.store_app_id rule, got %v", errs)
	}
}

// TestValidate_MissingTitle flags the missing display name.
func TestValidate_MissingTitle(t *testing.T) {
	root := t.TempDir()
	writeApp(t, root, "no-title", `services:
  app: { image: nginx }
x-powerlab:
  store_app_id: foo
  source: { catalog: umbrel-apps }
`)
	errs, _ := ValidateCatalogTree(root)
	if !hasRule(errs, "field.title") {
		t.Errorf("expected field.title rule, got %v", errs)
	}
}

// TestValidate_MissingSource — provenance is required for every
// community-catalog entry. Locks the "debug origem" requirement
// at the on-disk level: a catalog entry without provenance fails
// validation, so the CI gate cannot let one slip through.
func TestValidate_MissingSource(t *testing.T) {
	root := t.TempDir()
	writeApp(t, root, "no-source", `services:
  app: { image: nginx }
x-powerlab:
  store_app_id: foo
  title: { en_us: "Foo" }
`)
	errs, _ := ValidateCatalogTree(root)
	if !hasRule(errs, "field.source") {
		t.Errorf("expected field.source rule, got %v", errs)
	}
}

// TestValidate_EmptyIcon_FailsButPresent — icon is OPTIONAL,
// but if present must be non-empty + URL-parseable.
func TestValidate_EmptyIcon_FailsButPresent(t *testing.T) {
	root := t.TempDir()
	writeApp(t, root, "empty-icon", `services:
  app: { image: nginx }
x-powerlab:
  store_app_id: foo
  title: { en_us: "Foo" }
  icon: ""
  source: { catalog: umbrel-apps }
`)
	errs, _ := ValidateCatalogTree(root)
	if !hasRule(errs, "field.icon") {
		t.Errorf("expected field.icon rule for empty icon, got %v", errs)
	}
}

// TestValidate_IconAbsent_IsFine — an entry with no `icon:` at
// all passes validation. The UI falls through to its Lucide
// Package fallback. We don't enforce icons.
func TestValidate_IconAbsent_IsFine(t *testing.T) {
	root := t.TempDir()
	writeApp(t, root, "no-icon", `services:
  app: { image: nginx }
x-powerlab:
  store_app_id: foo
  title: { en_us: "Foo" }
  source: { catalog: umbrel-apps }
`)
	errs, _ := ValidateCatalogTree(root)
	if len(errs) != 0 {
		t.Errorf("absent icon should not error, got %v", errs)
	}
}

// TestValidate_MultipleErrorsPerApp returns all rules at once
// for one badly-shaped entry — CI runs print every problem in
// one pass instead of make-fix-rerun ping-pong.
func TestValidate_MultipleErrorsPerApp(t *testing.T) {
	root := t.TempDir()
	writeApp(t, root, "mess", `# nothing useful
`)
	errs, _ := ValidateCatalogTree(root)
	// Expect at least services + x-powerlab rules
	if len(errs) < 2 {
		t.Errorf("expected ≥2 errors for empty doc, got %d:\n%v", len(errs), errs)
	}
}

// TestValidate_DeterministicOrder confirms CI logs are diffable
// across runs — sort by path then rule.
func TestValidate_DeterministicOrder(t *testing.T) {
	root := t.TempDir()
	writeApp(t, root, "alpha-broken", "this: is: not: yaml")
	writeApp(t, root, "beta-also-broken", "this: is: not: yaml")

	errs, _ := ValidateCatalogTree(root)
	if len(errs) < 2 {
		t.Fatalf("expected ≥2 errors, got %d", len(errs))
	}
	if !strings.Contains(errs[0].Path, "alpha-broken") {
		t.Errorf("expected alpha-broken first (alpha order), got %q", errs[0].Path)
	}
}

func hasRule(errs []ValidationError, rule string) bool {
	for _, e := range errs {
		if e.Rule == rule {
			return true
		}
	}
	return false
}
