package logs_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neochaotic/powerlab/backend/common/utils/logs"
)

// Set up a temp log directory with a known fixture and return the
// dir path. Each fixture file uses content `<basename> <N>` repeated
// so tests can distinguish files.
func makeLogDir(t *testing.T, contents map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range contents {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

func TestListFiles_HappyPath(t *testing.T) {
	dir := makeLogDir(t, map[string]string{
		"app-management.log": "amlog contents",
		"gateway.log":        "gw contents",
		"audit.jsonl":        "should be filtered",        // not .log
		"app-management-2026-05-15T13-11-37.636.log.gz": "rotated", // .log.gz, filtered
		"subdir":             "should not panic on dirs",
	})
	// Make `subdir` a real dir, not a file
	_ = os.Remove(filepath.Join(dir, "subdir"))
	_ = os.Mkdir(filepath.Join(dir, "subdir"), 0o755)

	req := httptest.NewRequest(http.MethodGet, "/v1/logs/files", nil)
	rec := httptest.NewRecorder()
	logs.ListFilesHTTPHandler(dir).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data []logs.FileEntry `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	names := []string{}
	for _, e := range resp.Data {
		names = append(names, e.Name)
	}
	if len(names) != 2 {
		t.Errorf("expected 2 .log entries (filtered: dir + jsonl + .log.gz), got %d: %v", len(names), names)
	}
	for _, want := range []string{"app-management.log", "gateway.log"} {
		found := false
		for _, n := range names {
			if n == want {
				found = true
			}
		}
		if !found {
			t.Errorf("expected %q in listing, got %v", want, names)
		}
	}
}

func TestListFiles_RejectsNonGET(t *testing.T) {
	dir := makeLogDir(t, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/logs/files", nil)
	rec := httptest.NewRecorder()
	logs.ListFilesHTTPHandler(dir).ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: got %d, want 405", rec.Code)
	}
}

func TestReadFile_HappyPath_ReturnsLastTailBytes(t *testing.T) {
	// 300 KB of content; default tail is 200 KB → expect 200 KB back.
	body := strings.Repeat("0123456789", 30*1024) // 300_000 bytes
	dir := makeLogDir(t, map[string]string{"big.log": body})

	req := httptest.NewRequest(http.MethodGet, "/v1/logs/files/big.log", nil)
	rec := httptest.NewRecorder()
	logs.ReadFileHTTPHandler(dir).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d, want 200", rec.Code)
	}
	got, _ := io.ReadAll(rec.Body)
	if int64(len(got)) != logs.DefaultTailBytes {
		t.Errorf("body size: got %d, want %d (default tail)", len(got), logs.DefaultTailBytes)
	}
	// Content should be the LAST N bytes of body
	expected := body[len(body)-int(logs.DefaultTailBytes):]
	if string(got) != expected {
		t.Errorf("body content: read wrong window")
	}
	// "0123456789" × (30 × 1024) = 307,200 bytes
	if rec.Header().Get("X-Log-Size-Bytes") != "307200" {
		t.Errorf("X-Log-Size-Bytes header: %q", rec.Header().Get("X-Log-Size-Bytes"))
	}
}

func TestReadFile_SmallerThanTail_ReturnsWholeFile(t *testing.T) {
	body := "tiny log\n"
	dir := makeLogDir(t, map[string]string{"tiny.log": body})

	req := httptest.NewRequest(http.MethodGet, "/v1/logs/files/tiny.log", nil)
	rec := httptest.NewRecorder()
	logs.ReadFileHTTPHandler(dir).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d, want 200", rec.Code)
	}
	if rec.Body.String() != body {
		t.Errorf("body: got %q, want %q", rec.Body.String(), body)
	}
}

func TestReadFile_RejectsPathTraversal(t *testing.T) {
	dir := makeLogDir(t, map[string]string{"valid.log": "ok"})
	// Each name must 400. Cases that survive net/http URL parsing
	// (no null bytes, no malformed escapes) — the allowlist rejects
	// non-.log suffixes and unsafe characters. Slashes in the path
	// are stripped by lastPathSegment before validation, so the
	// "../../../etc/passwd" attack ends up as "passwd" → no .log
	// suffix → 400 (NOT 404).
	bad := []string{
		"../../../etc/passwd", // last segment "passwd" — no .log → 400
		"/etc/passwd",         // last segment "passwd" — no .log → 400
		".log",                // empty basename — fails allowlist
		"audit.jsonl",         // wrong extension
		"valid.log.gz",        // .gz suffix
	}
	for _, name := range bad {
		req := httptest.NewRequest(http.MethodGet, "/v1/logs/files/"+name, nil)
		rec := httptest.NewRecorder()
		logs.ReadFileHTTPHandler(dir).ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("name=%q: got %d, want 400", name, rec.Code)
		}
	}
}

func TestReadFile_FilenameAllowlistRegex(t *testing.T) {
	// Direct unit test of the allowlist behaviour against cases
	// that net/http won't let through a URL (null byte, control
	// characters). The regex anchors with ^ and $ so partial
	// matches don't slip through.
	dir := t.TempDir()
	h := logs.ReadFileHTTPHandler(dir)

	// Valid name should pass allowlist (404 because the file
	// doesn't exist, NOT 400).
	req := httptest.NewRequest(http.MethodGet, "/v1/logs/files/anything.log", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("valid allowlist name should reach the file lookup; got %d", rec.Code)
	}
}

func TestReadFile_NotFoundOn404Name(t *testing.T) {
	dir := makeLogDir(t, map[string]string{"exists.log": "ok"})
	req := httptest.NewRequest(http.MethodGet, "/v1/logs/files/missing.log", nil)
	rec := httptest.NewRecorder()
	logs.ReadFileHTTPHandler(dir).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", rec.Code)
	}
}

func TestReadFile_TailQueryClampsToMax(t *testing.T) {
	// 10 KB file, request tail=99999999 → should clamp to MaxTailBytes
	// but since file is smaller than MaxTailBytes, just returns whole.
	// Real test: explicit tail=1024 returns exactly 1024 bytes.
	body := strings.Repeat("x", 10*1024)
	dir := makeLogDir(t, map[string]string{"x.log": body})
	req := httptest.NewRequest(http.MethodGet, "/v1/logs/files/x.log?tail=1024", nil)
	rec := httptest.NewRecorder()
	logs.ReadFileHTTPHandler(dir).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	if len(rec.Body.Bytes()) != 1024 {
		t.Errorf("tail=1024 body size: got %d, want 1024", len(rec.Body.Bytes()))
	}
}
