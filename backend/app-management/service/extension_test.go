package service

import (
	"testing"

	"github.com/neochaotic/powerlab/backend/app-management/common"
)

// The translation layer is the only thing standing between us and a hard
// rebrand of every store YAML in the wild. These tests pin the priority
// order and make sure adding a new alias never silently changes which
// extension wins on a doc that has more than one.
func TestLookupAppExtension_PriorityOrder(t *testing.T) {
	cases := []struct {
		name    string
		ext     map[string]interface{}
		wantKey string
		wantOK  bool
	}{
		{
			name: "x-powerlab wins over x-web and x-casaos",
			ext: map[string]interface{}{
				common.ComposeExtensionNameXPowerLab: map[string]interface{}{"src": "powerlab"},
				common.ComposeExtensionNameWeb:       map[string]interface{}{"src": "web"},
				common.ComposeExtensionNameXCasaOS:   map[string]interface{}{"src": "casaos"},
			},
			wantKey: common.ComposeExtensionNameXPowerLab,
			wantOK:  true,
		},
		{
			name: "x-web wins when x-powerlab absent",
			ext: map[string]interface{}{
				common.ComposeExtensionNameWeb:     map[string]interface{}{"src": "web"},
				common.ComposeExtensionNameXCasaOS: map[string]interface{}{"src": "casaos"},
			},
			wantKey: common.ComposeExtensionNameWeb,
			wantOK:  true,
		},
		{
			name: "x-casaos used as last resort",
			ext: map[string]interface{}{
				common.ComposeExtensionNameXCasaOS: map[string]interface{}{"src": "casaos"},
			},
			wantKey: common.ComposeExtensionNameXCasaOS,
			wantOK:  true,
		},
		{
			name:    "no recognized extension returns not-found",
			ext:     map[string]interface{}{"x-something-else": map[string]interface{}{}},
			wantKey: "",
			wantOK:  false,
		},
		{
			name:    "nil map returns not-found without panic",
			ext:     nil,
			wantKey: "",
			wantOK:  false,
		},
		{
			name: "explicit nil value is skipped (treated as absent)",
			ext: map[string]interface{}{
				common.ComposeExtensionNameXPowerLab: nil,
				common.ComposeExtensionNameXCasaOS:   map[string]interface{}{"src": "casaos"},
			},
			wantKey: common.ComposeExtensionNameXCasaOS,
			wantOK:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, key, ok := LookupAppExtension(tc.ext)
			if ok != tc.wantOK {
				t.Fatalf("found = %v, want %v", ok, tc.wantOK)
			}
			if key != tc.wantKey {
				t.Fatalf("key = %q, want %q", key, tc.wantKey)
			}
		})
	}
}

func TestLookupAppExtensionMap_ReturnsTypedMap(t *testing.T) {
	ext := map[string]interface{}{
		common.ComposeExtensionNameXPowerLab: map[string]interface{}{
			"port_map": "8080",
			"main":     "nginx",
		},
	}
	m, key, ok := LookupAppExtensionMap(ext)
	if !ok || key != common.ComposeExtensionNameXPowerLab {
		t.Fatalf("expected hit on x-powerlab, got key=%q ok=%v", key, ok)
	}
	if m["port_map"] != "8080" || m["main"] != "nginx" {
		t.Fatalf("unexpected map content: %v", m)
	}
}

// If an author put their extension under x-powerlab as a non-map value (e.g.
// a typo where they wrote a scalar), we should refuse to coerce it — that
// would mask their bug behind a silent fallback.
func TestLookupAppExtensionMap_NonMapValueDoesNotFallThrough(t *testing.T) {
	ext := map[string]interface{}{
		common.ComposeExtensionNameXPowerLab: "this is not a map",
		common.ComposeExtensionNameXCasaOS:   map[string]interface{}{"valid": true},
	}
	_, key, ok := LookupAppExtensionMap(ext)
	// We picked x-powerlab (it was first present) but it's not a map, so
	// we report not-found rather than silently using the x-casaos value.
	// This surfaces the author error.
	if ok {
		t.Fatalf("expected not-found because x-powerlab is not a map, got key=%q", key)
	}
}
