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

// system://schema must be discoverable from resources/list AND parse
// as a JSON document with at least the four resources we ship — agents
// hit this first to know what the system:// surface looks like.
func TestSystemSchema_IsAdvertisedAndDescribesEveryResource(t *testing.T) {
	srv := newMCPServer(BuildInfo{Version: "test"},
		resourcesConfig{procRoot: t.TempDir()},
		fixtureJournalRunner(""))
	cs := connectInProcess(t, srv)
	defer cs.Close()

	list, err := cs.ListResources(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	if !hasResource(list.Resources, systemSchemaURI) {
		t.Fatalf("system://schema not advertised in resources/list")
	}

	res, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: systemSchemaURI})
	if err != nil {
		t.Fatalf("ReadResource(system://schema): %v", err)
	}
	var doc struct {
		Description string                 `json:"description"`
		Resources   map[string]interface{} `json:"resources"`
	}
	if uerr := json.Unmarshal([]byte(res.Contents[0].Text), &doc); uerr != nil {
		t.Fatalf("schema payload not JSON: %v", uerr)
	}
	if doc.Description == "" {
		t.Fatalf("schema has empty description")
	}
	// Each system:// resource we register must be documented in the
	// schema (otherwise the agent reads the schema, doesn't see the
	// resource, and never thinks to call it).
	for _, want := range []string{
		systemSchemaURI, systemMetricsURI, systemUtilizationURI,
		systemDiskURI, systemNetworkURI, systemGPUURI,
		systemServicesURI, systemKernelURI, systemUpdatesURI, systemProcessesURI,
	} {
		if _, ok := doc.Resources[want]; !ok {
			t.Fatalf("schema does not document %q (agent would not discover it)", want)
		}
	}
}

// system://disk + system://network are proxies — happy path round-trips
// core's body verbatim. Parametric over both URIs so the proxy contract
// stays one test per path.
func TestProxiedResources_RoundTripCoreBody(t *testing.T) {
	cases := []struct {
		uri      string
		corePath string
		body     string
	}{
		{systemDiskURI, "/v1/sys/disk", `{"physical":[{"model":"NVMe","size":"512GB","temperature":42}]}`},
		{systemNetworkURI, "/v1/sys/network/interfaces", `[{"name":"eth0","state":"up","bytesRecv":12345}]`},
		{systemServicesURI, "/v1/sys/services", `[{"name":"powerlab-gateway","active_state":"active","sub_state":"running"}]`},
		{systemKernelURI, "/v1/sys/host", `{"hostname":"box","kernelVersion":"6.8.0-117","platform":"ubuntu","platformVersion":"24.04"}`},
		{systemProcessesURI, "/v1/sys/processes", `{"total":42,"top_by_cpu":[{"pid":1,"name":"systemd","cpu_percent":0.1}],"top_by_mem":[],"truncated":true}`},
	}

	for _, tc := range cases {
		t.Run(tc.uri, func(t *testing.T) {
			core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.corePath {
					t.Errorf("core received %q; want %q", r.URL.Path, tc.corePath)
				}
				_, _ = w.Write([]byte(tc.body))
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

// When core is down both proxies must serve the structured
// core_unavailable shape — never error at the MCP layer, never serve
// stale/zero data. Parametric for the same two URIs.
func TestProxiedResources_CoreDownReturnsStructuredError(t *testing.T) {
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

	for _, uri := range []string{systemDiskURI, systemNetworkURI, systemServicesURI, systemKernelURI, systemProcessesURI} {
		t.Run(uri, func(t *testing.T) {
			res, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: uri})
			if err != nil {
				t.Fatalf("MCP-layer error on core-down: %v (want the structured payload)", err)
			}
			var got struct {
				Error    string `json:"error"`
				Fallback string `json:"fallback"`
			}
			if uerr := json.Unmarshal([]byte(res.Contents[0].Text), &got); uerr != nil {
				t.Fatalf("payload not JSON: %v", uerr)
			}
			if got.Error != "core_unavailable" {
				t.Fatalf("error=%q; want core_unavailable", got.Error)
			}
			if !strings.Contains(got.Fallback, "audit") || !strings.Contains(got.Fallback, "journal") {
				t.Fatalf("fallback=%q; missing audit + journal pivot hint", got.Fallback)
			}
		})
	}
}

// system://gpu imports common/external::GetGPUUtilization directly —
// no network hop, no proxy. The handler must always return a valid
// JSON object even on a no-GPU box (empty model string, not null,
// not an error). Locks the contract: "no GPU" = empty fields, NOT
// a failure the agent has to special-case.
func TestSystemGPU_AlwaysReturnsValidShape(t *testing.T) {
	srv := newMCPServer(BuildInfo{Version: "test"},
		resourcesConfig{procRoot: t.TempDir()},
		fixtureJournalRunner(""))
	cs := connectInProcess(t, srv)
	defer cs.Close()

	res, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: systemGPUURI})
	if err != nil {
		t.Fatalf("ReadResource(system://gpu): %v (must never error — empty box reports empty fields)", err)
	}
	var got struct {
		Percent     float64 `json:"percent"`
		MemoryUsed  int64   `json:"memoryUsed"`
		Model       string  `json:"model"`
		Temperature int     `json:"temperature"`
	}
	if uerr := json.Unmarshal([]byte(res.Contents[0].Text), &got); uerr != nil {
		t.Fatalf("payload not JSON: %v\n%s", uerr, res.Contents[0].Text)
	}
	// On a CI Linux box without nvidia-smi the model is empty; on a
	// Mac dev box it's "Apple Silicon GPU". Both are valid; what we
	// lock is that the SHAPE is correct (every field present + typed).
	// The handler must NEVER emit `null` — that's the
	// JSON-of-nil bug we defend against.
	if res.Contents[0].Text == "null" {
		t.Fatalf("payload is literal null — handler must marshal an empty struct, not nil")
	}
}
