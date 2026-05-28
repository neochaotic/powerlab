package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/neochaotic/powerlab/backend/powerlab-mcp/coreproxy"
)

// apps://list happy path: MCP proxies /v2/app_management/compose and
// returns the body verbatim. Pins the contract for the simplest
// concrete-URI apps:// resource.
func TestAppsList_RoundTripsAppManagementBody(t *testing.T) {
	want := `{"apps":[{"id":"plex","name":"Plex","status":"running"},{"id":"jellyfin","name":"Jellyfin","status":"stopped"}]}`
	appsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/app_management/compose" {
			t.Errorf("app-management received %q; want /v2/app_management/compose", r.URL.Path)
		}
		_, _ = w.Write([]byte(want))
	}))
	defer appsSrv.Close()

	srv := buildAppsTestServer(t, appsSrv.URL, appsSrv.Client())
	cs := connectInProcess(t, srv)
	defer cs.Close()

	res, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: appsListURI})
	if err != nil {
		t.Fatalf("ReadResource(apps://list): %v", err)
	}
	if res.Contents[0].Text != want {
		t.Fatalf("got payload %q; want %q (verbatim)", res.Contents[0].Text, want)
	}
}

// apps://state/{id} dispatches to /v2/app_management/compose/{id}.
// Confirms the URI parser extracts the id correctly and the proxy
// routes to the per-app detail endpoint.
func TestAppsState_RoutesToPerAppEndpoint(t *testing.T) {
	var gotPath string
	appsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"id":"plex","state":"running"}`))
	}))
	defer appsSrv.Close()

	srv := buildAppsTestServer(t, appsSrv.URL, appsSrv.Client())
	cs := connectInProcess(t, srv)
	defer cs.Close()

	_, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: "apps://state/plex"})
	if err != nil {
		t.Fatalf("ReadResource(apps://state/plex): %v", err)
	}
	if gotPath != "/v2/app_management/compose/plex" {
		t.Fatalf("app-management received %q; want /v2/app_management/compose/plex", gotPath)
	}
}

// apps://state sub-paths (containers/health/stats/disk) route to the
// matching app-management sub-endpoints. Table-driven so the routing
// contract is one test per sub-path.
func TestAppsState_SubPathsRouteCorrectly(t *testing.T) {
	cases := []struct {
		mcpURI       string
		expectedPath string
	}{
		{"apps://state/plex/containers", "/v2/app_management/compose/plex/containers"},
		{"apps://state/plex/health", "/v2/app_management/compose/plex/health"},
		{"apps://state/plex/stats", "/v2/app_management/compose/plex/stats"},
		{"apps://state/plex/disk", "/v2/app_management/compose/plex/disk"},
	}

	for _, tc := range cases {
		t.Run(tc.mcpURI, func(t *testing.T) {
			var gotPath string
			appsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				_, _ = w.Write([]byte(`{}`))
			}))
			defer appsSrv.Close()

			srv := buildAppsTestServer(t, appsSrv.URL, appsSrv.Client())
			cs := connectInProcess(t, srv)
			defer cs.Close()

			_, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: tc.mcpURI})
			if err != nil {
				t.Fatalf("ReadResource(%s): %v", tc.mcpURI, err)
			}
			if gotPath != tc.expectedPath {
				t.Fatalf("got upstream path %q; want %q", gotPath, tc.expectedPath)
			}
		})
	}
}

// docker://logs/{id} routes to /v2/app_management/compose/{id}/logs
// — MCP NEVER touches the Docker socket. This is the ADR-0045 win #2
// pinned as a test: app-management is the only thing in PowerLab
// that talks to Docker; MCP just calls its HTTP API.
func TestDockerLogs_RoutesThroughAppManagement(t *testing.T) {
	var gotPath string
	want := `[{"ts":"2026-05-28T00:00:01Z","stream":"stdout","line":"server started"}]`
	appsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(want))
	}))
	defer appsSrv.Close()

	srv := buildAppsTestServer(t, appsSrv.URL, appsSrv.Client())
	cs := connectInProcess(t, srv)
	defer cs.Close()

	res, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: "docker://logs/plex"})
	if err != nil {
		t.Fatalf("ReadResource(docker://logs/plex): %v", err)
	}
	if gotPath != "/v2/app_management/compose/plex/logs" {
		t.Fatalf("upstream path %q; want /v2/app_management/compose/plex/logs (must go through app-management, NOT direct Docker socket)", gotPath)
	}
	if res.Contents[0].Text != want {
		t.Fatalf("body not round-tripped verbatim")
	}
}

// When app-management is down every apps:// resource must serve the
// canonical apps_unavailable shape — NEVER an error at the MCP layer
// (the agent must always read a real payload + pivot via the
// fallback hint). Parametric for the resources that don't need a
// sub-path id parse.
func TestAppsResources_AppManagementDownReturnsStructuredError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, coreproxy.AppsURLFile), []byte("http://127.0.0.1:1"), 0o600); err != nil {
		t.Fatalf("write apps url: %v", err)
	}
	rc := resourcesConfig{
		procRoot:   t.TempDir(),
		coreClient: coreproxy.NewClient(dir, &http.Client{}),
	}
	srv := newMCPServer(BuildInfo{Version: "test"}, rc, fixtureJournalRunner(""))
	cs := connectInProcess(t, srv)
	defer cs.Close()

	for _, uri := range []string{appsListURI, "apps://state/plex", "docker://logs/plex"} {
		t.Run(uri, func(t *testing.T) {
			res, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: uri})
			if err != nil {
				t.Fatalf("MCP-layer error on app-management-down: %v", err)
			}
			var got struct {
				Error    string `json:"error"`
				Fallback string `json:"fallback"`
			}
			if uerr := json.Unmarshal([]byte(res.Contents[0].Text), &got); uerr != nil {
				t.Fatalf("payload not JSON: %v\n%s", uerr, res.Contents[0].Text)
			}
			if got.Error != "apps_unavailable" {
				t.Fatalf("error=%q; want apps_unavailable", got.Error)
			}
			if !strings.Contains(got.Fallback, "audit") || !strings.Contains(got.Fallback, "journal") {
				t.Fatalf("fallback=%q; missing audit + journal pivot hint", got.Fallback)
			}
		})
	}
}

// apps://schema is the agent's discovery doc — it MUST document every
// resource registered, otherwise the agent reads the schema and never
// thinks to call the missing one. Locks the registration ↔ schema
// invariant.
func TestAppsSchema_DocumentsEveryRegisteredResource(t *testing.T) {
	srv := buildAppsTestServer(t, "http://unused", nil)
	cs := connectInProcess(t, srv)
	defer cs.Close()

	res, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: appsSchemaURI})
	if err != nil {
		t.Fatalf("ReadResource(apps://schema): %v", err)
	}
	var doc struct {
		Description string                 `json:"description"`
		Resources   map[string]interface{} `json:"resources"`
	}
	if uerr := json.Unmarshal([]byte(res.Contents[0].Text), &doc); uerr != nil {
		t.Fatalf("schema payload not JSON: %v", uerr)
	}
	for _, want := range []string{
		appsSchemaURI, appsListURI,
		appsStateTemplate, appsStateContainersTmpl,
		appsStateHealthTmpl, appsStateStatsTmpl, appsStateDiskTmpl,
		dockerLogsTemplate,
	} {
		if _, ok := doc.Resources[want]; !ok {
			t.Fatalf("schema does not document %q (agent would not discover it)", want)
		}
	}
}

// buildAppsTestServer spins up an MCP server pointed at the given
// app-management URL (planted as the apps .url file). Reused by every
// happy-path apps test to keep the boilerplate to one place.
func buildAppsTestServer(t *testing.T, appsURL string, httpClient *http.Client) *mcp.Server {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, coreproxy.AppsURLFile), []byte(appsURL), 0o600); err != nil {
		t.Fatalf("write apps url: %v", err)
	}
	rc := resourcesConfig{
		procRoot:   t.TempDir(),
		coreClient: coreproxy.NewClient(dir, httpClient),
	}
	return newMCPServer(BuildInfo{Version: "test"}, rc, fixtureJournalRunner(""))
}
