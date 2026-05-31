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
// substring search across the concepts directory. Returns up to top_k
// matches with {file, line_number, snippet}. The agent can chain reads
// of the matching docs://concepts/<name> for full context.
//
// Substring search (no regex, no fuzzy distance) is intentional: for
// the ~10-20 concept files PowerLab ships, brute-force grep is sub-ms,
// has no parser edge cases, and the agent can iterate on its query.

type searchDocsInput struct {
	Query string `json:"query" jsonschema:"required substring to look for in PowerLab concept docs; case-insensitive; minimum 2 chars"`
	TopK  int    `json:"top_k,omitempty" jsonschema:"maximum number of hits to return; default 5; ceiling 20"`
}

type searchDocsHit struct {
	Concept    string `json:"concept"`              // file stem without .md
	LineNumber int    `json:"line_number"`
	Snippet    string `json:"snippet"`              // the matching line trimmed
	URI        string `json:"uri"`                  // docs://concepts/<concept>
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

func registerSearchDocs(s *mcp.Server, conceptsDir string) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "search_docs",
		Description: "READ ONLY — substring search across the PowerLab concept documents (docs/concepts/*.md). Returns up to top_k matches with file + line number + snippet. Use the returned URIs (docs://concepts/<concept>) to fetch full context. Case-insensitive; minimum 2-character query.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in searchDocsInput) (*mcp.CallToolResult, searchDocsOutput, error) {
		out := searchDocs(ctx, conceptsDir, in)
		return nil, out, nil
	})
}

func searchDocs(_ context.Context, conceptsDir string, in searchDocsInput) searchDocsOutput {
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

	entries, err := os.ReadDir(conceptsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return searchDocsOutput{Query: q, Matches: hits, Note: fmt.Sprintf("concepts directory %s does not exist on this host", conceptsDir)}
		}
		return searchDocsOutput{Query: q, Matches: hits, Note: fmt.Sprintf("read concepts dir: %v", err)}
	}

	lowerQ := strings.ToLower(q)
	// Sort entries for deterministic hit order across runs.
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		stem := strings.TrimSuffix(name, ".md")
		path := filepath.Join(conceptsDir, name)
		// #nosec G304 -- conceptsDir is operator-configured; entries
		// come from a directory listing of that dir.
		body, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			// soft-skip other read errors so one bad file doesn't
			// kill the whole search
			continue
		}
		for lineNum, line := range strings.Split(string(body), "\n") {
			if strings.Contains(strings.ToLower(line), lowerQ) {
				hits = append(hits, searchDocsHit{
					Concept:    stem,
					LineNumber: lineNum + 1, // 1-indexed for human readability
					Snippet:    strings.TrimSpace(line),
					URI:        docsConceptsPrefix + stem,
				})
				if len(hits) >= topK {
					return searchDocsOutput{Query: q, Matches: hits}
				}
			}
		}
	}

	return searchDocsOutput{Query: q, Matches: hits}
}
