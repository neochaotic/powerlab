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

// THE most important gate test: when EnableDestructiveTools is false
// the destructive tools MUST NOT appear in tools/list. An agent that
// reads tools/list and doesn't see install_app/uninstall_app has no
// way to call them. This is the operator opt-in surface contract.
func TestDestructiveTools_NotAdvertisedWhenFlagFalse(t *testing.T) {
	srv := newMCPServer(BuildInfo{Version: "test"},
		resourcesConfig{procRoot: t.TempDir(), enableDestructiveTools: false},
		fixtureJournalRunner(""))
	cs := connectInProcess(t, srv)
	defer cs.Close()

	list, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	for _, tool := range list.Tools {
		if tool.Name == "install_app" || tool.Name == "uninstall_app" {
			t.Fatalf("destructive tool %q advertised when EnableDestructiveTools=false (gate broken!)", tool.Name)
		}
	}
}

// And when the flag IS true, the tools DO advertise — with the
// SIDE EFFECT / DESTRUCTIVE marker in the description so the LLM
// surfaces it.
func TestDestructiveTools_AdvertisedWhenFlagTrue(t *testing.T) {
	srv := newMCPServer(BuildInfo{Version: "test"},
		resourcesConfig{procRoot: t.TempDir(), enableDestructiveTools: true},
		fixtureJournalRunner(""))
	cs := connectInProcess(t, srv)
	defer cs.Close()

	list, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	gotNames := map[string]string{}
	for _, tool := range list.Tools {
		gotNames[tool.Name] = tool.Description
	}
	for _, want := range []string{"install_app", "uninstall_app"} {
		desc, ok := gotNames[want]
		if !ok {
			t.Fatalf("EnableDestructiveTools=true but %q missing from tools/list", want)
		}
		if !strings.Contains(desc, "DESTRUCTIVE") {
			t.Fatalf("%q description missing DESTRUCTIVE class: %q", want, desc)
		}
	}
}

// install_app runs composevalidator BEFORE app-management ever sees
// the YAML. A privileged: true compose must be rejected at the tool
// layer with the structured violations payload — the upstream is
// never called. This is the core ADR-0046 §4 contract: layered defence.
func TestInstallApp_ValidatorRejectsBeforeReachingUpstream(t *testing.T) {
	var upstreamCalls int
	appsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamCalls++
		_, _ = w.Write([]byte(`{"installed":true}`))
	}))
	defer appsSrv.Close()

	srv := buildDestructiveTestServer(t, appsSrv.URL, appsSrv.Client(), true)
	cs := connectInProcess(t, srv)
	defer cs.Close()

	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "install_app",
		Arguments: map[string]any{
			"compose_yaml": "services:\n  evil:\n    image: x\n    privileged: true\n",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatalf("rejected YAML returned IsError=false; want a structured rejection")
	}
	if upstreamCalls != 0 {
		t.Fatalf("upstream received %d call(s) on a validator-rejected YAML; want 0", upstreamCalls)
	}
	// Confirm the payload carries the violations list so the agent
	// can explain WHY it was rejected.
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			if strings.Contains(tc.Text, "privileged_true") && strings.Contains(tc.Text, "rejected_by_validator") {
				return
			}
		}
	}
	t.Fatalf("rejection payload missing violations: %+v", res.Content)
}

// Valid compose passes validator → POST to app-management with
// Content-Type application/yaml (per the upstream OpenAPI spec).
// Wire shape locked so a refactor can't break the upstream contract.
func TestInstallApp_ValidYAMLPostsToAppManagement(t *testing.T) {
	var gotMethod, gotPath, gotContentType string
	var gotBody []byte
	appsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path + "?" + r.URL.RawQuery
		gotContentType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"id":"plex","status":"installed"}`))
	}))
	defer appsSrv.Close()

	srv := buildDestructiveTestServer(t, appsSrv.URL, appsSrv.Client(), true)
	cs := connectInProcess(t, srv)
	defer cs.Close()

	yamlBody := `services:
  plex:
    image: lscr.io/linuxserver/plex:latest
    volumes:
      - /DATA/PowerLabAppData/plex/config:/config
