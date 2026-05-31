package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// docs://concepts/index lists every *.md in the configured directory
// and the per-concept template round-trips the raw markdown. Happy
// path locks the contract; missing dir + path traversal land in
// dedicated subtests.
func TestDocsConcepts_HappyPath(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, dir, "compose-conventions.md", "# Compose conventions\n\nUse /DATA paths.\n")
	mustWrite(t, dir, "security-model.md", "# Security\n\nNo docker.sock.\n")

	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	registerDocsConcepts(srv, dir)
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	// Index lists both files.
	idx, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: docsConceptsIndexURI})
	if err != nil {
		t.Fatalf("ReadResource(index): %v", err)
	}
	var manifest conceptsManifest
	if uerr := json.Unmarshal([]byte(idx.Contents[0].Text), &manifest); uerr != nil {
		t.Fatalf("manifest not JSON: %v", uerr)
	}
	if len(manifest.Concepts) != 2 {
		t.Fatalf("manifest has %d entries; want 2", len(manifest.Concepts))
	}
	if manifest.Concepts[0].Name != "compose-conventions" || manifest.Concepts[1].Name != "security-model" {
		t.Errorf("manifest entries not sorted: %+v", manifest.Concepts)
	}

	// docs://concepts/compose-conventions round-trips the file body.
	body, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: "docs://concepts/compose-conventions"})
	if err != nil {
		t.Fatalf("ReadResource(concept): %v", err)
	}
	if !strings.Contains(body.Contents[0].Text, "Use /DATA paths") {
		t.Errorf("body missing expected text: %s", body.Contents[0].Text)
	}
}

// Missing concepts directory → empty manifest, NOT error. Mac dev box
// without the install staged is the canonical scenario.
func TestDocsConcepts_MissingDirReturnsEmptyManifest(t *testing.T) {
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	registerDocsConcepts(srv, "/nonexistent/path/that/does/not/exist")
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	idx, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: docsConceptsIndexURI})
	if err != nil {
		t.Fatalf("ReadResource(index): %v (want graceful empty manifest)", err)
	}
	var manifest conceptsManifest
	if uerr := json.Unmarshal([]byte(idx.Contents[0].Text), &manifest); uerr != nil {
		t.Fatalf("manifest not JSON: %v", uerr)
	}
	if len(manifest.Concepts) != 0 {
		t.Errorf("manifest has %d entries; want 0 for missing dir", len(manifest.Concepts))
	}
	if manifest.Description == "" {
		t.Errorf("manifest description empty — agent still needs a hint about what this surface is")
	}
}

// Path traversal must be rejected — the concept name segment cannot
// contain '/', '\\', or '.' (no separators, no parent traversal).
// Adversarial input lands at the MCP read handler, not at the
// filesystem.
func TestDocsConcepts_PathTraversalRejected(t *testing.T) {
	dir := t.TempDir()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	registerDocsConcepts(srv, dir)
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	cases := []string{
		"docs://concepts/../../../etc/passwd",
		"docs://concepts/sub/dir",
		"docs://concepts/.hidden",
	}
	for _, uri := range cases {
		t.Run(uri, func(t *testing.T) {
			_, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: uri})
			if err == nil {
				t.Fatalf("path-traversal URI %q was accepted — must be rejected at read", uri)
			}
		})
	}
}

