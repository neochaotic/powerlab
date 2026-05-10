package common

import (
	"testing"
)

// Tests pin the ADR-0021 contract: dual-read accepts either naming,
// dual-write produces both. Per memory rule "TDD strict — tests first"
// these assertions are the executable spec for every label call site
// the next sub-PR rewrites.

// TestIsPowerLabApp_AcceptsCanonicalSentinel — new containers (created
// after the wiring PR) will have only the canonical sentinel.
func TestIsPowerLabApp_AcceptsCanonicalSentinel(t *testing.T) {
	labels := map[string]string{
		LabelKindKey: LabelKindValueApp,
	}
	if !IsPowerLabApp(labels) {
		t.Errorf("expected IsPowerLabApp=true for canonical sentinel, got false")
	}
}

// TestIsPowerLabApp_AcceptsLegacySentinel — existing PowerLab containers
// (created before this PR) carry only the legacy sentinel. They must
// stay visible through the upgrade.
func TestIsPowerLabApp_AcceptsLegacySentinel(t *testing.T) {
	labels := map[string]string{
		LegacyLabelKindKey: LegacyLabelKindValueApp,
	}
	if !IsPowerLabApp(labels) {
		t.Errorf("expected IsPowerLabApp=true for legacy sentinel, got false")
	}
}

// TestIsPowerLabApp_AcceptsBoth — during the dual-write window every
// new container carries both. Both present must still return true
// (not double-positive or any weird AND semantics).
func TestIsPowerLabApp_AcceptsBoth(t *testing.T) {
	labels := map[string]string{
		LabelKindKey:       LabelKindValueApp,
		LegacyLabelKindKey: LegacyLabelKindValueApp,
	}
	if !IsPowerLabApp(labels) {
		t.Errorf("expected IsPowerLabApp=true with both sentinels, got false")
	}
}

// TestIsPowerLabApp_RejectsArbitraryContainers — random non-PowerLab
// containers must not be claimed. THIS is the original-sin fix: the
// pre-ADR code claimed every CasaOS container as PowerLab's, and vice
// versa. After the wiring PR, neither product touches the other's.
func TestIsPowerLabApp_RejectsArbitraryContainers(t *testing.T) {
	cases := []map[string]string{
		nil,
		{},
		{"unrelated": "value"},
		{LabelKindKey: "not-app"},
		{LegacyLabelKindKey: "not-casaos"},
		{"random.label": "powerlab"},
	}
	for i, labels := range cases {
		if IsPowerLabApp(labels) {
			t.Errorf("case %d: expected IsPowerLabApp=false for %v", i, labels)
		}
	}
}

// TestLabelValue_PrefersCanonical — if both keys are set with
// different values (a corrupt-state edge case), canonical wins.
func TestLabelValue_PrefersCanonical(t *testing.T) {
	labels := map[string]string{
		LabelIconKey:       "canonical-icon.png",
		LegacyLabelIconKey: "legacy-icon.png",
	}
	got := LabelValue(labels, LabelIconKey)
	if got != "canonical-icon.png" {
		t.Errorf("LabelValue(icon) = %q, want canonical preferred", got)
	}
}

// TestLabelValue_FallsBackToLegacy — pre-PR containers only have the
// legacy key. The reader must find it via the canonical query.
func TestLabelValue_FallsBackToLegacy(t *testing.T) {
	labels := map[string]string{
		LegacyLabelIconKey: "icon-from-legacy.png",
	}
	got := LabelValue(labels, LabelIconKey)
	if got != "icon-from-legacy.png" {
		t.Errorf("LabelValue(icon) = %q, want legacy fallback", got)
	}
}

// TestLabelValue_EmptyCanonicalFallsBack — an explicit empty string
// at the canonical key shouldn't shadow a meaningful legacy value.
// The compose generator may produce empty strings for fields the
// AppLabels left blank; if a partial set ever gets written, the
// dual-read must still surface non-empty data from the legacy side.
func TestLabelValue_EmptyCanonicalFallsBack(t *testing.T) {
	labels := map[string]string{
		LabelIconKey:       "",
		LegacyLabelIconKey: "icon-from-legacy.png",
	}
	got := LabelValue(labels, LabelIconKey)
	if got != "icon-from-legacy.png" {
		t.Errorf("LabelValue(icon) = %q, want legacy fallback when canonical is empty", got)
	}
}

// TestLabelValue_ReturnsEmptyWhenAbsent — neither key set → "".
func TestLabelValue_ReturnsEmptyWhenAbsent(t *testing.T) {
	labels := map[string]string{}
	if got := LabelValue(labels, LabelIconKey); got != "" {
		t.Errorf("LabelValue on empty map = %q, want empty string", got)
	}
	if got := LabelValue(nil, LabelIconKey); got != "" {
		t.Errorf("LabelValue(nil) = %q, want empty string", got)
	}
}

