// Package common — labels.go is the single source of truth for every
// container label PowerLab writes or reads. Per ADR-0021, PowerLab is
// migrating from unnamespaced legacy keys (the sentinel `casaos =
// "casaos"` plus naked `origin`, `icon`, `name` etc.) to canonical
// `io.powerlab.v1.*` namespaced keys. During the dual-write window
// every new container gets BOTH naming sets so existing PowerLab
// installs (whose containers still carry only legacy keys) keep
// being recognized by the "is mine" filter without forcing a
// container recreate.
//
// All constants and helpers in this file are pure data + pure
// functions — no Docker client calls, no filesystem access. Wiring
// them into the actual create/list code paths happens in a separate
// PR (the second sub-PR of #85).
package common

// Canonical container labels — `io.powerlab.v1.*` reverse-DNS
// namespaced. These are what new code reads/writes; the legacy
// keys below are accepted on read (one release window) but will
// stop being written after the dual-write window closes.
const (
	// LabelKindKey + LabelKindValueApp form the "is mine" sentinel.
	// Any container with `io.powerlab.v1.kind = "app"` is considered
	// a PowerLab-managed app container.
	LabelKindKey      = "io.powerlab.v1.kind"
	LabelKindValueApp = "app"

	LabelOriginKey      = "io.powerlab.v1.origin"
	LabelWebPortKey     = "io.powerlab.v1.web-port"
	LabelIconKey        = "io.powerlab.v1.icon"
	LabelDescriptionKey = "io.powerlab.v1.description"
	LabelWebIndexKey    = "io.powerlab.v1.web-index"
	LabelCustomIDKey    = "io.powerlab.v1.custom-id"
	LabelShowEnvKey     = "io.powerlab.v1.show-env"
	LabelProtocolKey    = "io.powerlab.v1.protocol"
	LabelHostKey        = "io.powerlab.v1.host"
	LabelNameKey        = "io.powerlab.v1.name"
	LabelAppStoreIDKey  = "io.powerlab.v1.app.store.id"
)

// Legacy unnamespaced container labels — read for backward compat
// with containers PowerLab itself created before ADR-0021 landed.
// New code MUST NOT add to this list. After the dual-write window
// closes (the next PR after the wiring PR), the dual-WRITE drops
// but the dual-READ stays for at least one further release window.
const (
	LegacyLabelKindKey      = "casaos"
	LegacyLabelKindValueApp = "casaos"

	LegacyLabelOriginKey      = "origin"
	LegacyLabelWebPortKey     = "web"
	LegacyLabelIconKey        = "icon"
	LegacyLabelDescriptionKey = "desc"
	LegacyLabelWebIndexKey    = "index"
	LegacyLabelCustomIDKey    = "custom_id"
	LegacyLabelShowEnvKey     = "show_env"
	LegacyLabelProtocolKey    = "protocol"
	LegacyLabelHostKey        = "host"
	LegacyLabelNameKey        = "name"
	// LegacyLabelAppStoreID is the existing namespaced key — kept
	// alongside its replacement during the dual-write window to
	// match the rest of the legacy set.
	LegacyLabelAppStoreIDKey = "io.casaos.v1.app.store.id"
)

// IsPowerLabApp returns true if the supplied container labels
// indicate a container PowerLab manages. Accepts EITHER the
// canonical sentinel (io.powerlab.v1.kind = "app") OR the legacy
// sentinel (casaos = "casaos").
//
// Use this in every container-list filter that previously checked
// `Labels["casaos"] == "casaos"` directly. Direct map reads are an
// anti-pattern after this PR — they bypass the dual-read contract.
//
// nil input is safe; returns false.
func IsPowerLabApp(labels map[string]string) bool {
	if labels == nil {
		return false
	}
	if labels[LabelKindKey] == LabelKindValueApp {
		return true
	}
	if labels[LegacyLabelKindKey] == LegacyLabelKindValueApp {
		return true
	}
	return false
}

