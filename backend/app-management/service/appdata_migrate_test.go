package service

import (
	"os"
	"path/filepath"
	"testing"
)

// Tests for the on-boot AppData migration (ADR-0021 + #85 PR-C).
// Each test scaffolds a tiny filesystem under t.TempDir():
//
//   <tmp>/storage/AppData/<X>      — legacy data
//   <tmp>/storage/PowerLabAppData/ — canonical destination root
//   <tmp>/apps/<X>/docker-compose.yml  — proves PowerLab manages X
//
// Per memory rule "TDD strict — tests first" the assertions lock the
// migration's boundary contracts: which apps move, which stay, what
// the result actions are.

func setup(t *testing.T) (storage, apps string) {
	t.Helper()
	tmp := t.TempDir()
	storage = filepath.Join(tmp, "storage")
	apps = filepath.Join(tmp, "apps")
	if err := os.MkdirAll(storage, 0o755); err != nil {
		t.Fatalf("setup storage: %v", err)
	}
	if err := os.MkdirAll(apps, 0o755); err != nil {
		t.Fatalf("setup apps: %v", err)
	}
	return storage, apps
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func makeAppDataDir(t *testing.T, storage, app, content string) {
	t.Helper()
	dir := filepath.Join(storage, "AppData", app)
	writeFile(t, filepath.Join(dir, "data.txt"), content)
}

func makeComposeProject(t *testing.T, apps, app string) {
	t.Helper()
	writeFile(t, filepath.Join(apps, app, "docker-compose.yml"), "version: '3'\n")
}

func resultsByApp(rs []AppDataMigrationResult) map[string]AppDataMigrationResult {
	m := make(map[string]AppDataMigrationResult, len(rs))
	for _, r := range rs {
		m[r.AppName] = r
	}
	return m
}

// TestMigrate_HappyPath — PowerLab manages `nextcloud`; its AppData
// gets moved canonical-side, source disappears.
func TestMigrate_HappyPath(t *testing.T) {
	storage, apps := setup(t)
	makeAppDataDir(t, storage, "nextcloud", "user data")
	makeComposeProject(t, apps, "nextcloud")

	results := MigrateAppData(storage, apps)

	r, ok := resultsByApp(results)["nextcloud"]
	if !ok {
		t.Fatal("expected a result for nextcloud, got none")
	}
	if r.Action != "migrated" {
		t.Errorf("Action = %q, want migrated", r.Action)
	}

	// Canonical now exists with content
	dst := filepath.Join(storage, "PowerLabAppData", "nextcloud", "data.txt")
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("canonical missing: %v", err)
	}
	if string(got) != "user data" {
		t.Errorf("canonical content = %q, want %q", got, "user data")
	}

	// Legacy gone
	if _, err := os.Stat(filepath.Join(storage, "AppData", "nextcloud")); !os.IsNotExist(err) {
		t.Errorf("legacy still exists after migration")
	}
}

// TestMigrate_SkipsAppsPowerLabDoesNotManage — `casaos-only-app`
// exists in AppData/ but PowerLab has no compose project for it.
// Must not be moved (would steal CasaOS's data on a coexistence host).
func TestMigrate_SkipsAppsPowerLabDoesNotManage(t *testing.T) {
	storage, apps := setup(t)
	makeAppDataDir(t, storage, "casaos-only-app", "casaos data")
	// no compose project for casaos-only-app

	results := MigrateAppData(storage, apps)

	r, ok := resultsByApp(results)["casaos-only-app"]
	if !ok {
		t.Fatal("expected a result for casaos-only-app, got none")
	}
	if r.Action != "skipped-no-project" {
		t.Errorf("Action = %q, want skipped-no-project", r.Action)
	}

	// Legacy still there, untouched
	got, err := os.ReadFile(filepath.Join(storage, "AppData", "casaos-only-app", "data.txt"))
	if err != nil {
		t.Fatalf("legacy disappeared: %v", err)
	}
	if string(got) != "casaos data" {
		t.Errorf("legacy content corrupted: %q", got)
	}

	// Canonical NOT created
	if _, err := os.Stat(filepath.Join(storage, "PowerLabAppData", "casaos-only-app")); !os.IsNotExist(err) {
		t.Errorf("canonical wrongly created for unmanaged app")
	}
}

// TestMigrate_MixedHost — PowerLab + CasaOS coexistence: two apps
// in AppData/, only one with a PowerLab compose project. Each
// gets its correct disposition, in one pass.
func TestMigrate_MixedHost(t *testing.T) {
	storage, apps := setup(t)
	makeAppDataDir(t, storage, "nextcloud", "powerlab nextcloud data")
	makeAppDataDir(t, storage, "jellyfin", "casaos jellyfin data")
	makeComposeProject(t, apps, "nextcloud")
	// no compose project for jellyfin

	results := MigrateAppData(storage, apps)
	rmap := resultsByApp(results)

	if rmap["nextcloud"].Action != "migrated" {
		t.Errorf("nextcloud: Action = %q, want migrated", rmap["nextcloud"].Action)
	}
	if rmap["jellyfin"].Action != "skipped-no-project" {
		t.Errorf("jellyfin: Action = %q, want skipped-no-project", rmap["jellyfin"].Action)
	}

	// Confirm the disk state
	if _, err := os.Stat(filepath.Join(storage, "PowerLabAppData", "nextcloud")); err != nil {
		t.Errorf("nextcloud not at canonical: %v", err)
	}
	if _, err := os.Stat(filepath.Join(storage, "AppData", "jellyfin")); err != nil {
		t.Errorf("jellyfin no longer at legacy: %v", err)
	}
	if _, err := os.Stat(filepath.Join(storage, "PowerLabAppData", "jellyfin")); !os.IsNotExist(err) {
		t.Errorf("jellyfin wrongly created at canonical")
	}
}

