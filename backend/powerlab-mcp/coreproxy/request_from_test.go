package coreproxy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// RequestFrom is the verb-and-body capable extension of GetFrom
// added for ADR-0046 write tools (restart_app, install_app, ...).
// This test pins the contract: method + body + Content-Type all
// propagate to the upstream verbatim.
func TestRequestFrom_RoundTripsVerbAndBody(t *testing.T) {
	var gotMethod, gotPath, gotContentType, gotUA string
	var gotBody []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		gotUA = r.Header.Get("User-Agent")
		gotBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, AppsURLFile), []byte(upstream.URL), 0o600); err != nil {
		t.Fatalf("write apps url: %v", err)
	}

	body := []byte(`"restart"`)
	out, err := NewClient(dir, upstream.Client()).RequestFrom(
		context.Background(), http.MethodPut, ServiceApps,
		"/v2/app_management/compose/plex/status", "", body, "application/json")
	if err != nil {
		t.Fatalf("RequestFrom: %v", err)
	}
	if string(out) != `{"ok":true}` {
		t.Fatalf("body not round-tripped: %q", out)
	}

	if gotMethod != http.MethodPut {
		t.Fatalf("upstream method=%q; want PUT", gotMethod)
	}
	if gotPath != "/v2/app_management/compose/plex/status" {
		t.Fatalf("upstream path=%q", gotPath)
	}
	if gotContentType != "application/json" {
		t.Fatalf("upstream Content-Type=%q; want application/json", gotContentType)
	}
	if string(gotBody) != `"restart"` {
		t.Fatalf("upstream body=%q; want '\"restart\"'", gotBody)
	}
	if !strings.Contains(gotUA, "powerlab-mcp/proxy") {
		t.Fatalf("upstream User-Agent=%q; want it to identify MCP", gotUA)
	}
}

// A PUT against a 5xx upstream surfaces as apps_status_500 — same
// pattern GetFrom uses, with the verb in the Detail string so an
// operator reading the audit can tell read vs write apart.
func TestRequestFrom_NonOKStatusIncludesVerbInDetail(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"msg":"boom"}`))
	}))
	defer upstream.Close()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, AppsURLFile), []byte(upstream.URL), 0o600); err != nil {
		t.Fatalf("write url: %v", err)
	}
	_, err := NewClient(dir, upstream.Client()).RequestFrom(
		context.Background(), http.MethodPut, ServiceApps,
		"/anything", "", []byte(`{}`), "application/json")
	pe, ok := err.(*Error)
	if !ok {
		t.Fatalf("non-2xx returned %T; want *Error", err)
	}
	if pe.Code != "apps_status_500" {
		t.Fatalf("Code=%q; want apps_status_500", pe.Code)
	}
	if !strings.Contains(pe.Detail, "PUT") {
		t.Fatalf("Detail=%q; want it to mention the verb so audit readers can distinguish reads from writes", pe.Detail)
	}
	if !strings.Contains(pe.Body, "boom") {
		t.Fatalf("Body=%q; want the upstream payload preserved", pe.Body)
	}
}

// GetFrom still works after the refactor — backwards compat for
// every existing call site (system://* resources, apps:// resources).
func TestGetFrom_StillWorksAfterRequestFromRefactor(t *testing.T) {
	want := `{"cpu":42}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("upstream got %s; want GET", r.Method)
		}
		_, _ = w.Write([]byte(want))
	}))
	defer upstream.Close()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, CoreURLFile), []byte(upstream.URL), 0o600); err != nil {
		t.Fatalf("write url: %v", err)
	}
	got, err := NewClient(dir, upstream.Client()).GetFrom(context.Background(), ServiceCore, "/v1/sys/utilization", "")
	if err != nil {
		t.Fatalf("GetFrom: %v", err)
	}
	if string(got) != want {
		t.Fatalf("body=%q; want %q", got, want)
	}
}
