package v1

// Handler tests for the file download routes (#39): Content-Disposition
// must reach the client (it used to be set on the request), carry an
// RFC 6266 filename*= for non-ASCII names, and user-controlled bodies
// must ship with a CSP that stops uploaded HTML/SVG from running script
// in the panel origin.

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func writeTempFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func serve(t *testing.T, h echo.HandlerFunc, target string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	if err := h(e.NewContext(req, rec)); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	return rec
}

func TestGetDownloadSingleFile_HTMLIsSandboxedAndInline(t *testing.T) {
	p := writeTempFile(t, t.TempDir(), "evil.html", "<script>alert(document.cookie)</script>")
	rec := serve(t, GetDownloadSingleFile, "/v1/file?path="+url.QueryEscape(p))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if got, want := rec.Header().Get("Content-Security-Policy"), "sandbox; script-src 'none'"; got != want {
		t.Errorf("CSP: got %q, want %q", got, want)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options: got %q, want nosniff", got)
	}
	if got, want := rec.Header().Get("Content-Disposition"), `inline; filename="evil.html"; filename*=UTF-8''evil.html`; got != want {
		t.Errorf("Content-Disposition: got %q, want %q", got, want)
	}
}

func TestGetDownloadSingleFile_NonASCIIFilename(t *testing.T) {
	p := writeTempFile(t, t.TempDir(), "relatório; 日本.pdf", "%PDF-1.4\n")
	rec := serve(t, GetDownloadSingleFile, "/v1/file?path="+url.QueryEscape(p))

	want := `inline; filename="relat_rio; __.pdf"; filename*=UTF-8''relat%C3%B3rio%3B%20%E6%97%A5%E6%9C%AC.pdf`
	if got := rec.Header().Get("Content-Disposition"); got != want {
		t.Errorf("Content-Disposition:\n got %q\nwant %q", got, want)
	}
	// PDFs keep script blocked but skip `sandbox`, which would stop the
	// browser's built-in PDF viewer used by the preview drawer.
	if got, want := rec.Header().Get("Content-Security-Policy"), "script-src 'none'"; got != want {
		t.Errorf("CSP: got %q, want %q", got, want)
	}
}

func TestGetDownloadFile_SingleFileAttachment(t *testing.T) {
	p := writeTempFile(t, t.TempDir(), "naïve.html", "<h1>x</h1>")
	rec := serve(t, GetDownloadFile, "/v1/batch?files="+url.QueryEscape(p))

	if got, want := rec.Header().Get("Content-Disposition"), `attachment; filename="na_ve.html"; filename*=UTF-8''na%C3%AFve.html`; got != want {
		t.Errorf("Content-Disposition: got %q, want %q", got, want)
	}
	if got := rec.Header().Get("Content-Security-Policy"); got != "sandbox; script-src 'none'" {
		t.Errorf("CSP: got %q", got)
	}
	if got := rec.Body.String(); got != "<h1>x</h1>" {
		t.Errorf("body: got %q, want only the file (no archive appended)", got)
	}
}

func TestGetDownloadFile_ArchiveHeaders(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fotos-ção")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	a := writeTempFile(t, dir, "a.html", "<script></script>")
	b := writeTempFile(t, dir, "b.txt", "b")
	rec := serve(t, GetDownloadFile, "/v1/batch?format=zip&files="+url.QueryEscape(a)+","+url.QueryEscape(b))

	if got, want := rec.Header().Get("Content-Disposition"), `attachment; filename="_fotos-__o.zip"; filename*=UTF-8''_fotos-%C3%A7%C3%A3o.zip`; got != want {
		t.Errorf("Content-Disposition: got %q, want %q", got, want)
	}
	if got := rec.Header().Get("Content-Security-Policy"); got != "sandbox; script-src 'none'" {
		t.Errorf("CSP: got %q", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options: got %q", got)
	}
	if !strings.HasPrefix(rec.Body.String(), "PK") {
		t.Errorf("body is not a zip stream")
	}
}

func TestContentDisposition_EscapesHeaderMetacharacters(t *testing.T) {
	got := contentDisposition("attachment", `a"b\c,d;e.txt`)
	want := `attachment; filename="a_b_c,d;e.txt"; filename*=UTF-8''a%22b%5Cc%2Cd%3Be.txt`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