// TestMigrate_CanonicalAlreadyExists — partial migration scenario:
// canonical PowerLabAppData/nextcloud already has content (e.g. user
// manually moved it). Migration must NOT auto-merge; instead it
// renames the legacy aside as <legacy>.bak.<ts> and reports
// skipped-canonical-exists.
func TestMigrate_CanonicalAlreadyExists(t *testing.T) {
	storage, apps := setup(t)
	makeAppDataDir(t, storage, "nextcloud", "stale legacy data")
	writeFile(t, filepath.Join(storage, "PowerLabAppData", "nextcloud", "live.txt"), "fresh canonical data")
	makeComposeProject(t, apps, "nextcloud")

	// Stub time so the .bak suffix is deterministic
	origNow := nowUnix
	t.Cleanup(func() { nowUnix = origNow })
	nowUnix = func() int64 { return 1234567890 }

	results := MigrateAppData(storage, apps)
	r := resultsByApp(results)["nextcloud"]
	if r.Action != "skipped-canonical-exists" {
		t.Errorf("Action = %q, want skipped-canonical-exists", r.Action)
	}

	// Canonical content untouched
	got, err := os.ReadFile(filepath.Join(storage, "PowerLabAppData", "nextcloud", "live.txt"))
	if err != nil || string(got) != "fresh canonical data" {
		t.Errorf("canonical disturbed; got %q err=%v", got, err)
	}

	// Legacy moved to .bak.<ts>
	wantBackup := filepath.Join(storage, "AppData", "nextcloud.bak.1234567890")
	if _, err := os.Stat(wantBackup); err != nil {
		t.Errorf("legacy not preserved as backup at %s: %v", wantBackup, err)
	}
	if r.Backup != wantBackup {
		t.Errorf("result.Backup = %q, want %q", r.Backup, wantBackup)
	}

	// Original legacy path gone
	if _, err := os.Stat(filepath.Join(storage, "AppData", "nextcloud")); !os.IsNotExist(err) {
		t.Errorf("legacy still at original path; should be backed up")
	}
}

// TestMigrate_IdempotentReRun — running migration a second time on
// an already-migrated host is a no-op (legacy is empty so discovery
// returns nothing).
func TestMigrate_IdempotentReRun(t *testing.T) {
	storage, apps := setup(t)
	makeAppDataDir(t, storage, "nextcloud", "data")
	makeComposeProject(t, apps, "nextcloud")

	first := MigrateAppData(storage, apps)
	if first[0].Action != "migrated" {
		t.Fatalf("first run setup wrong: %v", first)
	}

	second := MigrateAppData(storage, apps)
	if len(second) != 0 {
		t.Errorf("second run returned %d results, want 0", len(second))
	}

	// Canonical still has the content
	dst := filepath.Join(storage, "PowerLabAppData", "nextcloud", "data.txt")
	if got, err := os.ReadFile(dst); err != nil || string(got) != "data" {
		t.Errorf("canonical disturbed by second run; got %q err=%v", got, err)
	}
}

// TestMigrate_NoLegacyDir — fresh install: no AppData/ exists yet.
// Returns empty result, no error.
func TestMigrate_NoLegacyDir(t *testing.T) {
	storage, apps := setup(t)
	results := MigrateAppData(storage, apps)
	if len(results) != 0 {
		t.Errorf("fresh-install migration returned %d results, want 0", len(results))
	}
}

// TestMigrate_StrayFilesIgnored — non-directory entries at AppData/
// root (e.g. a stray .DS_Store) must be skipped silently.
func TestMigrate_StrayFilesIgnored(t *testing.T) {
	storage, apps := setup(t)
	writeFile(t, filepath.Join(storage, "AppData", ".DS_Store"), "metadata junk")
	makeAppDataDir(t, storage, "nextcloud", "real data")
	makeComposeProject(t, apps, "nextcloud")

	results := MigrateAppData(storage, apps)

	// Should have a result for nextcloud only (no entry for .DS_Store)
	if len(results) != 1 {
		t.Errorf("got %d results, want 1 (nextcloud only)", len(results))
	}
	if results[0].AppName != "nextcloud" {
		t.Errorf("result for unexpected app: %s", results[0].AppName)
	}

	// Stray file untouched
	if got, err := os.ReadFile(filepath.Join(storage, "AppData", ".DS_Store")); err != nil || string(got) != "metadata junk" {
		t.Errorf("stray file disturbed; got %q err=%v", got, err)
	}
}

// TestMigrate_AppWithComposeYAMLExtension — PowerLab convention is
// docker-compose.yml but a few legacy apps ship .yaml. Both must be
// recognized as PowerLab projects.
func TestMigrate_AppWithComposeYAMLExtension(t *testing.T) {
	storage, apps := setup(t)
	makeAppDataDir(t, storage, "trilium", "data")
	writeFile(t, filepath.Join(apps, "trilium", "docker-compose.yaml"), "version: '3'\n")

	results := MigrateAppData(storage, apps)
	r := resultsByApp(results)["trilium"]
	if r.Action != "migrated" {
		t.Errorf(".yaml extension not recognized: Action = %q", r.Action)
	}
}
