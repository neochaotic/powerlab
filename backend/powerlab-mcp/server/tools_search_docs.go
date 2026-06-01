package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ADR-0048 — search_docs is a READ ONLY tool that does case-insensitive
// substring search across PowerLab's documentation surfaces. Returns
// up to top_k matches with {source, line_number, snippet, uri}. The
// agent can chain reads of the matching URIs for full context.
//
// Substring search (no regex, no fuzzy distance) is intentional: for
// PowerLab's order-of-100 indexed files, brute-force grep is sub-ms,
// has no parser edge cases, and the agent can iterate on its query.
//
// Surfaces indexed (P0.3, 2026-06-01 — extends original concepts-only
// coverage that the chat-mode test surfaced as a gap):
//   - concepts → docs://concepts/<stem>     (top-level .md files)
//   - openapi  → docs://api/<svc>           (top-level .yaml files)
//   - catalog  → catalog://app/<app-id>     (Apps/<id>/docker-compose.yml)

type searchDocsInput struct {
	Query string `json:"query" jsonschema:"required substring to look for in PowerLab docs (concepts, OpenAPI specs, catalog app definitions); case-insensitive; minimum 2 chars"`
	TopK  int    `json:"top_k,omitempty" jsonschema:"maximum number of hits to return; default 5; ceiling 20"`
}

type searchDocsHit struct {
	// Concept kept for backwards compat with operators / agents that
	// already key off this field; it carries the same value as Title
	// when the hit came from the concepts source, and is empty
	// otherwise. Prefer Title in new tooling.
	Concept    string `json:"concept,omitempty"`
	Source     string `json:"source"`         // "concepts" | "openapi" | "catalog"
	Title      string `json:"title"`          // file stem / app id / service name
	LineNumber int    `json:"line_number"`
	Snippet    string `json:"snippet"`
	URI        string `json:"uri"`
}

type searchDocsOutput struct {
	Query   string          `json:"query"`
	Matches []searchDocsHit `json:"matches"`
	Note    string          `json:"note,omitempty"`
}

const (
	searchDocsDefaultTopK = 5
	searchDocsMaxTopK     = 20
	searchDocsMinQuery    = 2
)

// searchRoot is a single indexable surface — a directory plus the
// recipe for turning each matched file into its canonical MCP
// resource URI (so the agent can chain to the full document).
type searchRoot struct {
	Source string                  // "concepts" | "openapi" | "catalog"
	Path   string                  // filesystem dir
	URIFn  func(stem string) string // map matched file → resource URI
}

func registerSearchDocs(s *mcp.Server, conceptsDir, openAPIDir, catalogDir string) {
	roots := []searchRoot{
		{Source: "concepts", Path: conceptsDir, URIFn: func(stem string) string { return docsConceptsPrefix + stem }},
		{Source: "openapi", Path: openAPIDir, URIFn: func(stem string) string { return "docs://api/" + stem }},
		{Source: "catalog", Path: catalogDir, URIFn: func(appID string) string { return "catalog://app/" + appID }},
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "search_docs",
		Description: "READ ONLY — case-insensitive substring search across PowerLab documentation: concepts (docs://concepts/*), OpenAPI specs (docs://api/*), and the bundled app catalog (catalog://app/*). Returns up to top_k hits with the canonical URI of each match so the agent can fetch full context. Minimum 2-character query.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in searchDocsInput) (*mcp.CallToolResult, searchDocsOutput, error) {
		out := searchDocsMulti(ctx, roots, in)
		return nil, out, nil
	})
}

// searchDocs preserves the original single-root signature for the
// existing test suite. New callers should use searchDocsMulti.
func searchDocs(ctx context.Context, conceptsDir string, in searchDocsInput) searchDocsOutput {
	roots := []searchRoot{
		{Source: "concepts", Path: conceptsDir, URIFn: func(stem string) string { return docsConceptsPrefix + stem }},
	}
	return searchDocsMulti(ctx, roots, in)
}

