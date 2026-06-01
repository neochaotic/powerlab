package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSearchDocs_FindsSubstringAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, dir, "compose-conventions.md", "Use /DATA/PowerLabAppData paths.\nNever bind /var/run/docker.sock.\n")
	mustWrite(t, dir, "security-model.md", "The validator rejects privileged: true.\nDocker socket binds = container escape.\n")

	out := searchDocs(context.Background(), dir, searchDocsInput{Query: "docker", TopK: 10})
	if len(out.Matches) != 2 {
		t.Fatalf("got %d matches; want 2 (one per file). matches=%+v", len(out.Matches), out.Matches)
	}
	// Per-hit shape: concept stem set, line_number > 0, URI present.
	for i, h := range out.Matches {
		if h.Concept == "" || h.LineNumber == 0 || h.URI == "" {
			t.Errorf("match[%d] missing fields: %+v", i, h)
		}
	}
}

func TestSearchDocs_TopKHonoured(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, dir, "noisy.md", "match\nmatch\nmatch\nmatch\nmatch\nmatch\n")

	out := searchDocs(context.Background(), dir, searchDocsInput{Query: "match", TopK: 3})
	if len(out.Matches) != 3 {
		t.Errorf("got %d matches; top_k=3 should cap at 3", len(out.Matches))
	}
}

func TestSearchDocs_DefaultTopKWhenZero(t *testing.T) {
	dir := t.TempDir()
	body := ""
	for i := 0; i < 20; i++ {
		body += "match\n"
	}
	mustWrite(t, dir, "noisy.md", body)

	out := searchDocs(context.Background(), dir, searchDocsInput{Query: "match"})
	if len(out.Matches) != searchDocsDefaultTopK {
		t.Errorf("got %d matches; default top_k=%d", len(out.Matches), searchDocsDefaultTopK)
	}
}

func TestSearchDocs_ShortQueryReturnsNote(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, dir, "x.md", "anything")

	out := searchDocs(context.Background(), dir, searchDocsInput{Query: "x"})
	if len(out.Matches) != 0 {
		t.Errorf("got matches for 1-char query; want 0")
	}
	if out.Note == "" {
		t.Errorf("Note empty — agent needs a hint that query was too short")
	}
}

func TestSearchDocs_MissingDirReturnsNoteNotError(t *testing.T) {
	out := searchDocs(context.Background(), "/nonexistent/concepts", searchDocsInput{Query: "test", TopK: 5})
	if len(out.Matches) != 0 {
		t.Errorf("got matches against missing dir; want 0")
	}
	if out.Note == "" {
		t.Errorf("Note empty — agent needs a hint about why no matches")
	}
}

func TestSearchDocs_CaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, dir, "x.md", "PowerLab uses /DATA/PowerLabAppData paths\n")

	out := searchDocs(context.Background(), dir, searchDocsInput{Query: "POWERLAB", TopK: 5})
	if len(out.Matches) != 1 {
		t.Errorf("case-insensitive match failed; got %d, want 1", len(out.Matches))
	}
}

