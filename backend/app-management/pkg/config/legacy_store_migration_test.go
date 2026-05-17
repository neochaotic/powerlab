package config

import (
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
