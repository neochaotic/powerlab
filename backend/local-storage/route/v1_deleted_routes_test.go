//go:build linux

package route_test

// Regression locks for the Sprint 3 cloud-drive removal (#101 / #143).
//
// Build-tagged linux because local-storage transitively depends on fuse +
// mergerfs + udev (syscall.AF_NETLINK, syscall.Setxattr, …) which fail to
// compile on macOS. CI Linux runner covers this.
//
// CasaOS shipped /v1/recover/:type as the OAuth callback for cloud-drive
// recovery (Dropbox / Google Drive / OneDrive), backed by the CasaOS-team-
// hosted OAuth proxy at `cloudoauth.files.casaos.app`. Keeping it would
// have tethered PowerLab to CasaOS infra forever. Sprint 3 Phase 3 deleted
// the route + the underlying `backend/local-storage/service/cloud_storage.go`
// surface.
//
// These tests pin the deletion at the HTTP surface: a future "let's bring
// back cloud drives" refactor would now red-fail CI before re-introducing
// the upstream dependency. Issue #150 Phase 2.

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/neochaotic/powerlab/backend/local-storage/route"
)

func TestDeletedCloudDriveRoutes_return404(t *testing.T) {
	h := route.InitV1Router()

	cases := []struct {
		name   string
		method string
		path   string
	}{
		// Cloud-drive recovery OAuth callback (CasaOS upstream proxy)
		{"recover dropbox", http.MethodGet, "/v1/recover/dropbox"},
		{"recover gdrive", http.MethodGet, "/v1/recover/gdrive"},
		{"recover onedrive", http.MethodGet, "/v1/recover/onedrive"},
		// Driver listing endpoint that the cloud-drive surface fed
		{"driver list GET", http.MethodGet, "/v1/driver"},
		{"driver list POST", http.MethodPost, "/v1/driver"},
		// Generic cloud namespace (defensive — confirm nothing slips back)
		{"cloud root GET", http.MethodGet, "/v1/cloud"},
		{"cloud list GET", http.MethodGet, "/v1/cloud/list"},
		{"cloud delete", http.MethodDelete, "/v1/cloud/foo"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			// /v1/* is JWT-gated with a 127.0.0.1 skipper. Force localhost
			// so the test asserts route presence (404), not auth (401).
			req.RemoteAddr = "127.0.0.1:1234"
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Errorf("expected 404 for %s %s, got %d (body=%q)",
					tc.method, tc.path, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestSurvivingV1Routes_stillRegistered(t *testing.T) {
	// Sibling test: confirm we didn't over-delete. The /v1/disks +
	// /v1/storage + /v1/usb groups MUST still respond (even if with
	// 5xx from the underlying handler hitting an empty test env) —
	// the point is that they're not 404.
	h := route.InitV1Router()

	survivors := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/v1/disks"},
		{http.MethodGet, "/v1/storage"},
		{http.MethodGet, "/v1/usb/usb-auto-mount"},
	}

	for _, s := range survivors {
		req := httptest.NewRequest(s.method, s.path, nil)
		req.RemoteAddr = "127.0.0.1:1234"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code == http.StatusNotFound {
			t.Errorf("expected %s %s to be registered (any non-404 status), got 404", s.method, s.path)
		}
	}
}