`
	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "install_app",
		Arguments: map[string]any{
			"compose_yaml": yamlBody,
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("valid YAML errored: %+v", res.Content)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("upstream got %s; want POST", gotMethod)
	}
	if !strings.HasPrefix(gotPath, "/v2/app_management/compose") {
		t.Fatalf("upstream path %q; want /v2/app_management/compose", gotPath)
	}
	if gotContentType != "application/yaml" {
		t.Fatalf("upstream Content-Type %q; want application/yaml (per spec)", gotContentType)
	}
	if string(gotBody) != yamlBody {
		t.Fatalf("upstream body not forwarded verbatim:\nwant: %q\ngot:  %q", yamlBody, gotBody)
	}
}

// Dry-run flag appends ?dry_run=true to the upstream URL, so the
// upstream's own validation runs without actually installing. Combined
// with composevalidator the agent gets a full pre-flight answer.
func TestInstallApp_DryRunForwardsQueryParam(t *testing.T) {
	var gotURL string
	appsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		_, _ = w.Write([]byte(`{"dry":"ok"}`))
	}))
	defer appsSrv.Close()

	srv := buildDestructiveTestServer(t, appsSrv.URL, appsSrv.Client(), true)
	cs := connectInProcess(t, srv)
	defer cs.Close()

	_, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "install_app",
		Arguments: map[string]any{
			"compose_yaml": "services:\n  ok:\n    image: x\n",
			"dry_run":      true,
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !strings.Contains(gotURL, "dry_run=true") {
		t.Fatalf("upstream URL %q missing dry_run=true", gotURL)
	}
}

// uninstall_app sends a DELETE to /v2/app_management/compose/{id}.
// Pins the wire shape so a refactor can't silently break the
// upstream contract or accidentally upgrade to a stronger verb.
func TestUninstallApp_DELETEsToAppManagement(t *testing.T) {
	var gotMethod, gotPath string
	appsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"removed":true}`))
	}))
	defer appsSrv.Close()

	srv := buildDestructiveTestServer(t, appsSrv.URL, appsSrv.Client(), true)
	cs := connectInProcess(t, srv)
	defer cs.Close()

	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "uninstall_app",
		Arguments: map[string]any{"id": "plex"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("uninstall errored: %+v", res.Content)
	}
	if gotMethod != http.MethodDelete {
		t.Fatalf("upstream got %s; want DELETE", gotMethod)
	}
	if gotPath != "/v2/app_management/compose/plex" {
		t.Fatalf("upstream path %q; want /v2/app_management/compose/plex", gotPath)
	}

	// Decode the typed output.
	var got UninstallAppOutput
	b, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != "plex" {
		t.Fatalf("output.id=%q; want plex", got.ID)
	}
}

// Both destructive tools reject empty / path-traversal-shaped /
// dotted / query-shaped ids at the tool layer. Mirrors the
// restart_app validation discipline so an agent can't escape with
// a crafted id.
func TestDestructiveTools_RejectInvalidIDs(t *testing.T) {
	srv := buildDestructiveTestServer(t, "http://unused", nil, true)
	cs := connectInProcess(t, srv)
	defer cs.Close()

	for _, tool := range []string{"uninstall_app"} {
		for _, id := range []string{"", "../etc/passwd", "foo/bar", "with.dot", "with?query"} {
			args := map[string]any{}
			if id != "" {
				args["id"] = id
			}
			res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
				Name:      tool,
				Arguments: args,
			})
			if err == nil && !res.IsError {
				t.Fatalf("%s with invalid id %q succeeded; want validation error", tool, id)
			}
		}
	}
}

// install_app requires a non-empty compose_yaml. Empty input is a
// validation error, not a passthrough.
func TestInstallApp_RequiresComposeYAML(t *testing.T) {
	srv := buildDestructiveTestServer(t, "http://unused", nil, true)
	cs := connectInProcess(t, srv)
	defer cs.Close()

	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "install_app",
		Arguments: map[string]any{},
	})
	if err == nil && !res.IsError {
		t.Fatalf("install_app with empty compose_yaml succeeded; want validation error")
	}
}

// buildDestructiveTestServer is a focused helper that spins up an
// MCP server with the destructive-tools flag set explicitly and a
// coreproxy.Client pointed at the test upstream.
func buildDestructiveTestServer(t *testing.T, upstreamURL string, httpClient *http.Client, enable bool) *mcp.Server {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, coreproxy.AppsURLFile), []byte(upstreamURL), 0o600); err != nil {
		t.Fatalf("write apps url: %v", err)
	}
	rc := resourcesConfig{
		procRoot:               t.TempDir(),
		coreClient:             coreproxy.NewClient(dir, httpClient),
		enableDestructiveTools: enable,
	}
	return newMCPServer(BuildInfo{Version: "test"}, rc, fixtureJournalRunner(""))
}
