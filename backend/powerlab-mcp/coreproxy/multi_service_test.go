package coreproxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ADR-0045 generalises the proxy to a second upstream. ResolveCore
// continues to work (#609/#610 callers); the new code path is
// Resolve("apps") + GetFrom(ctx, "apps", ...).
//
// This test pins the generalisation: Resolve(ServiceApps) reads the
// `app-management.url` file and returns the URL, exactly the same
// way Resolve(ServiceCore) reads casaos.url.
func TestResolve_AppsReadsAppManagementURLFile(t *testing.T) {
	dir := t.TempDir()
	want := "http://127.0.0.1:9876"
	if err := os.WriteFile(filepath.Join(dir, AppsURLFile), []byte(want), 0o600); err != nil {
		t.Fatalf("write apps url file: %v", err)
	}
	got, err := NewClient(dir, nil).Resolve(ServiceApps)
	if err != nil {
		t.Fatalf("Resolve(apps): %v", err)
	}
	if got != want {
		t.Fatalf("Resolve(apps) = %q; want %q", got, want)
	}
}

// Missing the apps .url file must surface as apps_unavailable —
// distinct from core_unavailable so an agent can pivot precisely:
// when apps:// is down but system:// is fine, it still drives
// system:// reads. The shape contract is identical (Code suffix
// `_unavailable`, Detail mentions the missing file path).
func TestResolve_MissingAppsURLFileSurfacesAsAppsUnavailable(t *testing.T) {
	dir := t.TempDir()
	// Plant a CORE url file so this test exercises ONLY the apps
	// resolution path — a fully-empty dir would also trip core
	// missing-file checks and confuse the assertion.
	if err := os.WriteFile(filepath.Join(dir, CoreURLFile), []byte("http://127.0.0.1:8810"), 0o600); err != nil {
		t.Fatalf("write core url: %v", err)
	}
	_, err := NewClient(dir, nil).Resolve(ServiceApps)
	pe, ok := err.(*Error)
	if !ok {
		t.Fatalf("Resolve(apps) on missing apps url returned %T; want *Error", err)
	}
	if pe.Code != "apps_unavailable" {
		t.Fatalf("Code=%q; want apps_unavailable", pe.Code)
	}
	if !strings.Contains(pe.Detail, AppsURLFile) {
		t.Fatalf("Detail=%q; want it to mention %q so the operator can locate the missing file", pe.Detail, AppsURLFile)
	}
}

// An unknown service ID must surface as <id>_unavailable with a
// helpful Detail listing the known services. A typo or refactor
// regression — `GetFrom(ctx, "appps", ...)` — produces a structured
// error the operator can spot, not a panic.
func TestResolve_UnknownServiceListsKnownIDsInDetail(t *testing.T) {
	_, err := NewClient(t.TempDir(), nil).Resolve("madeup")
	pe, ok := err.(*Error)
	if !ok {
		t.Fatalf("Resolve(unknown) returned %T; want *Error", err)
	}
	if pe.Code != "madeup_unavailable" {
		t.Fatalf("Code=%q; want madeup_unavailable (canonical <service>_unavailable shape)", pe.Code)
	}
	for _, want := range []string{ServiceCore, ServiceApps} {
		if !strings.Contains(pe.Detail, want) {
			t.Fatalf("Detail=%q; want it to list known service %q so a typo is debuggable", pe.Detail, want)
		}
	}
}

// Each upstream URL is cached separately — resolving core then apps
// shouldn't clobber the core cache. This guards against a sloppy
// shared-cache implementation where the second resolve evicts the
// first.
func TestResolve_PerServiceCacheIsolation(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, CoreURLFile), []byte("http://127.0.0.1:8810"), 0o600); err != nil {
		t.Fatalf("write core url: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, AppsURLFile), []byte("http://127.0.0.1:9876"), 0o600); err != nil {
		t.Fatalf("write apps url: %v", err)
	}
	c := NewClient(dir, nil)

	core1, err := c.Resolve(ServiceCore)
	if err != nil {
		t.Fatalf("Resolve(core): %v", err)
	}
	apps1, err := c.Resolve(ServiceApps)
	if err != nil {
		t.Fatalf("Resolve(apps): %v", err)
	}
	// Delete BOTH files — only the cache can satisfy the next reads.
	_ = os.Remove(filepath.Join(dir, CoreURLFile))
	_ = os.Remove(filepath.Join(dir, AppsURLFile))

	core2, err := c.Resolve(ServiceCore)
	if err != nil {
		t.Fatalf("cached Resolve(core) after both files gone: %v", err)
	}
	apps2, err := c.Resolve(ServiceApps)
	if err != nil {
		t.Fatalf("cached Resolve(apps) after both files gone: %v", err)
	}
	if core1 != core2 || apps1 != apps2 {
		t.Fatalf("per-service cache leaked: core(%q→%q), apps(%q→%q)", core1, core2, apps1, apps2)
	}
}

// GetFrom(ctx, ServiceApps, ...) round-trips a request to
// app-management — mirrors the existing TestGet_RoundTripsBodyFromCore
// for the new upstream. Locks the contract: same shape, different
// .url file resolution.
func TestGetFrom_AppsRoundTripsBody(t *testing.T) {
	want := `{"apps":[{"id":"plex","status":"running"}]}`
	appsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/app_management/compose" {
			t.Errorf("apps received unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(want))
	}))
	defer appsSrv.Close()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, AppsURLFile), []byte(appsSrv.URL), 0o600); err != nil {
		t.Fatalf("write apps url file: %v", err)
	}

	got, err := NewClient(dir, appsSrv.Client()).GetFrom(context.Background(), ServiceApps, "/v2/app_management/compose", "")
	if err != nil {
		t.Fatalf("GetFrom(apps): %v", err)
	}
	if string(got) != want {
		t.Fatalf("got body %q; want %q", got, want)
	}
}

// app-management returning a 5xx must surface as apps_status_<N>,
// distinct from apps_unavailable — an agent (and an operator
// reading the audit trail) can tell "app-management said no" apart
// from "app-management is offline." Pins the existing pattern from
// TestGet_NonOKStatusSurfacesAsTypedError for the new upstream.
func TestGetFrom_AppsNonOKStatusSurfaces(t *testing.T) {
	appsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"msg":"compose engine crashed"}`))
	}))
	defer appsSrv.Close()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, AppsURLFile), []byte(appsSrv.URL), 0o600); err != nil {
		t.Fatalf("write apps url: %v", err)
	}
	_, err := NewClient(dir, appsSrv.Client()).GetFrom(context.Background(), ServiceApps, "/v2/app_management/compose", "")
	pe, ok := err.(*Error)
	if !ok {
		t.Fatalf("non-2xx returned %T; want *Error", err)
	}
	if pe.Code != "apps_status_500" {
		t.Fatalf("Code=%q; want apps_status_500", pe.Code)
	}
	if !strings.Contains(pe.Body, "compose engine crashed") {
		t.Fatalf("Body=%q; want the upstream payload to be preserved", pe.Body)
	}
}
