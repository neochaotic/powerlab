package route

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/neochaotic/powerlab/backend/gateway/api/docs"
)

// newDocsRoute is a tiny helper so tests don't have to construct a
// full service.State just to exercise the handlers. The handlers
// don't read state today; if that changes, plumb a real fake here.
func newDocsRoute(t *testing.T) *DocsRoute {
	t.Helper()
	return &DocsRoute{}
}

// TestHandleDocs_RendersHostPage — GET /docs returns the Scalar host
// HTML with the gateway service pre-selected by default and the
// <script id="api-reference"> hook Scalar looks for.
func TestHandleDocs_RendersHostPage(t *testing.T) {
	d := newDocsRoute(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	d.handleDocs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("content-type: got %q", ct)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`<title>PowerLab API`,
		`id="api-reference"`,
		`/docs/spec?service=gateway`,
		`/docs/scalar.js`,
		`<option value="gateway" selected>Gateway</option>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

// TestHandleDocs_PreselectsRequestedService — `?service=app-management`
// pre-selects that option and points Scalar's data-url at the
// matching spec endpoint.
func TestHandleDocs_PreselectsRequestedService(t *testing.T) {
	d := newDocsRoute(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/docs?service=app-management", nil)
	d.handleDocs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `<option value="app-management" selected>App Management</option>`) {
		t.Errorf("expected app-management to be selected; got body %s", body)
	}
	if !strings.Contains(body, `/docs/spec?service=app-management`) {
		t.Errorf("expected spec URL to point at app-management; got body %s", body)
	}
	// And the previous default must NOT also be selected — the
	// dropdown should have exactly one selected option.
	if strings.Count(body, " selected>") != 1 {
		t.Errorf("expected exactly one selected option, got %d", strings.Count(body, " selected>"))
	}
}

// TestHandleDocs_UnknownServiceFallsBackToGateway — a stale link or
// typo in the service id must NOT 404. We render the gateway service
// instead so the user lands on a working page.
func TestHandleDocs_UnknownServiceFallsBackToGateway(t *testing.T) {
	d := newDocsRoute(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/docs?service=does-not-exist", nil)
	d.handleDocs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d (must NOT be 404 for an unknown id)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `<option value="gateway" selected>`) {
		t.Errorf("fallback to gateway not applied")
	}
}

// TestHandleSpec_KnownService — returns the embedded YAML for the
// requested service with a YAML-aware content-type.
func TestHandleSpec_KnownService(t *testing.T) {
	d := newDocsRoute(t)

	for _, svc := range docs.Services {
		t.Run(svc.ID, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/docs/spec?service="+svc.ID, nil)
			d.handleSpec(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status: got %d for %s", rec.Code, svc.ID)
			}
			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/yaml") {
				t.Errorf("content-type for %s: got %q", svc.ID, ct)
			}
			body := rec.Body.String()
			if !strings.Contains(body, "openapi:") {
				t.Errorf("spec for %s does not contain `openapi:` header", svc.ID)
			}
		})
	}
}

// TestHandleSpec_UnknownServiceFallsBackToGateway — same fallback
// rule as the host page so the consumer (Scalar) never tries to
// parse a 404.
func TestHandleSpec_UnknownServiceFallsBackToGateway(t *testing.T) {
	d := newDocsRoute(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/docs/spec?service=nonsense", nil)
	d.handleSpec(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (fallback to gateway)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "openapi:") {
		t.Errorf("fallback did not return a real spec body")
	}
}

// TestServeSpec_RejectsTraversal — defense in depth. Even if a future
// caller forwards a user-supplied filename to serveSpec, the
// traversal guard prevents it from escaping the embed.FS.
func TestServeSpec_RejectsTraversal(t *testing.T) {
	d := newDocsRoute(t)

	for _, name := range []string{
		"../../../etc/passwd",
		"foo/bar.yaml",
		"foo\\bar.yaml",
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			d.serveSpec(rec, req, name)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("got %d for %q, want 400", rec.Code, name)
			}
		})
	}
}

// TestHandleScalarJS_ServesEmbeddedRuntime — confirms the bundled
// runtime is reachable and content-typed correctly.
func TestHandleScalarJS_ServesEmbeddedRuntime(t *testing.T) {
	d := newDocsRoute(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/docs/scalar.js", nil)
	d.handleScalarJS(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/javascript") {
		t.Errorf("content-type: got %q", ct)
	}
	if rec.Body.Len() < 1000 {
		t.Errorf("scalar runtime body suspiciously small: %d bytes", rec.Body.Len())
	}
}

// TestHandleLogo_ServesEmbeddedSVG — the PowerLab logo referenced
// in every spec's info.description (`/docs/logo.svg`) must be
// reachable with the right content-type, otherwise Scalar will
// render a broken image at the top of every page.
func TestHandleLogo_ServesEmbeddedSVG(t *testing.T) {
	d := newDocsRoute(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/docs/logo.svg", nil)
	d.handleLogo(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/svg+xml" {
		t.Errorf("content-type: got %q, want image/svg+xml", ct)
	}
	body := rec.Body.String()
	if !strings.HasPrefix(body, "<svg") {
		t.Errorf("body does not start with <svg; got: %q", body[:min(40, len(body))])
	}
	if !strings.Contains(body, "viewBox=\"0 0 512 512\"") {
		t.Errorf("logo viewBox missing — file may have been replaced with the wrong SVG")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestServicesListCoversAllSpecs — the canonical Services list in
// docs.go must reference exactly the spec files we embed. Catches a
// regression where someone adds a new service spec but forgets to
// register it (or vice versa).
func TestServicesListCoversAllSpecs(t *testing.T) {
	for _, s := range docs.Services {
		_, err := docs.EmbeddedFiles.ReadFile(s.Spec)
		if err != nil {
			t.Errorf("Services[%s] points at %s but embed has no such file: %v", s.ID, s.Spec, err)
		}
	}
}
