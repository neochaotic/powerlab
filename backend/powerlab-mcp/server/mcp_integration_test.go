package server

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/neochaotic/powerlab/backend/powerlab-mcp/journal"
	"github.com/neochaotic/powerlab/backend/powerlab-mcp/metrics"
)

func testClient() *mcp.Client {
	return mcp.NewClient(&mcp.Implementation{Name: "powerlab-mcp-test", Version: "0"}, nil)
}

// fixtureJournalRunner returns a journal.Runner that yields canned
// journalctl NDJSON, so journal:// is exercised end-to-end without a
// real journalctl.
func fixtureJournalRunner(out string) journal.Runner {
	return func(_ context.Context, _ []string) ([]byte, error) {
		return []byte(out), nil
	}
}

// writeProcFixtures lays down a deterministic /proc so the system://
// resource serves known values on any OS (the macOS dev box has no
// /proc). used% = (2000-500)/2000 = 75; 2 cores.
func writeProcFixtures(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, dir, "meminfo", "MemTotal:        2000 kB\nMemAvailable:     500 kB\n")
	mustWrite(t, dir, "loadavg", "0.10 0.20 0.30 1/200 42\n")
	mustWrite(t, dir, "uptime", "4242.00 8000.00\n")
	mustWrite(t, dir, "cpuinfo", "processor\t: 0\nmodel name\t: x\n\nprocessor\t: 1\nmodel name\t: x\n")
	return dir
}

// connectInProcess wires an in-memory client↔server session (no HTTP, no
// auth gate) so the MCP protocol layer is exercised directly.
func connectInProcess(t *testing.T, srv *mcp.Server) *mcp.ClientSession {
	t.Helper()
	ctx := t.Context()
	serverT, clientT := mcp.NewInMemoryTransports()
	ss, err := srv.Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	cs, err := testClient().Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// assertMetricsResource checks system://metrics is advertised and returns
// the snapshot derived from the fixture /proc.
func assertMetricsResource(t *testing.T, ctx context.Context, cs *mcp.ClientSession) {
	t.Helper()

	list, err := cs.ListResources(ctx, nil)
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	if !hasResource(list.Resources, systemMetricsURI) {
		t.Fatalf("system://metrics not advertised in resources/list (%d resources)", len(list.Resources))
	}

	res, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: systemMetricsURI})
	if err != nil {
		t.Fatalf("ReadResource(system://metrics): %v", err)
	}
	if len(res.Contents) == 0 {
		t.Fatal("ReadResource returned no contents")
	}
	var got metrics.Metrics
	if err := json.Unmarshal([]byte(res.Contents[0].Text), &got); err != nil {
		t.Fatalf("resource payload is not metrics JSON: %v (payload=%q)", err, res.Contents[0].Text)
	}
	if got.MemUsedPercent != 75.0 {
		t.Fatalf("MemUsedPercent over MCP = %v; want 75.0", got.MemUsedPercent)
	}
	if got.CPUCores != 2 || got.Load1 != 0.10 || got.UptimeSeconds != 4242.0 {
		t.Fatalf("metrics over MCP = cores %d load1 %v uptime %v; want 2 / 0.10 / 4242.0", got.CPUCores, got.Load1, got.UptimeSeconds)
	}
}

// In-process client: proves the MCP protocol layer (initialize →
// resources/list → resources/read) works against the registered
// resources, bypassing the HTTP transport and auth.
func TestMCP_InProcess_ReadSystemMetrics(t *testing.T) {
	srv := newMCPServer(BuildInfo{Version: "test"}, writeProcFixtures(t), fixtureJournalRunner(""))
	cs := connectInProcess(t, srv)
	assertMetricsResource(t, t.Context(), cs)
}

// Over the real HTTP Streamable transport: proves the HTTP transport and
// the MCP protocol compose, end-to-end, through the same mux the gate
// mounts. (httptest binds loopback, so the read-tier gate's loopback
// skip applies and no token is needed here; auth-over-the-LAN stays
// covered by the gate unit tests and the live .142 smoke.)
func TestMCP_OverHTTPTransport_ReadSystemMetrics(t *testing.T) {
	srv := newServerWithProcRoot(BuildInfo{Version: "test"},
		func() (*ecdsa.PublicKey, error) { return nil, nil },
		writeProcFixtures(t))

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	ctx := t.Context()
	cs, err := testClient().Connect(ctx, &mcp.StreamableClientTransport{Endpoint: ts.URL + MCPEndpointPath}, nil)
	if err != nil {
		t.Fatalf("client.Connect over HTTP: %v", err)
	}
	defer func() { _ = cs.Close() }()

	assertMetricsResource(t, ctx, cs)
}

// journal:// over the MCP protocol: the schema resource is discoverable
// and the templated journal://{unit} read returns the parsed entries
// from the (fixture) journalctl output.
func TestMCP_InProcess_ReadJournal(t *testing.T) {
	out := `{"__REALTIME_TIMESTAMP":"1716854400000000","_SYSTEMD_UNIT":"powerlab-core.service","PRIORITY":"3","MESSAGE":"disk full"}` + "\n"
	srv := newMCPServer(BuildInfo{Version: "test"}, t.TempDir(), fixtureJournalRunner(out))
	cs := connectInProcess(t, srv)
	ctx := t.Context()

	list, err := cs.ListResources(ctx, nil)
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	if !hasResource(list.Resources, journalSchemaURI) {
		t.Fatalf("journal://schema not advertised in resources/list")
	}

	res, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "journal://core?lines=10"})
	if err != nil {
		t.Fatalf("ReadResource(journal://core): %v", err)
	}
	var got []journal.Entry
	if err := json.Unmarshal([]byte(res.Contents[0].Text), &got); err != nil {
		t.Fatalf("journal payload is not []Entry JSON: %v (payload=%q)", err, res.Contents[0].Text)
	}
	if len(got) != 1 || got[0].Message != "disk full" || got[0].Priority != 3 {
		t.Fatalf("journal entries = %+v; want one entry 'disk full' priority 3", got)
	}

	// The query is optional — an agent may read journal://<unit> with no
	// params. The template must still match.
	if _, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "journal://core"}); err != nil {
		t.Fatalf("ReadResource(journal://core) with no query: %v", err)
	}
}

func hasResource(rs []*mcp.Resource, uri string) bool {
	for _, r := range rs {
		if r.URI == uri {
			return true
		}
	}
	return false
}

func mustWrite(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
}
