package server

import (
	"context"
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
