package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/neochaotic/powerlab/backend/powerlab-mcp/metrics"
)

// systemMetricsURI is the MCP resource URI for the host metrics snapshot.
const systemMetricsURI = "system://metrics"

// registerSystemMetrics exposes system://metrics — a point-in-time host
// snapshot (memory, load average, uptime) read directly from procRoot.
// The handler returns the error from metrics.Collect verbatim so the
// agent sees a failed read rather than a zero-valued snapshot.
func registerSystemMetrics(m *mcpserver.MCPServer, procRoot string) {
	res := mcp.NewResource(
		systemMetricsURI,
		"System metrics",
		mcp.WithResourceDescription("Point-in-time host memory (total/available/used%), load average (1/5/15m), and uptime, read directly from /proc — independent of the rest of PowerLab."),
		mcp.WithMIMEType("application/json"),
	)

	m.AddResource(res, func(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		snap, err := metrics.Collect(procRoot)
		if err != nil {
			return nil, fmt.Errorf("collect system metrics: %w", err)
		}
		b, err := json.Marshal(snap)
		if err != nil {
			return nil, fmt.Errorf("marshal system metrics: %w", err)
		}
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      systemMetricsURI,
				MIMEType: "application/json",
				Text:     string(b),
			},
		}, nil
	})
}
