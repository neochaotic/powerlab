package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// catalog://index enumerates Apps/<id>/ subdirs that contain a
// docker-compose.yml; catalog://app/<id> round-trips that file.
func TestCatalog_HappyPath(t *testing.T) {
	dir := t.TempDir()
	mustCatalogApp(t, dir, "jellyfin", "services:\n  jellyfin:\n    image: jellyfin/jellyfin:10.10.5\n")
	mustCatalogApp(t, dir, "nginx", "services:\n  nginx:\n    image: nginx:1.27.3-alpine\n")
	// A subdirectory WITHOUT a docker-compose.yml must NOT be listed
	// — catalog hygiene: don't lead the agent to a 404.
	if err := os.MkdirAll(filepath.Join(dir, "Apps", "in-progress"), 0o700); err != nil {
		t.Fatalf("mkdir in-progress: %v", err)
	}

	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	registerCatalog(srv, dir)
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	// Index lists only the two apps with compose files, sorted.
	idx, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: catalogIndexURI})
	if err != nil {
		t.Fatalf("ReadResource(index): %v", err)
	}
	var manifest catalogManifest
	if uerr := json.Unmarshal([]byte(idx.Contents[0].Text), &manifest); uerr != nil {
		t.Fatalf("manifest not JSON: %v", uerr)
	}
	if len(manifest.Apps) != 2 {
		t.Fatalf("manifest has %d apps; want 2 (in-progress dir without compose must be skipped)", len(manifest.Apps))
	}
	if manifest.Apps[0].ID != "jellyfin" || manifest.Apps[1].ID != "nginx" {
		t.Errorf("manifest not sorted as expected: %+v", manifest.Apps)
	}

	// catalog://app/jellyfin round-trips the body.
	body, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: "catalog://app/jellyfin"})
	if err != nil {
		t.Fatalf("ReadResource(app): %v", err)
	}
	if !strings.Contains(body.Contents[0].Text, "jellyfin/jellyfin:10.10.5") {
		t.Errorf("body missing expected image line: %s", body.Contents[0].Text)
	}
}

func TestCatalog_MissingDirReturnsEmptyManifest(t *testing.T) {
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	registerCatalog(srv, "/nonexistent/catalog/path")
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	idx, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: catalogIndexURI})
	if err != nil {
		t.Fatalf("ReadResource(index): %v (want graceful empty)", err)
	}
	var manifest catalogManifest
	if uerr := json.Unmarshal([]byte(idx.Contents[0].Text), &manifest); uerr != nil {
		t.Fatalf("manifest not JSON: %v", uerr)
	}
	if len(manifest.Apps) != 0 {
		t.Errorf("manifest has %d apps; want 0 for missing dir", len(manifest.Apps))
	}
}

func TestCatalog_PathTraversalRejected(t *testing.T) {
	dir := t.TempDir()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	registerCatalog(srv, dir)
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	cases := []string{
		"catalog://app/../../../etc/passwd",
		"catalog://app/nested/path",
		"catalog://app/.hidden",
	}
	for _, uri := range cases {
		t.Run(uri, func(t *testing.T) {
			_, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: uri})
			if err == nil {
				t.Fatalf("path-traversal URI %q was accepted — must be rejected", uri)
			}
		})
	}
}

func mustCatalogApp(t *testing.T, catalogDir, id, composeBody string) {
	t.Helper()
	appDir := filepath.Join(catalogDir, "Apps", id)
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", appDir, err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "docker-compose.yml"), []byte(composeBody), 0o600); err != nil {
		t.Fatalf("write %s: %v", id, err)
	}
}