func searchDocsMulti(_ context.Context, roots []searchRoot, in searchDocsInput) searchDocsOutput {
	q := strings.TrimSpace(in.Query)
	if len(q) < searchDocsMinQuery {
		return searchDocsOutput{Query: in.Query, Matches: []searchDocsHit{}, Note: fmt.Sprintf("query must be at least %d characters", searchDocsMinQuery)}
	}
	topK := in.TopK
	if topK <= 0 {
		topK = searchDocsDefaultTopK
	}
	if topK > searchDocsMaxTopK {
		topK = searchDocsMaxTopK
	}

	hits := []searchDocsHit{}
	missingDirs := []string{}
	lowerQ := strings.ToLower(q)

	for _, root := range roots {
		more, missing := scanRoot(root, lowerQ, topK-len(hits))
		hits = append(hits, more...)
		if missing {
			missingDirs = append(missingDirs, root.Source)
		}
		if len(hits) >= topK {
			break
		}
	}

	out := searchDocsOutput{Query: q, Matches: hits}
	// Note when there were ZERO hits AND at least one surface was
	// missing — the agent gets a hint about why the search came up
	// empty (installer didn't stage the dir, dev box, etc.).
	if len(hits) == 0 && len(missingDirs) > 0 {
		out.Note = "no matches; missing doc surfaces on this host: " + strings.Join(missingDirs, ", ")
	}
	return out
}

// scanRoot walks one searchRoot. Catalog roots have a nested
// Apps/<id>/docker-compose.yml layout; other roots are flat dirs of
// .md / .yaml files. Returns (hits, missing-dir-flag).
func scanRoot(root searchRoot, lowerQ string, budget int) ([]searchDocsHit, bool) {
	if budget <= 0 {
		return nil, false
	}
	if root.Source == "catalog" {
		return scanCatalogRoot(root, lowerQ, budget)
	}
	return scanFlatRoot(root, lowerQ, budget)
}

// scanFlatRoot walks a flat directory of single-file documents
// (concepts = .md, openapi = .yaml). One file per indexed name.
func scanFlatRoot(root searchRoot, lowerQ string, budget int) ([]searchDocsHit, bool) {
	entries, err := os.ReadDir(root.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, true
		}
		return nil, false
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	hits := []searchDocsHit{}
	for _, e := range entries {
		name := e.Name()
		if !indexableFlatFile(name) {
			continue
		}
		stem := stemOf(name)
		path := filepath.Join(root.Path, name)
		more := scanFile(root, stem, path, lowerQ, budget-len(hits))
		hits = append(hits, more...)
		if len(hits) >= budget {
			return hits, false
		}
	}
	return hits, false
}

// scanCatalogRoot walks <CatalogDir>/Apps/*/docker-compose.yml — one
// docker-compose.yml per app, addressable as catalog://app/<id>.
func scanCatalogRoot(root searchRoot, lowerQ string, budget int) ([]searchDocsHit, bool) {
	appsDir := filepath.Join(root.Path, "Apps")
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, true
		}
		return nil, false
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	hits := []searchDocsHit{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		appID := e.Name()
		compose := filepath.Join(appsDir, appID, "docker-compose.yml")
		more := scanFile(root, appID, compose, lowerQ, budget-len(hits))
		hits = append(hits, more...)
		if len(hits) >= budget {
			return hits, false
		}
	}
	return hits, false
}

func indexableFlatFile(name string) bool {
	return strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")
}

func stemOf(name string) string {
	for _, ext := range []string{".md", ".yaml", ".yml"} {
		if strings.HasSuffix(name, ext) {
			return strings.TrimSuffix(name, ext)
		}
	}
	return name
}

func scanFile(root searchRoot, title, path, lowerQ string, budget int) []searchDocsHit {
	if budget <= 0 {
		return nil
	}
	// #nosec G304 -- path is composed from operator-configured root
	// dirs plus directory-listed entries; not user input.
	body, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return nil
	}
	hits := []searchDocsHit{}
	for lineNum, line := range strings.Split(string(body), "\n") {
		if !strings.Contains(strings.ToLower(line), lowerQ) {
			continue
		}
		hit := searchDocsHit{
			Source:     root.Source,
			Title:      title,
			LineNumber: lineNum + 1,
			Snippet:    strings.TrimSpace(line),
			URI:        root.URIFn(title),
		}
		if root.Source == "concepts" {
			hit.Concept = title // backwards compat
		}
		hits = append(hits, hit)
		if len(hits) >= budget {
			return hits
		}
	}
	return hits
}
