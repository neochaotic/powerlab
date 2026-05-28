package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/neochaotic/powerlab/backend/powerlab-mcp/coreproxy"
)

// tools/list must advertise restart_app. ADR-0046 requires every
// shipped tool to be discoverable from tools/list — silent
// registration would mean the agent never thinks to call it.
func TestRestartApp_IsAdvertised(t *testing.T) {
	srv := buildAppsTestServer(t, "http://unused", nil)
	cs := connectInProcess(t, srv)
	defer cs.Close()
	list, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	for _, tool := range list.Tools {
		if tool.Name == "restart_app" {
			if !strings.Contains(tool.Description, "SIDE EFFECT") {
				t.Fatalf("restart_app description missing SIDE EFFECT class: %q", tool.Description)
			}
			return
		}
	}
	t.Fatalf("tools/list missing restart_app")
}

// Happy path: tool calls PUT /v2/app_management/compose/{id}/status
// with body '"restart"' against app-management. Locks the route +
// verb + body shape so a refactor can't silently break the contract
// app-management expects.
func TestRestartApp_PUTsCorrectShapeToAppManagement(t *testing.T) {
	var gotMethod, gotPath, gotContentType string
	var gotBody []byte
	appsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"restart requested"}`))
	}))
	defer appsSrv.Close()

	srv := buildAppsTestServer(t, appsSrv.URL, appsSrv.Client())
	cs := connectInProcess(t, srv)
	defer cs.Close()

	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "restart_app",
		Arguments: map[string]any{"id": "plex"},
	})
	if err != nil {
		t.Fatalf("CallTool(restart_app): %v", err)
	}
	if res.IsError {
		t.Fatalf("CallTool errored: %+v", res.Content)
	}

	if gotMethod != http.MethodPut {
		t.Fatalf("upstream got %s; want PUT", gotMethod)
	}
	if gotPath != "/v2/app_management/compose/plex/status" {
		t.Fatalf("upstream path %q; want /v2/app_management/compose/plex/status", gotPath)
	}
	if gotContentType != "application/json" {
		t.Fatalf("upstream Content-Type %q; want application/json", gotContentType)
	}
	if string(gotBody) != `"restart"` {
		t.Fatalf("upstream body %q; want JSON-encoded enum '\"restart\"' (app-management spec)", gotBody)
	}

	// Verify the typed output the agent reads.
	var got RestartAppOutput
	b, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != "plex" {
		t.Fatalf("output.id=%q; want plex", got.ID)
	}
	if got.Status == "" {
		t.Fatalf("output.status empty; want a human-readable confirmation")
	}
}

// app-management returning 404 (compose app not found) surfaces as
// IsError=true + the structured apps_status_404 payload so the agent
// can pattern-match and tell the user "no such app." NOT a Go-side
// error — the SDK delivers a real CallToolResult.
func TestRestartApp_UnknownAppSurfacesAsAppsStatus404(t *testing.T) {
	appsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"compose app not found"}`))
	}))
	defer appsSrv.Close()

	srv := buildAppsTestServer(t, appsSrv.URL, appsSrv.Client())
	cs := connectInProcess(t, srv)
	defer cs.Close()

	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "restart_app",
		Arguments: map[string]any{"id": "not-a-real-app"},
	})
	if err != nil {
		t.Fatalf("CallTool errored at MCP layer (want IsError result instead): %v", err)
	}
	if !res.IsError {
		t.Fatalf("got IsError=false on a 404 upstream; want a structured error")
	}
	// The payload body should be the canonical apps_status_404 shape
	// with the upstream message preserved.
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			if strings.Contains(tc.Text, "apps_status_404") && strings.Contains(tc.Text, "compose app not found") {
				return
			}
		}
	}
	t.Fatalf("response did not include the canonical apps_status_404 + upstream body: %+v", res.Content)
}

// App-management down → tool returns IsError=true + structured
// apps_unavailable. Mirrors the apps:// resource degradation pattern
// so the agent's failure handler doesn't need a tool-specific code
// path.
func TestRestartApp_AppManagementDownReturnsStructuredError(t *testing.T) {
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

	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "restart_app",
		Arguments: map[string]any{"id": "plex"},
	})
	if err != nil {
		t.Fatalf("CallTool errored: %v", err)
	}
	if !res.IsError {
		t.Fatalf("got IsError=false with app-management down")
	}
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			if strings.Contains(tc.Text, "apps_unavailable") {
				return
			}
		}
	}
	t.Fatalf("response did not surface apps_unavailable: %+v", res.Content)
}

// Validation pins: empty id and path-traversal-shaped ids are
// rejected at the tool layer before the proxy gets the call.
// Prevents a typo or malicious agent from probing unexpected
// app-management endpoints via a crafted id.
func TestRestartApp_RejectsInvalidIDs(t *testing.T) {
	srv := buildAppsTestServer(t, "http://unused", nil)
	cs := connectInProcess(t, srv)
	defer cs.Close()

	for _, id := range []string{"", "../etc/passwd", "foo/bar", "with.dot", "with?query"} {
		args := map[string]any{}
		if id != "" {
			args["id"] = id
		}
		res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
			Name:      "restart_app",
			Arguments: args,
		})
		if err == nil && !res.IsError {
			t.Fatalf("invalid id %q succeeded; want validation error", id)
		}
	}
}
