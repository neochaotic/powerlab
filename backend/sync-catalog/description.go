package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// DescriptionResolver fetches per-app descriptions in a legally-safe
// way per ADR-0024.
//
// Resolution order:
//   1. If `<CatalogRoot>/apps/<id>/description-powerlab.md` exists,
//      use it verbatim. This is the maintainer override surface.
//   2. Otherwise fetch the README from the app's OWN upstream repo
//      (`manifest.Repo`), strip the markdown to plain text, and
//      truncate to MaxWords words.
//
// The Umbrel-curated `description:` field from umbrel-app.yml is
// NEVER consulted. The legal posture (facts vs expressive content)
// only allows the app maintainer's OWN README, which carries the
// app's own OSS license (MIT/Apache/GPL — all of which permit
// display + truncation).
type DescriptionResolver struct {
	// HTTPClient is injectable so tests can point at httptest servers.
	// Default is a 10-second timeout client.
	HTTPClient *http.Client

	// CatalogRoot is the directory where override files live.
	// Override path is <CatalogRoot>/apps/<id>/description-powerlab.md.
	CatalogRoot string

	// MaxWords caps the auto-fetched README. Default 200.
	MaxWords int
}

// DefaultMaxWords mirrors the audit doc's recommendation.
const DefaultMaxWords = 200

// NewDescriptionResolver returns a resolver with sensible defaults.
func NewDescriptionResolver(catalogRoot string) *DescriptionResolver {
	return &DescriptionResolver{
		HTTPClient:  &http.Client{Timeout: 10 * time.Second},
		CatalogRoot: catalogRoot,
		MaxWords:    DefaultMaxWords,
	}
}

// Resolve returns the description for one app, applying the
// override-then-upstream precedence. Errors from the upstream
// fetch are surfaced; the caller decides whether to soft-skip
// the app or to write a placeholder.
func (r *DescriptionResolver) Resolve(ctx context.Context, manifest UmbrelManifest) (string, error) {
	if override, ok := r.loadOverride(manifest.ID); ok {
		return override, nil
	}
	return r.fetchUpstreamReadme(ctx, manifest)
}

// loadOverride reads <CatalogRoot>/apps/<id>/description-powerlab.md.
// Returns (content, true) on success; ("", false) if the file does
// not exist OR is empty.
func (r *DescriptionResolver) loadOverride(appID string) (string, bool) {
	if r.CatalogRoot == "" {
		return "", false
	}
	path := filepath.Join(r.CatalogRoot, "apps", appID, "description-powerlab.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return "", false
	}
	return content, true
}

// fetchUpstreamReadme pulls the README from the app's repo,
// strips markdown to plain text, and truncates.
func (r *DescriptionResolver) fetchUpstreamReadme(ctx context.Context, manifest UmbrelManifest) (string, error) {
	if manifest.Repo == "" {
		return "", fmt.Errorf("manifest %q has no `repo:` field — no upstream to fetch from", manifest.ID)
	}
	url, err := readmeURL(manifest.Repo)
	if err != nil {
		return "", fmt.Errorf("derive readme URL for %q: %w", manifest.ID, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := r.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch %s: HTTP %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024)) // 5 MB cap
	if err != nil {
		return "", err
	}

	stripped := stripMarkdown(string(body))
	maxWords := r.MaxWords
	if maxWords <= 0 {
		maxWords = DefaultMaxWords
	}
	return truncateWords(stripped, maxWords), nil
}

// readmeURL maps a GitHub repo URL to its raw README candidate URL.
// Tries `main` first; the fetcher can be extended to retry `master`
// if main 404s, but main covers 90 %+ of modern repos.
//
// Input examples that must be handled:
//   - https://github.com/NginxProxyManager/nginx-proxy-manager
//   - https://github.com/NginxProxyManager/nginx-proxy-manager.git
//   - https://github.com/foo/bar/
func readmeURL(repo string) (string, error) {
	u, err := url.Parse(repo)
	if err != nil {
		return "", err
	}
	if u.Host != "github.com" {
		return "", fmt.Errorf("non-github upstream %q not yet supported", u.Host)
	}
	path := strings.TrimSuffix(strings.TrimSuffix(u.Path, "/"), ".git")
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("repo path %q not in owner/repo form", path)
	}
	owner, repoName := parts[0], parts[1]
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/README.md", owner, repoName), nil
}

// markdownStrippers in order of application. Each rune intentionally:
//  - badges (![Build]...) clutter the description top
//  - code blocks rarely belong in a tile description
//  - headers we keep as text but drop the # prefix
//  - links we keep the text, drop the URL
var (
	mdBadge      = regexp.MustCompile(`\!\[[^\]]*\]\([^\)]+\)`)
	mdCodeFence  = regexp.MustCompile(`(?s)\x60{3}.*?\x60{3}`)
	mdInlineCode = regexp.MustCompile(`\x60[^\x60]+\x60`)
	mdLink       = regexp.MustCompile(`\[([^\]]+)\]\([^\)]+\)`)
	mdHeader     = regexp.MustCompile(`(?m)^#{1,6}\s+`)
	mdHRule      = regexp.MustCompile(`(?m)^[-=*_]{3,}\s*$`)
	mdMultiSpace = regexp.MustCompile(`\s+`)
)

// stripMarkdown reduces a README to a plain-text paragraph stream.
// Not a real markdown parser — just enough to make a 200-word excerpt
// readable in a catalog tile.
func stripMarkdown(s string) string {
	s = mdCodeFence.ReplaceAllString(s, " ")
	s = mdBadge.ReplaceAllString(s, "")
	s = mdInlineCode.ReplaceAllString(s, "")
	s = mdLink.ReplaceAllString(s, "$1") // keep link text, drop URL
	s = mdHeader.ReplaceAllString(s, "")
	s = mdHRule.ReplaceAllString(s, "")
	s = mdMultiSpace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// truncateWords cuts after n whitespace-separated tokens, preserving
// word boundaries. Appends an ellipsis on truncation.
func truncateWords(s string, n int) string {
	if n <= 0 {
		return ""
	}
	fields := strings.Fields(s)
	if len(fields) <= n {
		return strings.Join(fields, " ")
	}
	return strings.Join(fields[:n], " ") + "…"
}