// LabelValue returns the value of a PowerLab container label, falling
// back to the legacy unnamespaced key if the canonical one isn't set.
// Returns "" if neither is present.
//
// Pass the canonical key constant (e.g. LabelIconKey); the legacy
// fallback is resolved internally via legacyKeyFor.
//
// nil input is safe; returns "".
func LabelValue(labels map[string]string, canonicalKey string) string {
	if labels == nil {
		return ""
	}
	if v, ok := labels[canonicalKey]; ok && v != "" {
		return v
	}
	if legacy := legacyKeyFor(canonicalKey); legacy != "" {
		return labels[legacy]
	}
	return ""
}

// legacyKeyFor maps a canonical label key to its legacy alias, or
// "" if no legacy alias exists. Defined as a switch so go vet can
// catch new constants added to one set but not the other.
func legacyKeyFor(canonicalKey string) string {
	switch canonicalKey {
	case LabelKindKey:
		return LegacyLabelKindKey
	case LabelOriginKey:
		return LegacyLabelOriginKey
	case LabelWebPortKey:
		return LegacyLabelWebPortKey
	case LabelIconKey:
		return LegacyLabelIconKey
	case LabelDescriptionKey:
		return LegacyLabelDescriptionKey
	case LabelWebIndexKey:
		return LegacyLabelWebIndexKey
	case LabelCustomIDKey:
		return LegacyLabelCustomIDKey
	case LabelShowEnvKey:
		return LegacyLabelShowEnvKey
	case LabelProtocolKey:
		return LegacyLabelProtocolKey
	case LabelHostKey:
		return LegacyLabelHostKey
	case LabelNameKey:
		return LegacyLabelNameKey
	case LabelAppStoreIDKey:
		return LegacyLabelAppStoreIDKey
	}
	return ""
}

// AppLabels is the subset of label values that PowerLab's compose
// generator + V1 Custom App builder produce when authoring a new
// container. Pass to BuildLabels to get a map ready to merge into
// the container Config.Labels.
type AppLabels struct {
	Origin      string
	WebPort     string
	Icon        string
	Description string
	WebIndex    string
	CustomID    string
	ShowEnv     string
	Protocol    string
	Host        string
	Name        string
	AppStoreID  string
}

// BuildLabels returns a map containing BOTH the canonical
// io.powerlab.v1.* labels AND the legacy unnamespaced labels for
// the supplied AppLabels values. Empty values are omitted from BOTH
// sides so a container's label set never carries empty-string keys.
//
// The kind sentinel (LabelKindKey + LegacyLabelKindKey) is always
// included regardless of AppLabels content — it identifies the
// container as PowerLab-managed.
//
// Per ADR-0021, this dual-write lasts for ONE release window; after
// that, a follow-up PR removes the Legacy* writes from this function
// while keeping the canonical writes (and IsPowerLabApp/LabelValue
// keep their dual-read for one further window).
func BuildLabels(a AppLabels) map[string]string {
	out := map[string]string{
		LabelKindKey:       LabelKindValueApp,
		LegacyLabelKindKey: LegacyLabelKindValueApp,
	}
	type pair struct{ canonical, legacy, value string }
	pairs := []pair{
		{LabelOriginKey, LegacyLabelOriginKey, a.Origin},
		{LabelWebPortKey, LegacyLabelWebPortKey, a.WebPort},
		{LabelIconKey, LegacyLabelIconKey, a.Icon},
		{LabelDescriptionKey, LegacyLabelDescriptionKey, a.Description},
		{LabelWebIndexKey, LegacyLabelWebIndexKey, a.WebIndex},
		{LabelCustomIDKey, LegacyLabelCustomIDKey, a.CustomID},
		{LabelShowEnvKey, LegacyLabelShowEnvKey, a.ShowEnv},
		{LabelProtocolKey, LegacyLabelProtocolKey, a.Protocol},
		{LabelHostKey, LegacyLabelHostKey, a.Host},
		{LabelNameKey, LegacyLabelNameKey, a.Name},
		{LabelAppStoreIDKey, LegacyLabelAppStoreIDKey, a.AppStoreID},
	}
	for _, p := range pairs {
		if p.value == "" {
			continue
		}
		out[p.canonical] = p.value
		out[p.legacy] = p.value
	}
	return out
}
