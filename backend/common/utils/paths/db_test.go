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
