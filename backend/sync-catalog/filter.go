package main

import (
	"fmt"
	"regexp"
	"strings"
)

// Filter applies the four-tier verdict from ADR-0024 to a single app.
// Construction is cheap; reuse across many Apply() calls in one sync run.
type Filter struct {
	// KnownAppIDs is the set of catalog directory names (lowercase
	// id form) used to disambiguate "same-compose sibling" (allow)
	// from "cross-app sibling" (Tier 1 reject). Real syncs feed
	// this from the upstream repo's directory listing; tests inject
	// a static map.
	KnownAppIDs map[string]bool

	// AllowedCategories are operator-opted-in categories that would
	// otherwise be Tier 2 soft-rejected. Empty = default policy
	// (Bitcoin / Lightning / Bitcoin Node are blocked).
	AllowedCategories []string
}

// defaultSoftRejectCategories matches the ADR-0024 list. Mutated only
// by removing entries via Filter.AllowedCategories (operator opt-in),
// never by adding.
var defaultSoftRejectCategories = []string{
	"Bitcoin", "Lightning", "Bitcoin Node",
}

// getumbrelImage detects images authored / republished by the Umbrel
// team. Matches both `getumbrel/x` and `ghcr.io/getumbrel/x`. Tags
// and SHA digests are ignored — only the namespace matters for the
// rule.
var getumbrelImage = regexp.MustCompile(`(?i)(^|/)getumbrel/`)

// crossAppSiblingEnvVar matches `${APP_<OTHER>_*}` patterns. The
// regex captures <OTHER> so we can look it up in KnownAppIDs. Note
// that Umbrel uses underscores in env names but hyphens in app IDs
// (e.g. APP_BITCOIN_NODE_IP → app id "bitcoin"), so the match
// captures the first segment.
var crossAppSiblingEnvVar = regexp.MustCompile(`\$\{APP_([A-Z]+)(_[A-Z_]+)?\}`)

// Apply returns the filter verdict for a single (manifest, compose) pair.
//
// Order of checks (first match wins):
//   1. Tier 1: getumbrel/* image
//   2. Tier 1: cross-app sibling env var (${APP_<OTHER>_*})
//   3. Tier 2: category-based soft reject (overridable via AllowedCategories)
//   4. Default: Tier 4 allow
func (f *Filter) Apply(appID string, manifest UmbrelManifest, compose ComposeFile) FilterResult {
	// Rule 1.1 — getumbrel/* image (covers docker.io and ghcr.io).
	// Inspect every service's image; any one match is enough.
	for svcName, svc := range compose.Services {
		if getumbrelImage.MatchString(svc.Image) {
			return FilterResult{
				Tier: TierHardReject,
				Reason: fmt.Sprintf(
					"service %q uses image %q (getumbrel/* publisher — Umbrel-only, cannot run on PowerLab without sibling apps)",
					svcName, svc.Image),
			}
		}
	}

	// Rule 1.2 — cross-app sibling env var.
	// Scan all services' environments for ${APP_<OTHER>_*} references
	// where OTHER (lowercased) is a known sibling app ID.
	for svcName, svc := range compose.Services {
		envText := flattenEnvironment(svc.EnvironmentRaw)
		matches := crossAppSiblingEnvVar.FindAllStringSubmatch(envText, -1)
		for _, m := range matches {
			otherID := strings.ToLower(m[1])
			// Don't flag self-references (e.g. ${APP_BITCOIN_*} inside the bitcoin app's own compose).
			if otherID == strings.ToLower(appID) {
				continue
			}
			if f.KnownAppIDs[otherID] {
				return FilterResult{
					Tier: TierHardReject,
					Reason: fmt.Sprintf(
						"service %q references cross-app sibling env var ${APP_%s_*} (app %q exists in the catalog — depend chain breaks on PowerLab)",
						svcName, m[1], otherID),
				}
			}
		}
	}

	// Rule 2 — category-based soft reject.
	for _, deny := range defaultSoftRejectCategories {
		if !strings.EqualFold(manifest.Category, deny) {
			continue
		}
		if f.categoryAllowed(deny) {
			break // operator opted back in
		}
		return FilterResult{
			Tier: TierSoftReject,
			Reason: fmt.Sprintf(
				"category %q is on the default-deny list (opt back in via community_catalog.allow_categories)",
				manifest.Category),
		}
	}

	return FilterResult{Tier: TierAllow, Reason: "no filter trigger fired"}
}

// flattenEnvironment normalises the polymorphic compose `environment:`
// shape (list-of-"KEY=VAL" OR map-of-KEY-to-VAL) into a single string
// the env-var regex can scan. We don't care about exact key/value
// boundaries — just whether the ${APP_<OTHER>_*} pattern appears
// anywhere in the env block.
func flattenEnvironment(raw any) string {
	if raw == nil {
		return ""
	}
	var sb strings.Builder
	switch v := raw.(type) {
	case []string:
		for _, s := range v {
			sb.WriteString(s)
			sb.WriteByte('\n')
		}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				sb.WriteString(s)
				sb.WriteByte('\n')
			}
		}
	case map[string]any:
		for k, val := range v {
			sb.WriteString(k)
			sb.WriteByte('=')
			if s, ok := val.(string); ok {
				sb.WriteString(s)
			} else {
				fmt.Fprintf(&sb, "%v", val)
			}
			sb.WriteByte('\n')
		}
	case map[any]any:
		// yaml.v3 sometimes produces this shape pre-marshal
		for k, val := range v {
			fmt.Fprintf(&sb, "%v=%v\n", k, val)
		}
	}
	return sb.String()
}

func (f *Filter) categoryAllowed(cat string) bool {
	for _, allowed := range f.AllowedCategories {
		if strings.EqualFold(allowed, cat) {
			return true
		}
	}
	return false
}
