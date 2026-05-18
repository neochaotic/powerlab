package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPruneLegacyAppStores(t *testing.T) {
	cases := []struct {
		name        string
		in          []string
		wantKept    []string
		wantRemoved []string
	}{
		{
			name: "casaos jsdelivr stripped",
			in: []string{
				"https://cdn.jsdelivr.net/gh/IceWhaleTech/CasaOS-AppStore@gh-pages/store/main.zip",
				"/var/lib/powerlab/community-catalog",
			},
			wantKept:    []string{"/var/lib/powerlab/community-catalog"},
			wantRemoved: []string{"https://cdn.jsdelivr.net/gh/IceWhaleTech/CasaOS-AppStore@gh-pages/store/main.zip"},
		},
		{
			name: "big-bear stripped",
			in: []string{
				"https://github.com/bigbeartechworld/big-bear-casaos/archive/refs/heads/master.zip",
				"../../community-catalog",
			},
			wantKept:    []string{"../../community-catalog"},
			wantRemoved: []string{"https://github.com/bigbeartechworld/big-bear-casaos/archive/refs/heads/master.zip"},
		},
		{
			name: "both legacy sources stripped, local kept",
			in: []string{
				"https://cdn.jsdelivr.net/gh/IceWhaleTech/CasaOS-AppStore@gh-pages/store/main.zip",
				"https://github.com/bigbeartechworld/big-bear-casaos/archive/refs/heads/master.zip",
				"/var/lib/powerlab/community-catalog",
			},
			wantKept: []string{"/var/lib/powerlab/community-catalog"},
			wantRemoved: []string{
				"https://cdn.jsdelivr.net/gh/IceWhaleTech/CasaOS-AppStore@gh-pages/store/main.zip",
				"https://github.com/bigbeartechworld/big-bear-casaos/archive/refs/heads/master.zip",
			},
		},
		{
			name: "case-insensitive match",
			in: []string{
				"https://CDN.JSDELIVR.NET/gh/IceWhaleTech/casaos-appstore@gh-pages/store/main.zip",
			},
			wantKept:    []string{},
			wantRemoved: []string{"https://CDN.JSDELIVR.NET/gh/IceWhaleTech/casaos-appstore@gh-pages/store/main.zip"},
		},
		{
			name:        "empty input → empty output",
			in:          []string{},
			wantKept:    []string{},
			wantRemoved: nil,
		},
		{
			name: "no legacy URLs → all kept, none removed",
			in: []string{
				"/var/lib/powerlab/community-catalog",
				"https://example.com/operator-custom-store.zip",
			},
			wantKept: []string{
				"/var/lib/powerlab/community-catalog",
				"https://example.com/operator-custom-store.zip",
			},
			wantRemoved: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kept, removed := pruneLegacyAppStores(tc.in)
			if !reflect.DeepEqual(kept, tc.wantKept) {
				t.Errorf("kept: got %v, want %v", kept, tc.wantKept)
			}
			if !reflect.DeepEqual(removed, tc.wantRemoved) {
				t.Errorf("removed: got %v, want %v", removed, tc.wantRemoved)
			}
		})
	}
}

func TestPruneLegacyAppStores_DoesNotMutateInput(t *testing.T) {
	orig := []string{
		"https://cdn.jsdelivr.net/gh/IceWhaleTech/CasaOS-AppStore@gh-pages/store/main.zip",
		"/var/lib/powerlab/community-catalog",
	}
	origCopy := make([]string, len(orig))
	copy(origCopy, orig)
	_, _ = pruneLegacyAppStores(orig)
	if !reflect.DeepEqual(orig, origCopy) {
		t.Errorf("input mutated; expected pure function. got %v, original %v", orig, origCopy)
	}
}

func TestIsLegacyCatalogStore(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"https://cdn.jsdelivr.net/gh/IceWhaleTech/CasaOS-AppStore@gh-pages/store/main.zip", true},
		{"https://github.com/IceWhaleTech/CasaOS-AppStore/archive/refs/heads/main.zip", true},
		{"https://github.com/bigbeartechworld/big-bear-casaos/archive/refs/heads/master.zip", true},
		{"/var/lib/powerlab/community-catalog", false},
		{"../../community-catalog", false},
		{"https://example.com/operator-custom-store.zip", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.url, func(t *testing.T) {
			if got := isLegacyCatalogStore(tc.url); got != tc.want {
				t.Errorf("isLegacyCatalogStore(%q) = %v, want %v", tc.url, got, tc.want)
			}
		})
	}
}

// ─── Workdir RMDir (#450) ─────────────────────────────────────────
// The original migration only stripped legacy URLs from
// ServerInfo.AppStoreList. The on-disk workdirs (e.g.
// /var/lib/powerlab/appstore/cdn.jsdelivr.net/<md5>/) survived,
// continuing to occupy disk and showing up in any `ls` operator
// audit. UnregisterAppStore in service/appstore_management.go does
// RMDir the workdir when an entry is unregistered via the UI; the
// migration should reach parity.

