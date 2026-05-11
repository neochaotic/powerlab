package route_test

// Regression locks for the Sprint 3 cloud-drive removal (#101 / #143)
// at the core service surface.
//
// CasaOS shipped /v1/recover/:type (cloud-drive OAuth callback) and
// /v1/cloud + /v1/driver (cloud storage backends + driver listing).
// All three groups required the CasaOS-team-hosted OAuth proxy at
// `cloudoauth.files.casaos.app`. Sprint 3 Phase 3 deleted the routes
// + `backend/core/drivers/` (#101).
//
// These tests pin the deletion at the HTTP surface: a future "let's
// bring back cloud drives" refactor would red-fail CI before
// re-introducing the upstream dependency. Issue #150 Phase 2.

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/neochaotic/powerlab/backend/core/route"
)

func TestDeletedCloudDriveRoutes_return404(t *testing.T) {
	h := route.InitV1Router()

	cases := []struct {
		name   string
		method string
		path   string
	}{
		// Cloud-drive OAuth recovery callback
		{"recover dropbox", http.MethodGet, "/v1/recover/dropbox"},
		{"recover gdrive", http.MethodGet, "/v1/recover/gdrive"},
		{"recover onedrive", http.MethodGet, "/v1/recover/onedrive"},
		// /v1/cloud group (cloud storage backends)
		{"cloud root", http.MethodGet, "/v1/cloud"},
		{"cloud list", http.MethodGet, "/v1/cloud/list"},
		{"cloud post", http.MethodPost, "/v1/cloud"},
		{"cloud delete", http.MethodDelete, "/v1/cloud/dropbox"},
		// /v1/driver group (driver listing endpoint)
		{"driver root", http.MethodGet, "/v1/driver"},
		{"driver list POST", http.MethodPost, "/v1/driver"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			// The /v1/* group has JWT middleware with a localhost
			// skipper. Without forcing 127.0.0.1, every request gets
			// 401 from middleware before route matching, hiding the
			// 404 we actually want to assert.
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

func TestDeletedCasaOSSelfUpdateRoutes_return404(t *testing.T) {
	// /v1/sys/version/check and /v1/sys/update were the CasaOS self-update
	// path: poll api.casaos.io for a "new version" string, then
	// `curl … | bash` the get.casaos.io/update installer. Removed for
	// security (curl-pipe-bash from upstream infra) + because PowerLab
	// has its own in-app updater via manifest.json. Lock those
	// deletions too — same regression class.
	h := route.InitV1Router()

	for _, p := range []string{
		"/v1/sys/version/check",
		"/v1/sys/update",
	} {
		t.Run(p, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, p, nil)
			req.RemoteAddr = "127.0.0.1:1234"
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Errorf("expected 404 for %s, got %d (body=%q)", p, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestSurvivingV1Routes_stillRegistered(t *testing.T) {
	// Sibling test: confirm we didn't over-delete. The unauthenticated
	// version handshake + ping + version/current MUST still respond
	// (200) — they're the smoke-test surface the UI calls on boot.
	h := route.InitV1Router()

	survivors := []struct {
		method     string
		path       string
		expectCode int
	}{
		{http.MethodGet, "/ping", http.StatusOK},
		{http.MethodGet, "/v1/sys/version/current", http.StatusOK},
		{http.MethodGet, "/v1/powerlab/version", http.StatusOK},
	}

	for _, s := range survivors {
		t.Run(s.path, func(t *testing.T) {
			req := httptest.NewRequest(s.method, s.path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != s.expectCode {
				t.Errorf("expected %d for %s %s, got %d (body=%q)",
					s.expectCode, s.method, s.path, rec.Code, rec.Body.String())
			}
		})
	}
}
