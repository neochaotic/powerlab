package server

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/neochaotic/powerlab/backend/powerlab-mcp/journal"
	"github.com/neochaotic/powerlab/backend/powerlab-mcp/metrics"
)

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

// assertMetricsResource runs resources/list + resources/read against an
// already-initialized client and checks system://metrics is advertised
// and returns the snapshot derived from the fixture /proc.
func assertMetricsResource(t *testing.T, ctx context.Context, cli *client.Client) {
	t.Helper()

	list, err := cli.ListResources(ctx, mcp.ListResourcesRequest{})
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	found := false
	for _, r := range list.Resources {
		if r.URI == systemMetricsURI {
			found = true
		}
	}
	if !found {
		t.Fatalf("system://metrics not advertised in resources/list (%d resources)", len(list.Resources))
	}

	req := mcp.ReadResourceRequest{}
	req.Params.URI = systemMetricsURI
	res, err := cli.ReadResource(ctx, req)
	if err != nil {
		t.Fatalf("ReadResource(system://metrics): %v", err)
	}
	if len(res.Contents) == 0 {
		t.Fatal("ReadResource returned no contents")
	}
	text, ok := mcp.AsTextResourceContents(res.Contents[0])
	if !ok {
		t.Fatalf("content[0] is not text: %T", res.Contents[0])
	}
	var got metrics.Metrics
	if err := json.Unmarshal([]byte(text.Text), &got); err != nil {
		t.Fatalf("resource payload is not metrics JSON: %v (payload=%q)", err, text.Text)
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
	cli, err := client.NewInProcessClient(srv)
	if err != nil {
		t.Fatalf("NewInProcessClient: %v", err)
	}
	defer func() { _ = cli.Close() }()

	ctx := t.Context()
	if err := cli.Start(ctx); err != nil {
		t.Fatalf("client.Start: %v", err)
	}
	if _, err := cli.Initialize(ctx, mcp.InitializeRequest{}); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	assertMetricsResource(t, ctx, cli)
}

// journal:// over the MCP protocol: the schema resource is discoverable
// and the templated journal://{unit} read returns the parsed entries
// from the (fixture) journalctl output.
func TestMCP_InProcess_ReadJournal(t *testing.T) {
	out := `{"__REALTIME_TIMESTAMP":"1716854400000000","_SYSTEMD_UNIT":"powerlab-core.service","PRIORITY":"3","MESSAGE":"disk full"}` + "\n"
	srv := newMCPServer(BuildInfo{Version: "test"}, t.TempDir(), fixtureJournalRunner(out))
	cli, err := client.NewInProcessClient(srv)
	if err != nil {
		t.Fatalf("NewInProcessClient: %v", err)
	}
	defer func() { _ = cli.Close() }()

	ctx := t.Context()
	if err := cli.Start(ctx); err != nil {
		t.Fatalf("client.Start: %v", err)
	}
	if _, err := cli.Initialize(ctx, mcp.InitializeRequest{}); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	// journal://schema is a static resource — it must be advertised.
	list, err := cli.ListResources(ctx, mcp.ListResourcesRequest{})
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	if !hasResource(list.Resources, journalSchemaURI) {
		t.Fatalf("journal://schema not advertised in resources/list")
	}

	// Reading the templated journal://{unit} URI returns the parsed entry.
	req := mcp.ReadResourceRequest{}
	req.Params.URI = "journal://core?lines=10"
	res, err := cli.ReadResource(ctx, req)
	if err != nil {
		t.Fatalf("ReadResource(journal://core): %v", err)
	}
	text, ok := mcp.AsTextResourceContents(res.Contents[0])
	if !ok {
		t.Fatalf("journal content is not text: %T", res.Contents[0])
	}
	var got []journal.Entry
	if err := json.Unmarshal([]byte(text.Text), &got); err != nil {
		t.Fatalf("journal payload is not []Entry JSON: %v (payload=%q)", err, text.Text)
	}
	if len(got) != 1 || got[0].Message != "disk full" || got[0].Priority != 3 {
		t.Fatalf("journal entries = %+v; want one entry 'disk full' priority 3", got)
	}

	// The query is optional — an agent may read journal://<unit> with no
	// params. The template must still match.
	bare := mcp.ReadResourceRequest{}
	bare.Params.URI = "journal://core"
	if _, err := cli.ReadResource(ctx, bare); err != nil {
		t.Fatalf("ReadResource(journal://core) with no query: %v", err)
	}
}

func hasResource(rs []mcp.Resource, uri string) bool {
	for _, r := range rs {
		if r.URI == uri {
			return true
		}
	}
	return false
}

// Over the real HTTP Streamable transport: this closes the gap the
// in-process test leaves — it proves the HTTP transport and the MCP
// protocol compose, end-to-end, through the same mux the gate mounts.
// (httptest binds loopback, so the read-tier gate's loopback skip
// applies and no token is needed here; auth-over-the-LAN stays covered
// by the gate unit tests and the live .142 smoke — those two seams are
// only ever joined on real hardware.)
func TestMCP_OverHTTPTransport_ReadSystemMetrics(t *testing.T) {
	srv := newServerWithProcRoot(BuildInfo{Version: "test"},
		func() (*ecdsa.PublicKey, error) { return nil, nil },
		writeProcFixtures(t))

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	cli, err := client.NewStreamableHttpClient(ts.URL + MCPEndpointPath)
	if err != nil {
		t.Fatalf("NewStreamableHttpClient: %v", err)
	}
	defer func() { _ = cli.Close() }()

	ctx := t.Context()
	if err := cli.Start(ctx); err != nil {
		t.Fatalf("client.Start: %v", err)
	}
	if _, err := cli.Initialize(ctx, mcp.InitializeRequest{}); err != nil {
		t.Fatalf("initialize over HTTP: %v", err)
	}
	assertMetricsResource(t, ctx, cli)
}

func mustWrite(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
}
