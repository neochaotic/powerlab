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

// The happy path: core's /v1/sys/utilization handler returns a body,
// MCP's system://utilization round-trips it byte-for-byte. Proves the
// proxy pattern is wired end-to-end through the SDK + resource layer
// + coreproxy (not just the coreproxy unit tests).
func TestSystemUtilization_RoundTripsCorePayload(t *testing.T) {
	want := `{"cpu":{"percent":[12.3,8.7],"temperature":48.5,"num":4,"power":3.2,"model":"intel"},"mem":{"total":16384,"used":8192}}`
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/sys/utilization" {
			t.Errorf("core received unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(want))
	}))
	defer core.Close()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, coreproxy.CoreURLFile), []byte(core.URL), 0o600); err != nil {
		t.Fatalf("write .url: %v", err)
	}
	rc := resourcesConfig{
		procRoot:   t.TempDir(),
		coreClient: coreproxy.NewClient(dir, core.Client()),
	}
	srv := newMCPServer(BuildInfo{Version: "test"}, rc, fixtureJournalRunner(""))
	cs := connectInProcess(t, srv)
	defer cs.Close()

	res, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: systemUtilizationURI})
	if err != nil {
		t.Fatalf("ReadResource(system://utilization): %v", err)
	}
	if res.Contents[0].Text != want {
		t.Fatalf("got payload %q; want core's body verbatim %q", res.Contents[0].Text, want)
	}
}

// When core is unreachable the resource must NOT error at the MCP
// layer (the agent receives a real payload) — it returns the
// structured core_unavailable shape with a fallback hint. The agent
// pattern-matches on `error == "core_unavailable"` and pivots to
// audit:// + journal:// reads.
func TestSystemUtilization_CoreUnavailableServesStructuredError(t *testing.T) {
	// Point at a port nothing's listening on.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, coreproxy.CoreURLFile), []byte("http://127.0.0.1:1"), 0o600); err != nil {
		t.Fatalf("write .url: %v", err)
	}
	rc := resourcesConfig{
		procRoot:   t.TempDir(),
		coreClient: coreproxy.NewClient(dir, &http.Client{}),
	}
	srv := newMCPServer(BuildInfo{Version: "test"}, rc, fixtureJournalRunner(""))
	cs := connectInProcess(t, srv)
	defer cs.Close()

	res, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: systemUtilizationURI})
	if err != nil {
		t.Fatalf("ReadResource(system://utilization) errored at MCP layer: %v (want the structured payload instead)", err)
	}
	var got struct {
		Error    string `json:"error"`
		Detail   string `json:"detail"`
		Fallback string `json:"fallback"`
	}
	if uerr := json.Unmarshal([]byte(res.Contents[0].Text), &got); uerr != nil {
		t.Fatalf("payload not valid JSON: %v\n%s", uerr, res.Contents[0].Text)
	}
	if got.Error != "core_unavailable" {
		t.Fatalf("error=%q; want core_unavailable", got.Error)
	}
	if !strings.Contains(got.Fallback, "audit") || !strings.Contains(got.Fallback, "journal") {
		t.Fatalf("fallback=%q; want it to mention audit + journal so the agent can pivot", got.Fallback)
	}
}

// A server constructed without a coreproxy (e.g. an early-boot
// minimal test) must NOT crash on system://utilization — it returns
// the same structured error as the core-down path, with a different
// detail message so an operator can see what's actually wrong.
func TestSystemUtilization_NilProxyServesStructuredError(t *testing.T) {
	rc := resourcesConfig{procRoot: t.TempDir()} // coreClient: nil
	srv := newMCPServer(BuildInfo{Version: "test"}, rc, fixtureJournalRunner(""))
	cs := connectInProcess(t, srv)
	defer cs.Close()

	res, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: systemUtilizationURI})
	if err != nil {
		t.Fatalf("read on nil-proxy server errored: %v", err)
	}
	if !strings.Contains(res.Contents[0].Text, "core_unavailable") {
		t.Fatalf("payload %q; want it to indicate core_unavailable", res.Contents[0].Text)
	}
}
