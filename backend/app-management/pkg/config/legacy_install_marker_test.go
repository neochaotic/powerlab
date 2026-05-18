package config

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// Legacy install marker (#437, Sprint 23). On the first boot after a
// box upgrades to v0.7.1, scan AppsPath and tag every existing app
// directory with a `.installed-pre-v0.7.1` marker. Apps installed
// AFTER the migration runs (via the install flow) never get the
// marker. A top-level sentinel `.legacy-scan-complete` makes the
// migration idempotent — boots after the first one are a noop.
//
// The marker is consumed in a follow-up PR that surfaces a "Legacy"
// badge in the apps grid (originally framed in #437 as part of the
// ADR-0038 toggle world; reframed for ADR-0039 catalog model in
// Sprint 23 — see the issue thread).

func mkApp(t *testing.T, appsDir, name string) {
	t.Helper()
	d := filepath.Join(appsDir, name)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", d, err)
	}
	if err := os.WriteFile(filepath.Join(d, "docker-compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write compose for %q: %v", d, err)
	}
}

func listMarked(t *testing.T, appsDir string) []string {
	t.Helper()
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		t.Fatalf("readdir %q: %v", appsDir, err)
	}
	out := []string{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(appsDir, e.Name(), legacyInstallMarkerFilename)); err == nil {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

func TestMarkPreUpgradeAppsAsLegacy_TagsAllExisting(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"adventurelog", "baikal", "enclosed"} {
		mkApp(t, dir, n)
	}

	count, err := markPreUpgradeAppsAsLegacy(dir)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 apps marked, got %d", count)
	}
	marked := listMarked(t, dir)
	want := []string{"adventurelog", "baikal", "enclosed"}
	if !reflect.DeepEqual(marked, want) {
		t.Errorf("marked apps: got %v, want %v", marked, want)
	}
}

func TestMarkPreUpgradeAppsAsLegacy_SkipsAlreadyMarked(t *testing.T) {
	// An app that already has the marker (e.g. shipped that way from
	// a previous partial migration run) must not be re-touched. The
	// marker file's mtime is the operator's reference point — if the
	// migration re-wrote it on every boot, the badge UX would lie.
	dir := t.TempDir()
	mkApp(t, dir, "adventurelog")
	preExistingMarker := filepath.Join(dir, "adventurelog", legacyInstallMarkerFilename)
	if err := os.WriteFile(preExistingMarker, []byte("pre-existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	origInfo, err := os.Stat(preExistingMarker)
	if err != nil {
		t.Fatal(err)
	}
	origModTime := origInfo.ModTime()

	count, err := markPreUpgradeAppsAsLegacy(dir)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	// adventurelog already marked → not counted as "newly marked".
	if count != 0 {
		t.Errorf("expected 0 NEW marks (already marked), got %d", count)
	}
	afterInfo, _ := os.Stat(preExistingMarker)
	if !afterInfo.ModTime().Equal(origModTime) {
		t.Errorf("marker was rewritten — mtime changed from %v to %v", origModTime, afterInfo.ModTime())
	}
}

func TestMarkPreUpgradeAppsAsLegacy_IgnoresNonDirEntries(t *testing.T) {
	// Files at the top of AppsPath (stray .DS_Store, logs, etc.) are
	// not apps — must not get tagged.
	dir := t.TempDir()
	mkApp(t, dir, "adventurelog")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".DS_Store"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	count, err := markPreUpgradeAppsAsLegacy(dir)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 mark (only adventurelog is a dir), got %d", count)
	}
}

func TestMarkPreUpgradeAppsAsLegacy_MissingAppsDir_NoOp(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "does-not-exist")
	count, err := markPreUpgradeAppsAsLegacy(dir)
	if err != nil {
		t.Fatalf("expected nil err for missing dir, got %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 marks on missing dir, got %d", count)
	}
}

func TestMigrateLegacyAppMarker_RunsOnceThenSentinelGates(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"adventurelog", "baikal"} {
		mkApp(t, dir, n)
	}

	// First run: 2 marks + sentinel created.
	if err := migrateLegacyAppMarkerAt(dir); err != nil {
		t.Fatalf("first run unexpected err: %v", err)
	}
	marked1 := listMarked(t, dir)
	if !reflect.DeepEqual(marked1, []string{"adventurelog", "baikal"}) {
		t.Errorf("first run marks: got %v, want [adventurelog baikal]", marked1)
	}
	sentinel := filepath.Join(dir, legacyScanSentinelFilename)
	if _, err := os.Stat(sentinel); err != nil {
		t.Errorf("sentinel not created: %v", err)
	}

	// Simulate an app installed AFTER the sentinel — must NOT get
	// tagged on the second run.
	mkApp(t, dir, "newly-installed")
	if err := migrateLegacyAppMarkerAt(dir); err != nil {
		t.Fatalf("second run unexpected err: %v", err)
	}
	marked2 := listMarked(t, dir)
	// adventurelog + baikal still marked; newly-installed must NOT be.
	if !reflect.DeepEqual(marked2, []string{"adventurelog", "baikal"}) {
		t.Errorf("after sentinel, marks: got %v, want [adventurelog baikal] (newly-installed must not be tagged)", marked2)
	}
}

func TestMigrateLegacyAppMarker_NilSafe(t *testing.T) {
	saved := AppInfo
	AppInfo = nil
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("MigrateLegacyAppMarker panicked on nil AppInfo: %v", r)
		}
		AppInfo = saved
	}()
	MigrateLegacyAppMarker()
}
