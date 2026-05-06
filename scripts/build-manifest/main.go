// build-manifest reads release-manifest.yaml from the repo root, fills
// in the dynamic fields the maintainer cannot know at edit time
// (version, released_at, per-arch tarball URL+SHA-256+size,
// changelog URL), and writes the final manifest.json to stdout.
//
// scripts/package-linux.sh invokes this once per release after both
// arch tarballs have been built — pipes the output into
// `dist/manifest.json` (uploaded as a release asset) and into the
// staged tarball at `<stage>/manifest.json` (so install.sh inside the
// tarball can read it locally).
//
// Usage:
//
//	go run ./scripts/build-manifest \
//	    -version 0.2.4 \
//	    -amd64-tarball dist/powerlab-0.2.4-linux-amd64.tar.gz \
//	    -arm64-tarball dist/powerlab-0.2.4-linux-arm64.tar.gz
//
// Exit codes: 0 on success, non-zero if the YAML cannot be read,
// the version cannot be parsed as semver, or a tarball is missing.
//
// The contract this command honours is documented in
// docs/UPDATE_MANIFEST.md.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// manifestSrc is the maintainer-edited fields. Mirrors release-manifest.yaml.
type manifestSrc struct {
	MinUpgradeFrom    string                 `yaml:"min_upgrade_from"    json:"min_upgrade_from"`
	SkipRelease       bool                   `yaml:"skip_release"        json:"skip_release"`
	Summary           string                 `yaml:"summary"             json:"summary"`
	BreakingChanges   []map[string]any       `yaml:"breaking_changes"    json:"breaking_changes"`
	PreInstallChecks  []map[string]any       `yaml:"pre_install_checks"  json:"pre_install_checks"`
	DBMigrations      []map[string]any       `yaml:"db_migrations"       json:"db_migrations"`
}

// manifestOut is the final shape written to manifest.json — the
// maintainer fields plus the build-time dynamic ones.
type manifestOut struct {
	Version          string                 `json:"version"`
	ReleasedAt       string                 `json:"released_at"`
	MinUpgradeFrom   string                 `json:"min_upgrade_from"`
	SkipRelease      bool                   `json:"skip_release"`
	Summary          string                 `json:"summary"`
	ChangelogURL     string                 `json:"changelog_url"`
	Tarball          map[string]tarballInfo `json:"tarball"`
	BreakingChanges  []map[string]any       `json:"breaking_changes"`
	PreInstallChecks []map[string]any       `json:"pre_install_checks"`
	DBMigrations     []map[string]any       `json:"db_migrations"`
}

type tarballInfo struct {
	URL       string `json:"url"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"size_bytes"`
}

func main() {
	var (
		yamlPath     = flag.String("source", "release-manifest.yaml", "path to the maintainer-edited YAML")
		version      = flag.String("version", "", "release version (semver, no leading v) — required")
		amd64Tarball = flag.String("amd64-tarball", "", "path to the linux/amd64 release tarball")
		arm64Tarball = flag.String("arm64-tarball", "", "path to the linux/arm64 release tarball")
		repo         = flag.String("repo", "neochaotic/powerlab", "GitHub owner/repo for tarball URLs")
	)
	flag.Parse()

	if *version == "" {
		die("--version is required")
	}
	if !looksLikeSemver(*version) {
		die("version %q is not semver (expected MAJOR.MINOR.PATCH)", *version)
	}

	src, err := readSource(*yamlPath)
	if err != nil {
		die("read source manifest: %v", err)
	}

	out := manifestOut{
		Version:          *version,
		ReleasedAt:       time.Now().UTC().Format(time.RFC3339),
		MinUpgradeFrom:   src.MinUpgradeFrom,
		SkipRelease:      src.SkipRelease,
		Summary:          strings.TrimSpace(src.Summary),
		ChangelogURL:     fmt.Sprintf("https://github.com/%s/blob/main/CHANGELOG.md#%s", *repo, anchorise(*version)),
		Tarball:          map[string]tarballInfo{},
		BreakingChanges:  defaultEmpty(src.BreakingChanges),
		PreInstallChecks: defaultEmpty(src.PreInstallChecks),
		DBMigrations:     defaultEmpty(src.DBMigrations),
	}

	if *amd64Tarball != "" {
		ti, err := tarballMeta(*amd64Tarball, *repo, *version, "amd64")
		if err != nil {
			die("amd64 tarball: %v", err)
		}
		out.Tarball["amd64"] = ti
	}
	if *arm64Tarball != "" {
		ti, err := tarballMeta(*arm64Tarball, *repo, *version, "arm64")
		if err != nil {
			die("arm64 tarball: %v", err)
		}
		out.Tarball["arm64"] = ti
	}
	if len(out.Tarball) == 0 {
		die("at least one of --amd64-tarball / --arm64-tarball must be provided")
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		die("encode manifest: %v", err)
	}
}

func readSource(path string) (*manifestSrc, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var src manifestSrc
	if err := yaml.Unmarshal(data, &src); err != nil {
		return nil, err
	}
	return &src, nil
}

// tarballMeta computes SHA-256 + size + the canonical download URL
// for a release tarball. The URL format mirrors how
// scripts/package-linux.sh names its outputs.
func tarballMeta(path, repo, version, arch string) (tarballInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return tarballInfo{}, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return tarballInfo{}, err
	}
	return tarballInfo{
		URL: fmt.Sprintf(
			"https://github.com/%s/releases/download/v%s/powerlab-%s-linux-%s.tar.gz",
			repo, version, version, arch,
		),
		SHA256:    hex.EncodeToString(h.Sum(nil)),
		SizeBytes: n,
	}, nil
}

// looksLikeSemver does a structural check, not a full RFC parse. We
// reject inputs the rest of the pipeline would silently mishandle
// (empty, leading 'v', three dots, etc.) and accept everything else.
func looksLikeSemver(v string) bool {
	if v == "" || strings.HasPrefix(v, "v") {
		return false
	}
	parts := strings.SplitN(v, "-", 2)
	core := strings.Split(parts[0], ".")
	if len(core) != 3 {
		return false
	}
	for _, p := range core {
		if p == "" {
			return false
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

// anchorise turns "0.2.4" into "024" — the GFM heading anchor format
// for "## [0.2.4]" sections in CHANGELOG.md.
func anchorise(v string) string {
	return strings.ReplaceAll(v, ".", "")
}

// defaultEmpty replaces nil with [] so the JSON output never has
// `null` where the contract promises an array.
func defaultEmpty(s []map[string]any) []map[string]any {
	if s == nil {
		return []map[string]any{}
	}
	return s
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "build-manifest: "+format+"\n", args...)
	os.Exit(1)
}