// REGRESSION (P0.3 from 2026-05-31 MCP-only chat-mode test): pre-fix
// search_docs only indexed conceptsDir, so a query for "install_app"
// (which lives in OpenAPI specs) or "vaultwarden" (which lives in
// the catalog) returned {matches: []} — the agent had no path from
// search to the canonical doc. This test locks the multi-source
// contract: concepts + OpenAPI + catalog are all searched, every
// hit carries its source + canonical URI.
func TestSearchDocsMulti_FindsAcrossConceptsOpenAPIAndCatalog(t *testing.T) {
	conceptsDir := t.TempDir()
	openAPIDir := t.TempDir()
	catalogDir := t.TempDir()

	// concepts — the original surface
	mustWrite(t, conceptsDir, "security-model.md",
		"PowerLab refuses privileged: true containers.\n")

	// openapi — service specs the agent should be able to grep
	mustWrite(t, openAPIDir, "app-management.yaml",
		"openapi: 3.0.3\npaths:\n  /install_app:\n    post:\n      summary: install an app\n")

	// catalog — app definitions in the bundled community catalog
	appDir := filepath.Join(catalogDir, "Apps", "vaultwarden")
	if err := os.MkdirAll(appDir, 0o750); err != nil {
		t.Fatalf("mkdir app: %v", err)
	}
	mustWrite(t, appDir, "docker-compose.yml",
		"services:\n  vaultwarden:\n    image: vaultwarden/server:1.32.7\n")

	roots := []searchRoot{
		{Source: "concepts", Path: conceptsDir, URIFn: func(stem string) string { return "docs://concepts/" + stem }},
		{Source: "openapi", Path: openAPIDir, URIFn: func(stem string) string { return "docs://api/" + stem }},
		{Source: "catalog", Path: catalogDir, URIFn: func(appID string) string { return "catalog://app/" + appID }},
	}

	// Query A — hits concepts only
	outA := searchDocsMulti(context.Background(), roots, searchDocsInput{Query: "privileged", TopK: 10})
	if len(outA.Matches) != 1 {
		t.Fatalf("'privileged' query: got %d matches; want 1 (concepts/security-model)", len(outA.Matches))
	}
	if outA.Matches[0].Source != "concepts" {
		t.Errorf("'privileged' source=%q; want concepts", outA.Matches[0].Source)
	}
	if outA.Matches[0].URI != "docs://concepts/security-model" {
		t.Errorf("'privileged' URI=%q; want docs://concepts/security-model", outA.Matches[0].URI)
	}

	// Query B — hits OpenAPI only ("install_app" is on the path line)
	outB := searchDocsMulti(context.Background(), roots, searchDocsInput{Query: "install_app", TopK: 10})
	if len(outB.Matches) != 1 {
		t.Fatalf("'install_app' query: got %d matches; want 1 (openapi/app-management)", len(outB.Matches))
	}
	if outB.Matches[0].Source != "openapi" {
		t.Errorf("'install_app' source=%q; want openapi", outB.Matches[0].Source)
	}
	if outB.Matches[0].URI != "docs://api/app-management" {
		t.Errorf("'install_app' URI=%q; want docs://api/app-management", outB.Matches[0].URI)
	}

	// Query C — hits catalog only (the app image tag)
	outC := searchDocsMulti(context.Background(), roots, searchDocsInput{Query: "vaultwarden/server", TopK: 10})
	if len(outC.Matches) != 1 {
		t.Fatalf("'vaultwarden/server' query: got %d matches; want 1 (catalog/vaultwarden)", len(outC.Matches))
	}
	if outC.Matches[0].Source != "catalog" {
		t.Errorf("'vaultwarden/server' source=%q; want catalog", outC.Matches[0].Source)
	}
	if outC.Matches[0].URI != "catalog://app/vaultwarden" {
		t.Errorf("'vaultwarden/server' URI=%q; want catalog://app/vaultwarden", outC.Matches[0].URI)
	}
}

// Missing dirs are NOT an error — a fresh box may not have all three
// indexed surfaces yet. Existing sources should still return hits;
// missing ones contribute zero. Note is populated when nothing
// matched anywhere so the agent gets a hint instead of a bare empty
// matches array.
func TestSearchDocsMulti_PartialMissingDirsAreNotErrors(t *testing.T) {
	conceptsDir := t.TempDir()
	mustWrite(t, conceptsDir, "x.md", "find me\n")

	roots := []searchRoot{
		{Source: "concepts", Path: conceptsDir, URIFn: func(s string) string { return "docs://concepts/" + s }},
		{Source: "openapi", Path: "/definitely/not/here", URIFn: func(s string) string { return "docs://api/" + s }},
		{Source: "catalog", Path: "/also/not/here", URIFn: func(s string) string { return "catalog://app/" + s }},
	}

	out := searchDocsMulti(context.Background(), roots, searchDocsInput{Query: "find me", TopK: 5})
	if len(out.Matches) != 1 {
		t.Fatalf("got %d matches; want 1 from the only present dir", len(out.Matches))
	}
	if out.Note != "" {
		t.Errorf("Note=%q; want empty when at least one source had a hit", out.Note)
	}
}

// All sources contributing simultaneously: TopK caps the total
// across sources, not per-source. Agents must see a stable cap.
func TestSearchDocsMulti_TopKCapsAggregate(t *testing.T) {
	conceptsDir := t.TempDir()
	openAPIDir := t.TempDir()
	mustWrite(t, conceptsDir, "a.md", "match\nmatch\nmatch\n")
	mustWrite(t, openAPIDir, "b.yaml", "match\nmatch\nmatch\n")

	roots := []searchRoot{
		{Source: "concepts", Path: conceptsDir, URIFn: func(s string) string { return "docs://concepts/" + s }},
		{Source: "openapi", Path: openAPIDir, URIFn: func(s string) string { return "docs://api/" + s }},
	}

	out := searchDocsMulti(context.Background(), roots, searchDocsInput{Query: "match", TopK: 4})
	if len(out.Matches) != 4 {
		t.Fatalf("got %d matches; TopK=4 should cap aggregate at 4 (had 3+3 available)", len(out.Matches))
	}
}
