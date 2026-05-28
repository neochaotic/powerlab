package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/neochaotic/powerlab/backend/powerlab-mcp/metrics"
)

// systemMetricsURI is the MCP resource URI for the host metrics snapshot.
const systemMetricsURI = "system://metrics"

// registerSystemMetrics exposes system://metrics — a point-in-time host
// snapshot (memory, load average, uptime) read directly from procRoot.
// The handler returns the error from metrics.Collect verbatim so the
// agent sees a failed read rather than a zero-valued snapshot.
func registerSystemMetrics(s *mcp.Server, procRoot string) {
	res := &mcp.Resource{
		URI:         systemMetricsURI,
		Name:        "System metrics",
		Description: "Point-in-time host memory (total/available/used%), load average (1/5/15m), and uptime, read directly from /proc — independent of the rest of PowerLab.",
		MIMEType:    "application/json",
	}

	s.AddResource(res, func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		snap, err := metrics.Collect(procRoot)
		if err != nil {
			return nil, fmt.Errorf("collect system metrics: %w", err)
		}
		b, err := json.Marshal(snap)
		if err != nil {
			return nil, fmt.Errorf("marshal system metrics: %w", err)
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{textJSON(systemMetricsURI, string(b))}}, nil
	})
}
