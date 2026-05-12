// Package sync-catalog imports the public Umbrel App Store catalog
// into PowerLab's community-catalog/, applying the four-tier filter
// pipeline specified in ADR-0024 (`docs/decisions/0024-umbrel-catalog-
// clean-room-import.md`).
//
// Field allowlist: this package extracts only functional/factual fields
// from the upstream YAMLs (id, image, ports, env keys, volumes, deps)
// and ignores expressive content (descriptions, screenshots, marketing
// copy). The legal posture is captured in the ADR; this code enforces
// it through the parser's explicit allowlist and the filter's tier
// rules.
package main

// UmbrelManifest mirrors the fields we extract from `umbrel-app.yml`.
// Anything not in this struct is ignored on purpose — adding a new
// field is a deliberate code change that goes through review.
type UmbrelManifest struct {
	ManifestVersion int    `yaml:"manifestVersion"`
	ID              string `yaml:"id"`
	Name            string `yaml:"name"`
	Tagline         string `yaml:"tagline"`
	Category        string `yaml:"category"`
	Version         string `yaml:"version"`
	Port            int    `yaml:"port"`

	// Pointer for "deferred fetch + maybe-override" — see description.go.
	// We deliberately do NOT carry the upstream `description` (Umbrel-
	// curated marketing copy); only the `repo:` field is kept so we can
	// fetch the app's OWN README at sync time.
	Developer string `yaml:"developer"`
	Repo      string `yaml:"repo"`
	Support   string `yaml:"support"`
	Website   string `yaml:"website"`
}

// ComposeFile is the minimal docker-compose shape the filter needs to
// inspect. The full compose grammar is rich; we only model what the
// filter rules touch.
type ComposeFile struct {
	Services map[string]Service `yaml:"services"`
	Networks map[string]any     `yaml:"networks"`
}

// Service is one entry under `services:` in docker-compose.yml.
// Environment can be either a map or a list in the wild — we accept
// both shapes via `EnvironmentRaw any` and normalise in Lookups().
type Service struct {
	Image          string   `yaml:"image"`
	DependsOn      any      `yaml:"depends_on"` // list-of-strings OR map-of-{condition}
	Networks       any      `yaml:"networks"`   // list-of-strings OR map
	Volumes        []string `yaml:"volumes"`
	EnvironmentRaw any      `yaml:"environment"` // list-of-"KEY=VAL" OR map
}

// FilterTier ranks the verdict from "import freely" to "do not import".
type FilterTier int

const (
	TierAllow         FilterTier = iota // OSS image, no sibling deps, recognisable license — import
	TierManualTriage                    // Optional sibling vars, unknown license — into _pending/
	TierSoftReject                      // Crypto / category-driven; opt-in via config
	TierHardReject                      // getumbrel/* image, cross-app sibling deps, umbrel-only network — never import
)

func (t FilterTier) String() string {
	switch t {
	case TierAllow:
		return "allow"
	case TierManualTriage:
		return "manual_triage"
	case TierSoftReject:
		return "soft_reject"
	case TierHardReject:
		return "hard_reject"
	}
	return "unknown"
}

// FilterResult is what Filter.Apply returns. Reason carries the
// human-readable trigger (which rule fired) so the sync log can
// surface it on the PR description.
type FilterResult struct {
	Tier   FilterTier
	Reason string
}
