package route

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/neochaotic/powerlab/backend/gateway/web"
)

// The embedded UI (ADR-0043) must be servable through the same
// serveSPAPath the on-disk path uses. This exercises the real
// embed.FS (not os.DirFS) so any embed-specific issue — ServeContent
// needing an io.ReadSeeker, http.FS path validation, the fs.Sub root —
// surfaces here. In CI the embedded build/ holds only the committed
// placeholder index.html; that is enough to prove the wiring.
func TestServeSPAPath_EmbeddedFSServesIndex(t *testing.T) {
	hfs := http.FS(web.FS())

	// Bare "/" and an unknown SPA route both resolve to index.html.
	for _, p := range []string{"/", "/some/client/route"} {
		t.Run(p, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, p, nil)
			rr := httptest.NewRecorder()
			if !serveSPAPath(rr, req, hfs, p) {
				t.Fatalf("embedded FS did not serve %q", p)
			}
			if rr.Code != http.StatusOK {
				t.Fatalf("%q status: got %d want 200", p, rr.Code)
			}
			if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
				t.Errorf("%q content-type: got %q", p, ct)
			}
			if !strings.Contains(rr.Body.String(), "<html") {
				t.Errorf("%q body is not an HTML document: %q", p, rr.Body.String())
			}
		})
	}
}

// A missing asset against the embedded FS must 404, not fall back to
// HTML — same content-type-sniffing guarantee as the on-disk path.
func TestServeSPAPath_EmbeddedFSMissingAsset404(t *testing.T) {
	hfs := http.FS(web.FS())

	req := httptest.NewRequest(http.MethodGet, "/_app/immutable/does-not-exist.js", nil)
	rr := httptest.NewRecorder()
	if serveSPAPath(rr, req, hfs, "/_app/immutable/does-not-exist.js") {
		t.Fatal("embedded FS should not have served a missing asset")
	}
}
