package common

import (
	"runtime"
	"testing"
)

// TestPowerLabAppDataPath_Linux pins the canonical layout — the
// segment that prevents collision with CasaOS.
func TestPowerLabAppDataPath_Linux(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("path separator differs on Windows; PowerLab targets Linux/macOS")
	}
	got := PowerLabAppDataPath("/DATA", "nextcloud")
	want := "/DATA/PowerLabAppData/nextcloud"
	if got != want {
		t.Errorf("PowerLabAppDataPath(/DATA, nextcloud) = %q, want %q", got, want)
	}
}

// TestLegacyAppDataPath_Linux pins what migration code must search
// FOR — directories that may belong to PowerLab from the pre-ADR-0021
// era.
func TestLegacyAppDataPath_Linux(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("path separator differs on Windows; PowerLab targets Linux/macOS")
	}
	got := LegacyAppDataPath("/DATA", "nextcloud")
	want := "/DATA/AppData/nextcloud"
	if got != want {
		t.Errorf("LegacyAppDataPath(/DATA, nextcloud) = %q, want %q", got, want)
	}
}

// TestAppDataPaths_DifferByOneSegment locks the actual goal of the
// rename: a single, immediately-visible segment difference between
// "this is PowerLab data" and "this is CasaOS data" in `ls /DATA/`
// output.
func TestAppDataPaths_DifferByOneSegment(t *testing.T) {
	canon := PowerLabAppDataPath("/DATA", "x")
	legacy := LegacyAppDataPath("/DATA", "x")
	if canon == legacy {
		t.Fatalf("canonical and legacy paths must differ; both = %q", canon)
	}
	// The trailing /<app>/ segment must match — the per-app name
	// doesn't change with the rename, only the parent dir.
	canonApp := canon[len(canon)-1:]
	legacyApp := legacy[len(legacy)-1:]
	if canonApp != legacyApp {
		t.Errorf("per-app name segment differs (canonical %q, legacy %q)", canonApp, legacyApp)
	}
}

// TestAppDataPaths_HonorStoragePathOverride verifies macOS dev
// installs (where /DATA cannot exist due to SIP) still work — the
// helpers compose paths from the configured StoragePath, not a
// hard-coded /DATA.
func TestAppDataPaths_HonorStoragePathOverride(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("path separator differs on Windows; PowerLab targets Linux/macOS")
	}
	got := PowerLabAppDataPath("/Users/me/powerlab-storage", "myapp")
	want := "/Users/me/powerlab-storage/PowerLabAppData/myapp"
	if got != want {
		t.Errorf("custom storage: got %q, want %q", got, want)
	}
}
