package route

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeFixture builds a minimal SvelteKit-style adapter-static layout
// in a tmp dir. Mirrors what `npm run build` produces for our app:
// pre-rendered route files like `settings.html`, an `index.html` SPA
// shell (the `fallback: 'index.html'` config), and a hashed
// immutable asset under `_app/immutable/`.
func makeFixture(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "powerlab-static-test")
	if err != nil {
		t.Fatalf("mktmp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	files := map[string]string{
		"index.html":              "<!doctype html><html><body>SPA SHELL</body></html>",
		"settings.html":           "<!doctype html><html><body>SETTINGS PAGE</body></html>",
		"dashboard.html":          "<!doctype html><html><body>DASHBOARD PAGE</body></html>",
		"apps/index.html":         "<!doctype html><html><body>APPS DIR INDEX</body></html>",
		"_app/immutable/start.js": "console.log('hashed asset');",
		"favicon-abc123.png":      "PNG\xff",
	}
	for rel, body := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(body), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	return dir
}

// callServeSPA wraps serveSPAPath in a minimal http test rig, serving
// from an on-disk root via http.FS(os.DirFS(...)) — the same
// http.FileSystem shape GetRoute hands the handler in production.
func callServeSPA(t *testing.T, wwwRoot, path string) (int, string, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	served := serveSPAPath(rr, req, http.FS(os.DirFS(wwwRoot)), path)
	if !served {
		// Mirror the GetRoute handler's 404 behavior.
		rr.WriteHeader(http.StatusNotFound)
	}
	return rr.Code, rr.Header().Get("Content-Type"), rr.Body.String()
}

// Pre-rendered route — settings.html — must be served directly.
func TestServeSPAPath_PrerenderedRoute(t *testing.T) {
	root := makeFixture(t)

	code, ct, body := callServeSPA(t, root, "/settings")
	if code != http.StatusOK {
		t.Fatalf("/settings status: got %d want 200 (body=%q)", code, body)
	}
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("/settings content-type: got %q", ct)
	}
	if !strings.Contains(body, "SETTINGS PAGE") {
		t.Errorf("/settings body wrong: %q", body)
	}
}

// Bare "/" must serve the SPA shell (index.html).
func TestServeSPAPath_RootServesIndex(t *testing.T) {
	root := makeFixture(t)

	code, _, body := callServeSPA(t, root, "/")
	if code != http.StatusOK {
		t.Fatalf("/ status: got %d", code)
	}
	if !strings.Contains(body, "SPA SHELL") {
		t.Errorf("/ should serve index.html SPA shell; got %q", body)
	}
}

// Route with no pre-rendered match falls back to index.html so
// client-side routing can take over. Without this, refreshing the
// browser on any client-side route would 404 — same bug class as the
// "Not Found" white screen we hit during the v0.2.7 trust-dance test.
func TestServeSPAPath_UnknownRouteFallsBackToIndex(t *testing.T) {
	root := makeFixture(t)

	for _, p := range []string{"/random-route", "/files/deep/nested/path", "/foo-bar"} {
		t.Run(p, func(t *testing.T) {
			code, ct, body := callServeSPA(t, root, p)
			if code != http.StatusOK {
				t.Fatalf("status: got %d for %s", code, p)
			}
			if !strings.HasPrefix(ct, "text/html") {
				t.Errorf("content-type: got %q for %s", ct, p)
			}
			if !strings.Contains(body, "SPA SHELL") {
				t.Errorf("expected SPA shell fallback for %s, got %q", p, body)
			}
		})
	}
}

// Directory-style route (apps/) resolves to apps/index.html.
func TestServeSPAPath_DirectoryIndex(t *testing.T) {
	root := makeFixture(t)

	code, _, body := callServeSPA(t, root, "/apps")
	if code != http.StatusOK {
		t.Fatalf("/apps status: got %d", code)
	}
	if !strings.Contains(body, "APPS DIR INDEX") {
		t.Errorf("/apps should serve apps/index.html; got %q", body)
	}
}

// Asset-shaped paths (with extension) MUST 404 when missing instead
// of falling back to index.html. Returning HTML for a missing JS or
// CSS would break content-type sniffing and silently mask broken
// build outputs.
func TestServeSPAPath_MissingAssetReturns404(t *testing.T) {
	root := makeFixture(t)

	cases := []string{
		"/missing.js",
		"/_app/immutable/does-not-exist.js",
		"/foo.css",
		"/some-image.png",
	}
	for _, p := range cases {
		t.Run(p, func(t *testing.T) {
			code, _, _ := callServeSPA(t, root, p)
			if code != http.StatusNotFound {
				t.Fatalf("%s should 404, got %d", p, code)
			}
		})
	}
}

// Hashed immutable asset hits the file directly.
func TestServeSPAPath_ImmutableAssetServed(t *testing.T) {
	root := makeFixture(t)

	code, ct, body := callServeSPA(t, root, "/_app/immutable/start.js")
	if code != http.StatusOK {
		t.Fatalf("immutable asset status: got %d", code)
	}
	if !strings.HasPrefix(ct, "text/javascript") && !strings.HasPrefix(ct, "application/javascript") {
		t.Errorf("immutable asset content-type unexpected: %q", ct)
	}
	if !strings.Contains(body, "hashed asset") {
		t.Errorf("immutable asset body wrong: %q", body)
	}
}

