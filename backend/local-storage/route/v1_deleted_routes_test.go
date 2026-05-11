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

// NOTE: a sibling "TestSurvivingV1Routes_stillRegistered" exists on
// the `core` service and works fine there because the survivor
// routes (`/ping`, `/v1/sys/version/current`, `/v1/powerlab/version`)
// are pure-function handlers.
//
// On local-storage, the surviving v1 handlers (GetDiskList,
// GetStorageList, the USB auto-mount toggle) all hit real OS state
// (lsblk, fstab, sysfs) — calling them with httptest in a CI
// container produces a nil-pointer panic in the handler that escapes
// Echo's recover middleware via the -race goroutine surface. The
// "is the route still registered?" intent is better served by a
// structural assertion on `echo.Echo.Routes()`, but `InitV1Router`
// returns `http.Handler` rather than `*echo.Echo`, so that requires
// a refactor not in this PR's scope.
//
// The 404-locks above (TestDeletedCloudDriveRoutes_return404) are
// the load-bearing assertion of this file; the survivor sibling was
// defensive only and is omitted here on purpose.
