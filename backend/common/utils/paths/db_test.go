package paths

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Tests for the split-brain detector + the canonical/legacy path
// helpers (issue #179, ADR follow-up). Per memory rule "bug fix =
// regression test, no exceptions" these lock in the contract:
//
//   - Helpers return the documented paths
//   - AssertNoSplitBrain returns nil with 0 or 1 paths present
//   - AssertNoSplitBrain returns ErrSplitBrain when 2+ exist
//   - Empty files are NOT counted as present (SQLite-just-initialised
//     edge case)
//   - Empty path strings are skipped silently
//
// The tests stub `dataRoot` to a temp dir so they don't touch /var/lib.

func setDataRoot(t *testing.T, dir string) {
	t.Helper()
	orig := dataRoot
	dataRoot = func() string { return dir }
	t.Cleanup(func() { dataRoot = orig })
}

// TestCanonicalPaths_StableContract pins the going-forward layout so
// a future maintainer changing convention has to update these
// assertions deliberately.
func TestCanonicalPaths_StableContract(t *testing.T) {
	setDataRoot(t, "/test/root")
	cases := []struct {
		name   string
		got    string
		want   string
	}{
		{"CanonicalUserServiceDB", CanonicalUserServiceDB(), "/test/root/user.db"},
		{"CanonicalCoreDB", CanonicalCoreDB(), "/test/root/core.db"},
		{"CanonicalLocalStorageDB", CanonicalLocalStorageDB(), "/test/root/local-storage.db"},
		{"CanonicalMessageBusDB", CanonicalMessageBusDB(), "/test/root/message-bus.db"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

// TestLegacyPaths_KnowWhereTheBodiesAreBuried documents (and locks)
// every legacy path the helpers know about so the migration code
// always knows what to migrate FROM.
func TestLegacyPaths_KnowWhereTheBodiesAreBuried(t *testing.T) {
	setDataRoot(t, "/test/root")
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"LegacyUserServiceDB", LegacyUserServiceDB(), "/test/root/db/user.db"},
		{"LegacyCoreDB", LegacyCoreDB(), "/test/root/db/casaOS.db"},
		{"LegacyCasaOSCoreDB", LegacyCasaOSCoreDB(), "/var/lib/casaos/db/casaOS.db"},
		{"LegacyLocalStorageDB", LegacyLocalStorageDB(), "/test/root/db/local-storage.db"},
		{"LegacyMessageBusDB", LegacyMessageBusDB(), "/test/root/db/message-bus.db"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

// TestAssertNoSplitBrain_NoPathsExist locks the trivial-case contract.
func TestAssertNoSplitBrain_NoPathsExist(t *testing.T) {
	tmp := t.TempDir()
	err := AssertNoSplitBrain(context.Background(), nil, "test-svc",
		filepath.Join(tmp, "missing-1.db"),
		filepath.Join(tmp, "missing-2.db"),
	)
	if err != nil {
		t.Errorf("expected nil for no-paths-exist, got: %v", err)
	}
}

// TestAssertNoSplitBrain_OnePathExists locks the happy path: one DB
// file is present, the other isn't, no split brain.
func TestAssertNoSplitBrain_OnePathExists(t *testing.T) {
	tmp := t.TempDir()
	canonical := filepath.Join(tmp, "user.db")
	legacy := filepath.Join(tmp, "db", "user.db")

	if err := os.WriteFile(canonical, []byte("real db data"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	err := AssertNoSplitBrain(context.Background(), nil, "user-service", canonical, legacy)
	if err != nil {
		t.Errorf("expected nil with one path present, got: %v", err)
	}
}

// TestAssertNoSplitBrain_TwoPathsExist locks the THE regression for
// #179 — when the user-service hot-fix copy AND the canonical path
// both exist with data, refuse to start.
func TestAssertNoSplitBrain_TwoPathsExist(t *testing.T) {
	tmp := t.TempDir()
	canonical := filepath.Join(tmp, "user.db")
	legacy := filepath.Join(tmp, "db", "user.db")

	if err := os.WriteFile(canonical, []byte("active data"), 0o644); err != nil {
		t.Fatalf("setup canonical: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatalf("setup legacy dir: %v", err)
	}
	if err := os.WriteFile(legacy, []byte("stale duplicate"), 0o644); err != nil {
		t.Fatalf("setup legacy: %v", err)
	}

	err := AssertNoSplitBrain(context.Background(), nil, "user-service", canonical, legacy)
	if err == nil {
		t.Fatal("expected ErrSplitBrain when both paths exist")
	}
	if !errors.Is(err, ErrSplitBrain) {
		t.Errorf("expected ErrSplitBrain wrapped error, got: %v", err)
	}
	// Both paths must appear in the error message so the operator
	// knows what to clean up.
	if !strings.Contains(err.Error(), canonical) {
		t.Errorf("error must name the canonical path %q, got: %v", canonical, err)
	}
	if !strings.Contains(err.Error(), legacy) {
		t.Errorf("error must name the legacy path %q, got: %v", legacy, err)
	}
	// Service name must appear so multi-service installs know which
	// service is misconfigured.
	if !strings.Contains(err.Error(), "user-service") {
		t.Errorf("error must name the service, got: %v", err)
	}
}

// TestAssertNoSplitBrain_EmptyFileIsNotCounted locks the SQLite
// gotcha: when sqlite.Open creates a new file, it's 0 bytes until
// the first migration runs. A zero-byte file alongside a real one is
// NOT split brain — only the real one matters.
func TestAssertNoSplitBrain_EmptyFileIsNotCounted(t *testing.T) {
	tmp := t.TempDir()
	canonical := filepath.Join(tmp, "user.db")
	legacy := filepath.Join(tmp, "db", "user.db")

	if err := os.WriteFile(canonical, []byte("real data"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(legacy, []byte{}, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	err := AssertNoSplitBrain(context.Background(), nil, "user-service", canonical, legacy)
	if err != nil {
		t.Errorf("expected nil when legacy is empty (sqlite-just-initialised case), got: %v", err)
	}
}

// TestAssertNoSplitBrain_EmptyPathStringSkipped locks the convenience
// behavior: callers can pass conditional alternates ("" when N/A)
// without first checking. Empty string is silently ignored.
func TestAssertNoSplitBrain_EmptyPathStringSkipped(t *testing.T) {
	tmp := t.TempDir()
	canonical := filepath.Join(tmp, "user.db")
	if err := os.WriteFile(canonical, []byte("data"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	err := AssertNoSplitBrain(context.Background(), nil, "test-svc", canonical, "", "")
	if err != nil {
		t.Errorf("expected nil with empty path strings, got: %v", err)
	}
}

// TestAssertNoSplitBrain_ThreeOrMorePaths covers core's case where
// canonical + LegacyCoreDB + LegacyCasaOSCoreDB are three separate
// candidates. Any 2+ being present must trigger.
func TestAssertNoSplitBrain_ThreeOrMorePaths(t *testing.T) {
	tmp := t.TempDir()
	a := filepath.Join(tmp, "a.db")
	b := filepath.Join(tmp, "b.db")
	c := filepath.Join(tmp, "c.db")

	if err := os.WriteFile(a, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := AssertNoSplitBrain(context.Background(), nil, "core", a, b, c)
	if err == nil {
		t.Fatal("expected error with 2 of 3 paths present")
	}
	if !errors.Is(err, ErrSplitBrain) {
		t.Errorf("expected ErrSplitBrain, got: %v", err)
	}
}

// AutoMoveLegacyAside tests — the v0.5.9 fix for the v0.5.8 lock-out
// regression. When user-service or local-storage finds a stale legacy
// duplicate at <DataPath>/db/<svc>.db (sobra do v0.5.4 hot-fix mishap),
// the helper moves it aside as <legacy>.bak.<ts> so split-brain doesn't
// trigger. Per memory rule "bug fix = regression test, no exceptions".

func stubNowUnix(t *testing.T, value int64) {
	t.Helper()
	orig := nowUnix
	nowUnix = func() int64 { return value }
	t.Cleanup(func() { nowUnix = orig })
}

// TestAutoMoveLegacyAside_MovesStaleDuplicate — THE v0.5.8 lock-out
// regression. Both canonical and legacy exist with data; legacy must
// be renamed to .bak.<ts>, canonical must stay untouched.
func TestAutoMoveLegacyAside_MovesStaleDuplicate(t *testing.T) {
	tmp := t.TempDir()
	canonical := filepath.Join(tmp, "user.db")
	legacy := filepath.Join(tmp, "db", "user.db")

	if err := os.WriteFile(canonical, []byte("authoritative data"), 0o644); err != nil {
		t.Fatalf("setup canonical: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatalf("setup legacy dir: %v", err)
	}
	if err := os.WriteFile(legacy, []byte("stale junk"), 0o644); err != nil {
		t.Fatalf("setup legacy: %v", err)
	}

	stubNowUnix(t, 1234567890)
	moved := AutoMoveLegacyAside(context.Background(), nil, "user-service", canonical, legacy)

	if len(moved) != 1 {
		t.Fatalf("expected 1 path moved, got %d: %v", len(moved), moved)
	}
	expectedBak := legacy + ".bak.1234567890"
	if moved[0] != expectedBak {
		t.Errorf("moved[0] = %q, want %q", moved[0], expectedBak)
	}

	// Legacy original gone
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("legacy still at original path; should be moved")
	}
	// .bak file exists with original content
	if got, err := os.ReadFile(expectedBak); err != nil || string(got) != "stale junk" {
		t.Errorf("backup content corrupt; got %q err=%v", got, err)
	}
	// Canonical untouched
	if got, err := os.ReadFile(canonical); err != nil || string(got) != "authoritative data" {
		t.Errorf("canonical disturbed; got %q err=%v", got, err)
	}

	// Subsequent call: legacy is gone now, no-op
	moved2 := AutoMoveLegacyAside(context.Background(), nil, "user-service", canonical, legacy)
	if len(moved2) != 0 {
		t.Errorf("idempotent re-run should move nothing; got %v", moved2)
	}
}

// TestAutoMoveLegacyAside_NoOpWhenCanonicalMissing — if canonical
// doesn't exist, legacy might be the only copy with real data. DON'T
// move it; let the natural fresh-install path handle it (or
// AssertNoSplitBrain fall through with a single path).
func TestAutoMoveLegacyAside_NoOpWhenCanonicalMissing(t *testing.T) {
	tmp := t.TempDir()
	canonical := filepath.Join(tmp, "user.db")
	legacy := filepath.Join(tmp, "db", "user.db")

	// Only legacy exists
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(legacy, []byte("possibly real data"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	moved := AutoMoveLegacyAside(context.Background(), nil, "user-service", canonical, legacy)
	if len(moved) != 0 {
		t.Errorf("expected no moves when canonical missing; got %v", moved)
	}

	// Legacy untouched
	if got, err := os.ReadFile(legacy); err != nil || string(got) != "possibly real data" {
		t.Errorf("legacy disturbed; got %q err=%v", got, err)
	}
}

// TestAutoMoveLegacyAside_NoOpWhenCanonicalEmpty — canonical exists
// but is 0 bytes (sqlite-just-initialised case). Same logic as missing:
// don't auto-move legacy, let the natural path play out.
func TestAutoMoveLegacyAside_NoOpWhenCanonicalEmpty(t *testing.T) {
	tmp := t.TempDir()
	canonical := filepath.Join(tmp, "user.db")
	legacy := filepath.Join(tmp, "db", "user.db")

	if err := os.WriteFile(canonical, []byte{}, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(legacy, []byte("data"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	moved := AutoMoveLegacyAside(context.Background(), nil, "user-service", canonical, legacy)
	if len(moved) != 0 {
		t.Errorf("expected no moves when canonical empty; got %v", moved)
	}
}

// TestAutoMoveLegacyAside_NoOpWhenLegacyMissing — fresh install case:
// no legacy file at all. Returns empty, no error.
func TestAutoMoveLegacyAside_NoOpWhenLegacyMissing(t *testing.T) {
	tmp := t.TempDir()
	canonical := filepath.Join(tmp, "user.db")
	legacy := filepath.Join(tmp, "db", "user.db")

	if err := os.WriteFile(canonical, []byte("data"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	moved := AutoMoveLegacyAside(context.Background(), nil, "user-service", canonical, legacy)
	if len(moved) != 0 {
		t.Errorf("expected no moves when legacy missing; got %v", moved)
	}
}

// TestAutoMoveLegacyAside_SkipsEmptyAndDuplicatePaths — convenience:
// empty path strings + canonical-equal-to-legacy are silently skipped.
func TestAutoMoveLegacyAside_SkipsEmptyAndDuplicatePaths(t *testing.T) {
	tmp := t.TempDir()
	canonical := filepath.Join(tmp, "user.db")
	if err := os.WriteFile(canonical, []byte("data"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	moved := AutoMoveLegacyAside(context.Background(), nil, "test", canonical, "", canonical)
	if len(moved) != 0 {
		t.Errorf("expected no moves when alternates are empty/duplicate of canonical; got %v", moved)
	}
}

// TestAutoMoveLegacyAside_MultipleLegacyPaths — caller can pass multiple
// known-stale legacy paths in one call; each gets its own .bak.<ts>.
func TestAutoMoveLegacyAside_MultipleLegacyPaths(t *testing.T) {
	tmp := t.TempDir()
	canonical := filepath.Join(tmp, "user.db")
	legacy1 := filepath.Join(tmp, "db", "user.db")
	legacy2 := filepath.Join(tmp, "old", "user.db")

	if err := os.WriteFile(canonical, []byte("data"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	for _, p := range []string{legacy1, legacy2} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.WriteFile(p, []byte("stale"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	stubNowUnix(t, 99)
	moved := AutoMoveLegacyAside(context.Background(), nil, "test", canonical, legacy1, legacy2)

	if len(moved) != 2 {
		t.Errorf("expected 2 paths moved; got %v", moved)
	}
}

// TestAutoMoveLegacyAside_AfterAssertNoSplitBrainPasses — the v0.5.9
// integration contract: calling AutoMoveLegacyAside FIRST means a
// subsequent AssertNoSplitBrain call sees only the canonical path
// (legacy moved aside). This is the intended call sequence in
// service main.go.
func TestAutoMoveLegacyAside_AfterAssertNoSplitBrainPasses(t *testing.T) {
	tmp := t.TempDir()
	canonical := filepath.Join(tmp, "user.db")
	legacy := filepath.Join(tmp, "db", "user.db")

	if err := os.WriteFile(canonical, []byte("data"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(legacy, []byte("stale"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// BEFORE auto-move: AssertNoSplitBrain would fail
	if err := AssertNoSplitBrain(context.Background(), nil, "test", canonical, legacy); err == nil {
		t.Fatal("setup wrong: expected split-brain error before auto-move")
	}

	// Auto-move stale legacy aside
	moved := AutoMoveLegacyAside(context.Background(), nil, "test", canonical, legacy)
	if len(moved) != 1 {
		t.Fatalf("expected 1 move; got %v", moved)
	}

	// AFTER auto-move: AssertNoSplitBrain passes
	if err := AssertNoSplitBrain(context.Background(), nil, "test", canonical, legacy); err != nil {
		t.Errorf("expected no error after auto-move; got %v", err)
	}
}
