package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/neochaotic/powerlab/backend/powerlab-mcp/metrics"
)

// This drives the REAL MCP protocol end-to-end via an in-process client:
// initialize → resources/list → resources/read. The earlier "mount" test
// only proved the HTTP endpoint is reachable; this proves the server
// actually speaks MCP and that system://metrics is discoverable and
// returns parseable data. The server reads a fixture /proc, so the
// assertions are deterministic and run on any OS (no real /proc needed).
func TestMCP_InitializeListReadSystemMetrics(t *testing.T) {
	procDir := t.TempDir()
	mustWrite(t, procDir, "meminfo", "MemTotal:        2000 kB\nMemAvailable:     500 kB\n")
	mustWrite(t, procDir, "loadavg", "0.10 0.20 0.30 1/200 42\n")
	mustWrite(t, procDir, "uptime", "4242.00 8000.00\n")

	srv := newMCPServer(BuildInfo{Version: "test"}, procDir)
	cli, err := client.NewInProcessClient(srv)
	if err != nil {
		t.Fatalf("NewInProcessClient: %v", err)
	}
	defer cli.Close()

	ctx := t.Context()
	if err := cli.Start(ctx); err != nil {
		t.Fatalf("client.Start: %v", err)
	}
	if _, err := cli.Initialize(ctx, mcp.InitializeRequest{}); err != nil {
		t.Fatalf("initialize handshake failed: %v", err)
	}

	// resources/list must advertise system://metrics.
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

	// resources/read must return the JSON snapshot derived from the
	// fixture /proc: used% = (2000-500)/2000 = 75.
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
		t.Fatalf("MemUsedPercent over MCP = %v; want 75.0 (read+parse+serve path is wrong)", got.MemUsedPercent)
	}
	if got.Load1 != 0.10 || got.UptimeSeconds != 4242.0 {
		t.Fatalf("metrics over MCP = load1 %v uptime %v; want 0.10 / 4242.0", got.Load1, got.UptimeSeconds)
	}
}

func mustWrite(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
}
