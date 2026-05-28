package journal

import (
	"os"
	"path/filepath"
	"testing"
)

// A dev box without /etc/systemd/system (Mac, container without
// systemd installed) must yield (nil, nil) — the resource degrades
// gracefully; it doesn't fail.
func TestListUnits_MissingDirIsEmptyNotError(t *testing.T) {
	got, err := ListUnits(filepath.Join(t.TempDir(), "no-such-dir"))
	if err != nil {
		t.Fatalf("missing systemd dir returned err %v; want nil", err)
	}
	if got != nil {
		t.Fatalf("missing systemd dir returned %d units; want nil", len(got))
	}
}

// The installer drops one powerlab-<svc>.service per service. ListUnits
// must return their stems (no powerlab- prefix, no .service suffix) so
// an agent can pipe them straight back into journal://{unit} —
// canonicalUnit re-applies the prefix/suffix.
func TestListUnits_ReturnsStrippedStemsSorted(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"powerlab-gateway.service",
		"powerlab-message-bus.service",
		"powerlab-core.service",
		"powerlab-user-service.service",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("# unit\n"), 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
	}
	got, err := ListUnits(dir)
	if err != nil {
		t.Fatalf("ListUnits: %v", err)
	}
	want := []string{"core", "gateway", "message-bus", "user-service"}
	if len(got) != len(want) {
		t.Fatalf("got %v; want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v; want %v (sorted)", got, want)
		}
	}
}

// /etc/systemd/system holds a forest of units — the file watcher's
// .timer, system targets, sockets, anything an operator dropped in
// manually. ListUnits must filter out everything that isn't a
// powerlab-*.service file — an agent must not learn about
// powerlab-mcp.timer if that ever ships, only the running .service
// units it can actually pull a journal:// read against.
func TestListUnits_IgnoresNonPowerlabAndNonServiceEntries(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"powerlab-gateway.service",   // ✓ valid, expect "gateway"
		"powerlab-mcp.timer",         // ✗ not a .service
		"powerlab-anything.target",   // ✗ not a .service
		"casaos.service",             // ✗ wrong prefix (not powerlab-)
		"systemd-networkd.service",   // ✗ wrong prefix
		"powerlab-.service",          // ✗ empty stem
		"powerlab-foo.service.broken", // ✗ wrong suffix
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("# unit\n"), 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
	}
	got, err := ListUnits(dir)
	if err != nil {
		t.Fatalf("ListUnits: %v", err)
	}
	if len(got) != 1 || got[0] != "gateway" {
		t.Fatalf("got %v; want exactly [gateway]", got)
	}
}

// systemd symlinks are common (a unit installed via systemctl link).
// Duplicate stems must dedupe — the agent shouldn't see "gateway"
// twice just because someone enabled it.
func TestListUnits_DedupesByStem(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"powerlab-gateway.service",
		"powerlab-gateway.service.bak", // not picked up (wrong suffix)
	} {
		_ = os.WriteFile(filepath.Join(dir, name), []byte("# unit\n"), 0o600)
	}
	// Create a second file with the same stem to simulate a duplicate
	// (in practice systemd would symlink — same effect on ReadDir).
	if err := os.WriteFile(filepath.Join(dir, "powerlab-gateway.service.duplicate"), []byte("# unit\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	got, err := ListUnits(dir)
	if err != nil {
		t.Fatalf("ListUnits: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %v; want exactly 1 entry (deduped by stem)", got)
	}
}
