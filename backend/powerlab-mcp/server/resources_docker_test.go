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

// docker://{containers,images,networks,volumes,system} all proxy to
// app-management's /v2/app_management/docker/* family of endpoints (#630).
// Per ADR-0045, MCP NEVER touches the Docker socket — it goes through
// app-management's HTTP API, which is the single PowerLab service that
// talks to the daemon. The same proxy pattern as apps://list.
func TestDockerRawVisibility_RoundTripsAppManagementBody(t *testing.T) {
	cases := []struct {
		uri          string
		expectedPath string
		body         string
	}{
		{dockerContainersURI, "/v2/app_management/docker/containers", `{"containers":[{"name":"plex","image":"plex/plex:latest","state":"running"}]}`},
		{dockerImagesURI, "/v2/app_management/docker/images", `{"images":[{"id":"sha256:abc","tags":["nginx:latest"],"size":12345}]}`},
		{dockerNetworksURI, "/v2/app_management/docker/networks", `{"networks":[{"name":"bridge","driver":"bridge","scope":"local"}]}`},
		{dockerVolumesURI, "/v2/app_management/docker/volumes", `{"volumes":[{"name":"plex_data","driver":"local","mountpoint":"/var/lib/docker/volumes/plex_data/_data"}]}`},
		{dockerSystemURI, "/v2/app_management/docker/system", `{"docker_version":"28.5.1","containers_count":3,"images_count":12,"disk_usage":{"containers":100,"images":2000,"volumes":500,"build_cache":0}}`},
	}
	for _, tc := range cases {
		t.Run(tc.uri, func(t *testing.T) {
			appsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.expectedPath {
					t.Errorf("app-management received %q; want %q", r.URL.Path, tc.expectedPath)
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			defer appsSrv.Close()

			srv := buildAppsTestServer(t, appsSrv.URL, appsSrv.Client())
			cs := connectInProcess(t, srv)
			defer func() { _ = cs.Close() }()

			res, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: tc.uri})
			if err != nil {
				t.Fatalf("ReadResource(%s): %v", tc.uri, err)
			}
			if res.Contents[0].Text != tc.body {
				t.Fatalf("got payload %q; want %q (verbatim)", res.Contents[0].Text, tc.body)
			}
		})
	}
}

// When app-management is down each docker://* resource must serve the
// canonical apps_unavailable shape — never error at the MCP layer.
// Same contract as apps_unavailable from resources_apps_test.go; the
// agent pattern-matches on the same Code prefix to know it should pivot
// to audit + journal.
func TestDockerRawVisibility_AppManagementDownReturnsStructuredError(t *testing.T) {
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
	defer func() { _ = cs.Close() }()

	for _, uri := range []string{
		dockerContainersURI, dockerImagesURI, dockerNetworksURI,
		dockerVolumesURI, dockerSystemURI,
	} {
		t.Run(uri, func(t *testing.T) {
			res, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: uri})
			if err != nil {
				t.Fatalf("MCP-layer error on app-management-down: %v (want structured payload)", err)
			}
			var got struct {
				Error    string `json:"error"`
				Fallback string `json:"fallback"`
			}
			if uerr := json.Unmarshal([]byte(res.Contents[0].Text), &got); uerr != nil {
				t.Fatalf("payload not JSON: %v", uerr)
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

// apps://schema must document the 5 new docker raw-visibility URIs so
// the agent's discovery doc round-trips with every resource the daemon
// registers. Locks the registration ↔ schema invariant for #630.
func TestAppsSchema_DocumentsRawDockerResources(t *testing.T) {
	srv := buildAppsTestServer(t, "http://unused", nil)
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	res, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: appsSchemaURI})
	if err != nil {
		t.Fatalf("ReadResource(apps://schema): %v", err)
	}
	var doc struct {
		Resources map[string]interface{} `json:"resources"`
	}
	if uerr := json.Unmarshal([]byte(res.Contents[0].Text), &doc); uerr != nil {
		t.Fatalf("schema payload not JSON: %v", uerr)
	}
	for _, want := range []string{
		dockerContainersURI, dockerImagesURI, dockerNetworksURI,
		dockerVolumesURI, dockerSystemURI,
	} {
		if _, ok := doc.Resources[want]; !ok {
			t.Fatalf("schema does not document %q (agent would not discover it)", want)
		}
	}
}

// All 5 docker://* raw-visibility URIs must be advertised by
// resources/list so the agent discovers them at session init. Mirrors
// the schema test but exercises the protocol-level advertisement path.
func TestDockerRawVisibility_AdvertisedInResourcesList(t *testing.T) {
	srv := buildAppsTestServer(t, "http://unused", nil)
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	list, err := cs.ListResources(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	for _, want := range []string{
		dockerContainersURI, dockerImagesURI, dockerNetworksURI,
		dockerVolumesURI, dockerSystemURI,
	} {
		if !hasResource(list.Resources, want) {
			t.Fatalf("%s not advertised in resources/list", want)
		}
	}
}
