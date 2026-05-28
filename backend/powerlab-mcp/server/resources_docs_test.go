package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// A box without /usr/share/powerlab/openapi (Mac dev box, pre-install)
// must return an empty manifest, not an error — agents calling
// docs://api on a fresh box get [] and can move on. Graceful
// degradation matches audittail / journal patterns.
func TestDocsAPIManifest_MissingDirIsEmptyNotError(t *testing.T) {
	srv := newMCPServerWithDocs(BuildInfo{Version: "test"}, filepath.Join(t.TempDir(), "no-such-dir"))
	cs := connectInProcess(t, srv)
	defer cs.Close()
	res, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: docsAPIURI})
	if err != nil {
		t.Fatalf("ReadResource(docs://api): %v", err)
	}

	var got docsManifest
	if uerr := json.Unmarshal([]byte(res.Contents[0].Text), &got); uerr != nil {
		t.Fatalf("payload not valid JSON: %v\n%s", uerr, res.Contents[0].Text)
	}
	if got.Description == "" {
		t.Fatalf("manifest description empty; want it set even for an empty listing")
	}
	if len(got.Specs) != 0 {
		t.Fatalf("got %d specs from missing dir; want 0", len(got.Specs))
	}
}

// The manifest sorted-by-service so the agent's output is stable;
// service descriptions come from the curated map so the agent can
// pick a spec without reading a single byte of YAML.
func TestDocsAPIManifest_ListsBundledSpecs(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"core.yaml", "gateway.yaml", "app-management.yaml", "unknown.yaml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("openapi: 3.0.0\n"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	srv := newMCPServerWithDocs(BuildInfo{Version: "test"}, dir)
	cs := connectInProcess(t, srv)
	defer cs.Close()
	res, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: docsAPIURI})
	if err != nil {
		t.Fatalf("ReadResource(docs://api): %v", err)
	}

	var got docsManifest
	if uerr := json.Unmarshal([]byte(res.Contents[0].Text), &got); uerr != nil {
		t.Fatalf("payload not valid JSON: %v", uerr)
	}
	if len(got.Specs) != 4 {
		t.Fatalf("got %d specs; want 4 (core, gateway, app-management, unknown)", len(got.Specs))
	}
	// Sorted alphabetically.
	want := []string{"app-management", "core", "gateway", "unknown"}
	for i, w := range want {
		if got.Specs[i].Service != w {
			t.Fatalf("got specs[%d] = %q; want %q (sorted)", i, got.Specs[i].Service, w)
		}
		if got.Specs[i].URI != docsAPIPrefix+w {
			t.Fatalf("got specs[%d].URI = %q; want %q (canonical pattern)", i, got.Specs[i].URI, docsAPIPrefix+w)
		}
	}
	// Curated descriptions land on known services; unknown ones have
	// empty descriptions (URI still works).
	descIndex := map[string]string{}
	for _, s := range got.Specs {
		descIndex[s.Service] = s.Description
	}
	if descIndex["core"] == "" {
		t.Fatalf("core description empty; the curated map must be applied")
	}
	if descIndex["unknown"] != "" {
		t.Fatalf("unknown service has description %q; want empty (no curated entry)", descIndex["unknown"])
	}
}

// docs://api/<svc> returns the raw YAML byte-for-byte — the agent
// must be able to feed the spec into whatever OpenAPI parser it
// chooses without MCP transforming the content.
func TestDocsAPIService_ReturnsRawYAML(t *testing.T) {
	dir := t.TempDir()
	body := "openapi: 3.0.0\ninfo:\n  title: PowerLab core\n  version: \"1\"\npaths: {}\n"
	if err := os.WriteFile(filepath.Join(dir, "core.yaml"), []byte(body), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	srv := newMCPServerWithDocs(BuildInfo{Version: "test"}, dir)
	cs := connectInProcess(t, srv)
	defer cs.Close()
	res, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: docsAPIPrefix + "core"})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	if res.Contents[0].Text != body {
		t.Fatalf("got payload %q; want raw fixture body verbatim", res.Contents[0].Text)
	}
	if res.Contents[0].MIMEType != "application/yaml" {
		t.Fatalf("got MIMEType %q; want application/yaml", res.Contents[0].MIMEType)
	}
}

// Path-traversal safety: a malicious URI such as docs://api/../etc/passwd
// must NOT yield any /etc/passwd bytes — whether the rejection happens
// at the SDK URI-template matcher (path separator stops `{service}`
// matching) or at our own validation in readSpec. We assert the
// end-to-end property: no error-free read against the traversal URIs.
func TestDocsAPIService_RejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()
	// Plant an evidence file ONE LEVEL UP from the openAPIDir so a
	// successful traversal would actually leak it. If the agent reads
	// bytes, this test catches it via the body content.
	leak := "ROOT:x:0:0:would-be-leaked"
	if err := os.WriteFile(filepath.Join(filepath.Dir(dir), "passwd"), []byte(leak), 0o600); err != nil {
		t.Fatalf("plant evidence: %v", err)
	}
	srv := newMCPServerWithDocs(BuildInfo{Version: "test"}, dir)
	cs := connectInProcess(t, srv)
	defer cs.Close()
	cases := []string{
		docsAPIPrefix + "../passwd",
		docsAPIPrefix + "foo/bar",
		docsAPIPrefix + ".",
		docsAPIPrefix + "..",
	}
	for _, uri := range cases {
		res, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: uri})
		if err == nil {
			// Some traversal shapes might match the URI template and
			// reach our handler. Either path-traversal is rejected by
			// the SDK (err != nil) OR our handler bounces it. What
			// MUST not happen: the plant ever escapes.
			for _, c := range res.Contents {
				if strings.Contains(c.Text, leak) {
					t.Fatalf("read %s leaked the planted file content — path-traversal escape!", uri)
				}
			}
		}
	}
}

// Reading docs://api/<svc> where the spec doesn't exist returns a
// pointer-back to docs://api so the agent knows how to discover what
// IS available. Important UX detail — without it the agent gets a
// generic "file not found" and the operator has to chase the
// filesystem layout.
func TestDocsAPIService_MissingSpecPointsBackToManifest(t *testing.T) {
	dir := t.TempDir()
	srv := newMCPServerWithDocs(BuildInfo{Version: "test"}, dir)
	cs := connectInProcess(t, srv)
	defer cs.Close()
	_, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: docsAPIPrefix + "nonexistent"})
	if err == nil {
		t.Fatalf("read of missing spec succeeded; want a friendly error")
	}
	if !strings.Contains(err.Error(), "docs://api") {
		t.Fatalf("error %v does not point the agent at docs://api for discovery", err)
	}
}

// newMCPServerWithDocs builds an MCP server that ONLY has docs://
// resources registered, for focused tests. Production wiring goes
// through newMCPServer in server.go which registers everything.
func newMCPServerWithDocs(info BuildInfo, openAPIDir string) *mcp.Server {
	m := mcp.NewServer(&mcp.Implementation{Name: "powerlab-mcp", Version: info.Version}, nil)
	registerDocs(m, openAPIDir)
	return m
}