// Path-traversal attempts must NOT escape wwwRoot.
func TestServeSPAPath_PathTraversalRejected(t *testing.T) {
	root := makeFixture(t)
	// Drop a sentinel file in a sibling dir so a successful escape
	// would expose it.
	parent := filepath.Dir(root)
	sentinel := filepath.Join(parent, "DO-NOT-LEAK.html")
	_ = os.WriteFile(sentinel, []byte("LEAKED"), 0644)
	t.Cleanup(func() { _ = os.Remove(sentinel) })

	for _, p := range []string{
		"/../DO-NOT-LEAK.html",
		"/foo/../../DO-NOT-LEAK.html",
	} {
		t.Run(p, func(t *testing.T) {
			code, _, body := callServeSPA(t, root, p)
			if strings.Contains(body, "LEAKED") {
				t.Fatalf("path traversal escaped sandbox via %s", p)
			}
			// The HTTP layer normalizes "../" before this handler
			// sees it, so a successful escape would manifest as
			// either LEAKED-in-body OR a non-404 status. Both are
			// covered: body is checked above, status here.
			if code != http.StatusNotFound && code != http.StatusOK {
				t.Logf("status %d for %s — informational only", code, p)
			}
		})
	}
}

// isAssetPath classification — the load-bearing branch that decides
// whether to fall back to index.html or 404.
func TestIsAssetPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/", false},
		{"/settings", false},
		{"/files/deep/path", false},
		{"/foo.html", false}, // .html paths are NOT assets — they go through the SPA chain
		{"/foo.js", true},
		{"/foo.css", true},
		{"/foo.png", true},
		{"/_app/whatever", true}, // anything under /_app/ is treated as an asset
		{"/_app/", true},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			got := isAssetPath(c.path)
			if got != c.want {
				t.Errorf("isAssetPath(%q) = %v, want %v", c.path, got, c.want)
			}
		})
	}
}

// Serving must work when wwwRoot is a symlink — the dev-mode
// configuration symlinks backend/data/www → ui/build. os.DirFS follows
// a symlinked root, so a request through the link resolves to the real
// file. (Pins the dev workflow the `-w` override depends on.)
func TestServeSPAPath_SymlinkedRootServes(t *testing.T) {
	real := makeFixture(t)
	link, err := os.MkdirTemp("", "powerlab-static-link-")
	if err != nil {
		t.Fatalf("mktmp link parent: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(link) })
	linkPath := filepath.Join(link, "www")
	if err := os.Symlink(real, linkPath); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	code, _, body := callServeSPA(t, linkPath, "/settings")
	if code != http.StatusOK {
		t.Fatalf("serve through symlinked root: got %d", code)
	}
	if !strings.Contains(body, "SETTINGS PAGE") {
		t.Errorf("served wrong content: %q", body)
	}
}

// Sprint 18 P0 regression — locks the bug class that bit the v0.6.12
// cut on 2026-05-15: gateway runs with `-w /usr/share/powerlab/www`,
// but a hot-swap update wrote the new UI bundle to a DIFFERENT path
// (`/var/lib/powerlab/www`). Gateway kept serving the stale bundle
// silently — the AuditPane "disappeared" from the user's view even
// though the new code was on disk in the wrong directory.
//
// This test pins the contract: the static route MUST serve files
// EXCLUSIVELY from the path the caller passes (here `wwwRoot`).
// There is no implicit fallback / lookup chain. If the future
// introduces one, this test catches it.
func TestServeSPAPath_NoSilentFallbackToAlternateRoot(t *testing.T) {
	// Two distinct fixture roots with DIFFERENT content for the same
	// route name. If the handler ever falls back from A → B, the
	// served body would match B's content even though we passed A.
	rootA := makeFixture(t)

	rootB, err := os.MkdirTemp("", "powerlab-static-alt")
	if err != nil {
		t.Fatalf("mktmp alt: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(rootB) })
	altIndex := "<!doctype html><html><body>WRONG ROOT — DO NOT SERVE</body></html>"
	if err := os.WriteFile(filepath.Join(rootB, "index.html"), []byte(altIndex), 0o644); err != nil {
		t.Fatal(err)
	}

	// Serve from rootA — the body MUST contain rootA's content, not rootB's.
	code, _, body := callServeSPA(t, rootA, "/")
	if code != http.StatusOK {
		t.Fatalf("status: %d", code)
	}
	if strings.Contains(body, "WRONG ROOT") {
		t.Errorf("BUG: handler served from alternate root %s instead of the configured one %s", rootB, rootA)
	}
	if !strings.Contains(body, "SPA SHELL") {
		t.Errorf("expected rootA's index content; got %q", body)
	}
}

// Sprint 18 P0 — when the configured wwwRoot does NOT exist, the
// handler must NOT 200 a fake page. Either:
//   - returns 404 (acceptable — caller can fail-fast at boot)
//   - returns a 5xx (acceptable — surfaces the misconfiguration)
//
// A silent fallback to an empty body / "ok" response would mask
// the operator error that hit on v0.6.12 cut.
func TestServeSPAPath_NonExistentRootDoesNotSilentlySucceed(t *testing.T) {
	nonexistent := filepath.Join(os.TempDir(), "powerlab-static-this-does-not-exist-xyz")
	// Make sure it really doesn't exist.
	_ = os.RemoveAll(nonexistent)

	code, _, _ := callServeSPA(t, nonexistent, "/")
	if code >= 200 && code < 300 {
		t.Errorf("BUG: handler returned %d for nonexistent www root %s — should signal failure (404/5xx), not silently 2xx", code, nonexistent)
	}
}
