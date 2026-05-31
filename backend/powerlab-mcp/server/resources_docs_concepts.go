package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ADR-0048 — docs://concepts/{name} exposes PowerLab's mkdocs concept
// files (compose-conventions.md, security-model.md, glossary.md, etc.)
// as MCP resources. Same discovery pattern as docs://api: an index
// resource lists what's available, a per-file template returns the
// raw markdown. The Prompt primitive compose_authoring (see
// prompts_compose_authoring.go) consumes this directory directly to
// build its bundle — the resource family is the agent-driven path,
// the prompt is the curated bundle path.

const (
	docsConceptsIndexURI = "docs://concepts/index"
	docsConceptsPrefix   = "docs://concepts/"
	docsConceptsTemplate = "docs://concepts/{name}"
)

// conceptsManifest is the wire shape of docs://concepts/index.
type conceptsManifest struct {
	Description string                  `json:"description"`
	Concepts    []conceptsManifestEntry `json:"concepts"`
}

type conceptsManifestEntry struct {
	Name      string `json:"name"`       // file stem without .md
	URI       string `json:"uri"`        // docs://concepts/<name>
	SizeBytes int64  `json:"size_bytes"`
}

func registerDocsConcepts(s *mcp.Server, conceptsDir string) {
	s.AddResource(
		&mcp.Resource{
			URI:         docsConceptsIndexURI,
			Name:        "PowerLab concepts manifest",
			Description: "Index of PowerLab concept documents — read this first to discover what's available, then read docs://concepts/<name> for the raw markdown. Covers compose conventions, security model, glossary, MCP architecture, and other operator-facing background.",
			MIMEType:    "application/json",
		},
		func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			b, err := buildConceptsManifest(conceptsDir)
			if err != nil {
				return nil, fmt.Errorf("build concepts manifest: %w", err)
			}
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{textJSON(docsConceptsIndexURI, string(b))}}, nil
		},
	)

	s.AddResourceTemplate(
		&mcp.ResourceTemplate{
			URITemplate: docsConceptsTemplate,
			Name:        "PowerLab concept document",
			Description: "Raw markdown for one PowerLab concept. Use docs://concepts/index to discover the available concepts first. Notable: docs://concepts/compose-conventions is the canonical 'how PowerLab thinks about docker-compose' document — the compose_authoring MCP prompt bundles it for the same surface.",
			MIMEType:    "text/markdown",
		},
		func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			name := conceptNameFromURI(req.Params.URI)
			if name == "" {
				return nil, fmt.Errorf("docs://concepts/{name} requires a concept name (see docs://concepts/index for the list)")
			}
			body, err := readConcept(conceptsDir, name)
			if err != nil {
				return nil, err
			}
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: req.Params.URI, MIMEType: "text/markdown", Text: body}}}, nil
		},
	)
}

// buildConceptsManifest enumerates *.md files in conceptsDir. Missing
// directory → empty manifest (graceful — a fresh box without the
// docs/ tree shouldn't error; agent sees an empty index and pivots).
func buildConceptsManifest(conceptsDir string) ([]byte, error) {
	manifest := conceptsManifest{
		Description: "Available PowerLab concept documents. Read docs://concepts/<name> for the raw markdown of any entry below. Start with 'compose-conventions' for the PowerLab docker-compose patterns the validator + install_app enforce.",
		Concepts:    []conceptsManifestEntry{},
	}

	entries, err := os.ReadDir(conceptsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return json.Marshal(manifest)
		}
		return nil, fmt.Errorf("read %s: %w", conceptsDir, err)
	}

	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		stem := strings.TrimSuffix(name, ".md")
		info, statErr := e.Info()
		var size int64
		if statErr == nil {
			size = info.Size()
		}
		manifest.Concepts = append(manifest.Concepts, conceptsManifestEntry{
			Name:      stem,
			URI:       docsConceptsPrefix + stem,
			SizeBytes: size,
		})
	}
	sort.Slice(manifest.Concepts, func(i, j int) bool { return manifest.Concepts[i].Name < manifest.Concepts[j].Name })
	return json.Marshal(manifest)
}

// readConcept reads one concept file. Path-traversal hardened: the
// name segment is rejected if it carries any path separators or dots
// — same defensive pattern as readSpec (docs://api/{service}).
func readConcept(conceptsDir, name string) (string, error) {
	if strings.ContainsAny(name, "/\\.") {
		return "", fmt.Errorf("invalid concept name %q (must match docs://concepts/index entries; no path separators)", name)
	}
	path := filepath.Join(conceptsDir, name+".md")
	// #nosec G304 -- conceptsDir is operator-configured + name is
	// validated above (no path separators, no dots).
	body, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("no concept named %q at %s — read docs://concepts/index for the available concepts", name, path)
		}
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return string(body), nil
}

// conceptNameFromURI extracts "<name>" from "docs://concepts/<name>"
// (and rejects the index URI itself, which has its own dedicated
// handler). Symmetric with serviceFromDocsURI in resources_docs.go.
func conceptNameFromURI(raw string) string {
	if !strings.HasPrefix(raw, docsConceptsPrefix) {
		return ""
	}
	tail := strings.TrimPrefix(raw, docsConceptsPrefix)
	if tail == "" || tail == "index" {
		return ""
	}
	return tail
}