// TestBuildLabels_ProducesBothNamespaces — the dual-write contract
// from ADR-0021. Every populated AppLabels field appears under BOTH
// the canonical and legacy keys.
func TestBuildLabels_ProducesBothNamespaces(t *testing.T) {
	out := BuildLabels(AppLabels{
		Origin:      "system",
		WebPort:     "8080",
		Icon:        "icon.png",
		Description: "desc",
		WebIndex:    "/",
		CustomID:    "custom-1",
		ShowEnv:     "FOO,BAR",
		Protocol:    "http",
		Host:        "myapp.local",
		Name:        "MyApp",
		AppStoreID:  "42",
	})

	pairs := []struct {
		canonical, legacy, expected string
	}{
		{LabelKindKey, LegacyLabelKindKey, ""}, // sentinel — values differ
		{LabelOriginKey, LegacyLabelOriginKey, "system"},
		{LabelWebPortKey, LegacyLabelWebPortKey, "8080"},
		{LabelIconKey, LegacyLabelIconKey, "icon.png"},
		{LabelDescriptionKey, LegacyLabelDescriptionKey, "desc"},
		{LabelWebIndexKey, LegacyLabelWebIndexKey, "/"},
		{LabelCustomIDKey, LegacyLabelCustomIDKey, "custom-1"},
		{LabelShowEnvKey, LegacyLabelShowEnvKey, "FOO,BAR"},
		{LabelProtocolKey, LegacyLabelProtocolKey, "http"},
		{LabelHostKey, LegacyLabelHostKey, "myapp.local"},
		{LabelNameKey, LegacyLabelNameKey, "MyApp"},
		{LabelAppStoreIDKey, LegacyLabelAppStoreIDKey, "42"},
	}
	for _, p := range pairs {
		if p.expected == "" {
			// sentinel — just assert presence + correct kind values
			if out[p.canonical] != LabelKindValueApp {
				t.Errorf("canonical kind: got %q, want %q", out[p.canonical], LabelKindValueApp)
			}
			if out[p.legacy] != LegacyLabelKindValueApp {
				t.Errorf("legacy kind: got %q, want %q", out[p.legacy], LegacyLabelKindValueApp)
			}
			continue
		}
		if out[p.canonical] != p.expected {
			t.Errorf("canonical %q = %q, want %q", p.canonical, out[p.canonical], p.expected)
		}
		if out[p.legacy] != p.expected {
			t.Errorf("legacy %q = %q, want %q", p.legacy, out[p.legacy], p.expected)
		}
	}
}

// TestBuildLabels_OmitsEmptyFields — empty AppLabels fields don't
// produce empty-string label entries on either side. Container labels
// with empty values are noise + slow down filter ops.
func TestBuildLabels_OmitsEmptyFields(t *testing.T) {
	out := BuildLabels(AppLabels{
		Origin: "system",
		// every other field empty
	})

	mustHave := []string{LabelKindKey, LegacyLabelKindKey, LabelOriginKey, LegacyLabelOriginKey}
	for _, k := range mustHave {
		if _, ok := out[k]; !ok {
			t.Errorf("expected key %q to be present", k)
		}
	}

	mustNotHave := []string{
		LabelIconKey, LegacyLabelIconKey,
		LabelDescriptionKey, LegacyLabelDescriptionKey,
		LabelHostKey, LegacyLabelHostKey,
		LabelAppStoreIDKey, LegacyLabelAppStoreIDKey,
	}
	for _, k := range mustNotHave {
		if v, ok := out[k]; ok {
			t.Errorf("expected key %q to be ABSENT, got value %q", k, v)
		}
	}
}

// TestBuildLabels_AlwaysIncludesSentinel — the kind sentinel is the
// "is mine" filter. It MUST be in every label map BuildLabels
// returns, even when AppLabels is entirely empty.
func TestBuildLabels_AlwaysIncludesSentinel(t *testing.T) {
	out := BuildLabels(AppLabels{})
	if out[LabelKindKey] != LabelKindValueApp {
		t.Errorf("canonical sentinel missing for empty AppLabels")
	}
	if out[LegacyLabelKindKey] != LegacyLabelKindValueApp {
		t.Errorf("legacy sentinel missing for empty AppLabels")
	}
	// IsPowerLabApp on the result must agree.
	if !IsPowerLabApp(out) {
		t.Errorf("BuildLabels output should pass IsPowerLabApp filter")
	}
}

// TestLegacyKeyFor_CompletenessGuard — every canonical key constant
// MUST have a legacy mapping. If a future maintainer adds a canonical
// constant but forgets to map it, this test catches the gap.
func TestLegacyKeyFor_CompletenessGuard(t *testing.T) {
	// Excludes LabelKindKey because IsPowerLabApp handles the sentinel
	// directly with both constants — legacyKeyFor must STILL map it
	// for symmetry with LabelValue lookups (an operator inspecting
	// kind via LabelValue should fall through correctly).
	allCanonical := []string{
		LabelKindKey,
		LabelOriginKey,
		LabelWebPortKey,
		LabelIconKey,
		LabelDescriptionKey,
		LabelWebIndexKey,
		LabelCustomIDKey,
		LabelShowEnvKey,
		LabelProtocolKey,
		LabelHostKey,
		LabelNameKey,
		LabelAppStoreIDKey,
	}
	for _, k := range allCanonical {
		if legacyKeyFor(k) == "" {
			t.Errorf("canonical key %q has no legacy mapping in legacyKeyFor", k)
		}
	}
}