func TestLegacyURLWorkDir_URLForm(t *testing.T) {
	// MD5("/gh/icewhaletech/casaos-appstore@gh-pages/store/main.zip") =
	// computed once for the test constant. The function should produce
	// <appStorePath>/<host>/<md5(lowercase path)>.
	got := legacyURLWorkDir(
		"https://cdn.jsdelivr.net/gh/IceWhaleTech/CasaOS-AppStore@gh-pages/store/main.zip",
		"/var/lib/powerlab/appstore",
	)
	// Expect host segment + md5 hex; verify shape rather than re-hashing
	// the input (which would just duplicate the production formula).
	if got == "" {
		t.Fatal("expected non-empty workdir path")
	}
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q", got)
	}
	if !contains(got, "cdn.jsdelivr.net") {
		t.Errorf("expected host segment 'cdn.jsdelivr.net' in path, got %q", got)
	}
	if filepath.Dir(filepath.Dir(got)) != "/var/lib/powerlab/appstore" {
		t.Errorf("expected appStorePath as grandparent, got %q (grandparent %q)", got, filepath.Dir(filepath.Dir(got)))
	}
}

func TestLegacyURLWorkDir_LocalPathReturnsEmpty(t *testing.T) {
	// Local-path entries (e.g. /var/lib/powerlab/community-catalog)
	// don't have a separate workdir — the path IS the workdir. The
	// helper should return "" so the caller skips the RMDir step for
	// them.
	got := legacyURLWorkDir("/var/lib/powerlab/community-catalog", "/var/lib/powerlab/appstore")
	if got != "" {
		t.Errorf("expected empty workdir for local path, got %q", got)
	}
}

func TestLegacyURLWorkDir_FileURLReturnsEmpty(t *testing.T) {
	got := legacyURLWorkDir("file:///var/lib/powerlab/community-catalog", "/var/lib/powerlab/appstore")
	if got != "" {
		t.Errorf("expected empty workdir for file:// URL, got %q", got)
	}
}

func TestLegacyURLWorkDir_InvalidURLReturnsEmpty(t *testing.T) {
	got := legacyURLWorkDir("not a url at all", "/var/lib/powerlab/appstore")
	// Implementation choice: invalid input returns "" so the migration
	// silently skips it rather than producing a junk path.
	if got != "" {
		t.Errorf("expected empty workdir for invalid url, got %q", got)
	}
}

func TestRemoveLegacyWorkDirs_RemovesURLDirs(t *testing.T) {
	// Set up a fake appstore root with a workdir matching the legacy
	// URL's predicted layout, plus a non-legacy workdir that must
	// survive. The migration helper should remove only the legacy one.
	appStorePath := t.TempDir()
	legacyURL := "https://cdn.jsdelivr.net/gh/IceWhaleTech/CasaOS-AppStore@gh-pages/store/main.zip"
	keepURL := "https://example.com/operator-custom-store.zip"

	legacyDir := legacyURLWorkDir(legacyURL, appStorePath)
	keepDir := legacyURLWorkDir(keepURL, appStorePath)
	for _, d := range []string{legacyDir, keepDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "marker"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	removeLegacyWorkDirs(appStorePath, []string{legacyURL})

	if _, err := os.Stat(legacyDir); !os.IsNotExist(err) {
		t.Errorf("expected legacy workdir %q removed, stat err = %v", legacyDir, err)
	}
	if _, err := os.Stat(keepDir); err != nil {
		t.Errorf("non-legacy workdir %q was wrongly removed: %v", keepDir, err)
	}
}

func TestRemoveLegacyWorkDirs_NoOpOnEmpty(t *testing.T) {
	appStorePath := t.TempDir()
	// Must not panic, must not create anything.
	removeLegacyWorkDirs(appStorePath, nil)
	removeLegacyWorkDirs(appStorePath, []string{})
}

func TestRemoveLegacyWorkDirs_MissingDirSilent(t *testing.T) {
	// If the workdir doesn't exist (clean install, already-cleaned box),
	// the helper must not error out — just log and move on.
	appStorePath := t.TempDir()
	removeLegacyWorkDirs(appStorePath, []string{
		"https://cdn.jsdelivr.net/gh/IceWhaleTech/CasaOS-AppStore@gh-pages/store/main.zip",
	})
	// No assertion needed — the test passes if no panic and no t.Fatal.
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (s == sub || hasSubstring(s, sub)) }
func hasSubstring(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestMigrateAppStoreListLegacyRemoval_NilSafe(t *testing.T) {
	// Defensive: function must not panic when called before
	// ServerInfo is populated (early-init path, tests, etc).
	saved := ServerInfo
	ServerInfo = nil
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("MigrateAppStoreListLegacyRemoval panicked on nil ServerInfo: %v", r)
		}
		ServerInfo = saved
	}()
	MigrateAppStoreListLegacyRemoval()
}
