package route

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neochaotic/powerlab/backend/common/utils/audit"
	"github.com/neochaotic/powerlab/backend/gateway/service"
)

// Regression suite for the bug class found in Sprint 16 pre-tag
// smoke (ADR-0035 motivation): audit endpoints existed only on the
// management Echo group, not the public stdlib mux. Browser hits
// to /v1/audit/recent fell through the catch-all and returned
// `index.html` (HTML), making the AuditPane non-functional.
//
// Each test below pins a contract that the original Sprint 16
// shipping with SQLite would have failed.

// makeGatewayWithAudit builds a GatewayRoute with a real audit
// Service (file-backed) and returns its public mux + cleanup.
func makeGatewayWithAudit(t *testing.T) (http.Handler, *audit.Service) {
	t.Helper()
	tmpdir, err := os.MkdirTemp("", "gateway-audit-test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpdir) })

	state := service.NewState()
	if err := state.SetRuntimePath(tmpdir); err != nil {
		t.Fatal(err)
	}
	mgmt := service.NewManagementService(state)

	auditSvc, err := audit.NewService(audit.ServiceOptions{
		Path: filepath.Join(tmpdir, "audit.jsonl"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = auditSvc.Close() })

	gw := NewGatewayRoute(mgmt, nil, state, auditSvc)
	return gw.GetRoute(), auditSvc
}

func TestGatewayPublicMux_AuditRecentReturnsJSON(t *testing.T) {
	mux, _ := makeGatewayWithAudit(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/audit/recent")
	if err != nil {
		t.Fatalf("GET /v1/audit/recent: %v", err)
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("BUG: /v1/audit/recent on public mux returned %q (likely HTML fallback). Original Sprint 16 bug — endpoint not mounted on public mux.", ct)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status: %d, want 200 (loopback skip applies)", resp.StatusCode)
	}
}

func TestGatewayPublicMux_AuditStatsReturnsJSON(t *testing.T) {
	mux, _ := makeGatewayWithAudit(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/audit/stats")
	if err != nil {
		t.Fatalf("GET /v1/audit/stats: %v", err)
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("BUG: /v1/audit/stats returned %q (likely HTML)", ct)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status: %d, want 200 (loopback skip applies)", resp.StatusCode)
	}
}

func TestGatewayPublicMux_AuditMiddleware_CapturesPublicTraffic(t *testing.T) {
	mux, auditSvc := makeGatewayWithAudit(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Hit an arbitrary non-skipped path — falls through to the
	// catch-all (404, no proxy registered in test). The audit
	// middleware should still capture the request.
	_, err := http.Get(srv.URL + "/some/api/call")
	if err != nil {
		t.Fatal(err)
	}

	waitForRecord(t, auditSvc)

	recent := auditSvc.Store.Recent(audit.RecentOptions{Limit: 10})
	found := false
	for _, r := range recent {
		if r.Path == "/some/api/call" {
			found = true
			if r.Status != 404 {
				t.Errorf("captured status: %d, want 404", r.Status)
			}
			break
		}
	}
	if !found {
		t.Errorf("BUG: /some/api/call not captured by audit middleware on public mux. Audit log silently empty for user traffic was original Sprint 16 bug class.")
	}
}

func TestGatewayPublicMux_AuditMiddleware_SkipsAuditEndpointsThemselves(t *testing.T) {
	// Sanity: hitting /v1/audit/* shouldn't recursively spam the
	// audit log with audit-pane-poll noise. Should be skipped by
	// the middleware's skipper.
	mux, auditSvc := makeGatewayWithAudit(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	for i := 0; i < 5; i++ {
		_, _ = http.Get(srv.URL + "/v1/audit/recent")
	}
	waitForRecord(t, auditSvc)

	recent := auditSvc.Store.Recent(audit.RecentOptions{Limit: 100})
	for _, r := range recent {
		if strings.HasPrefix(r.Path, "/v1/audit/") {
			t.Errorf("audit endpoints should be skipped, but found: %s", r.Path)
		}
	}
}

// waitForRecord blocks until at least one record lands in the ring
// or 1s elapses.
func waitForRecord(t *testing.T, svc *audit.Service) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(svc.Store.Recent(audit.RecentOptions{Limit: 1})) > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}
