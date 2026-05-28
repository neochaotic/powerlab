package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/neochaotic/powerlab/backend/powerlab-mcp/coreproxy"
	"github.com/neochaotic/powerlab/backend/powerlab-mcp/metrics"
)

// systemMetricsURI is the MCP resource URI for the host metrics snapshot
// MCP reads directly from /proc (independent of core — survives core down).
const systemMetricsURI = "system://metrics"

// systemUtilizationURI is the MCP resource URI for the rich utilisation
// snapshot proxied from core's /v1/sys/utilization endpoint (ADR-0044).
// Carries CPU percent + temperature + power + model + memory + network
// in one payload — the same surface core's panel dashboard reads.
const systemUtilizationURI = "system://utilization"

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

// registerSystemUtilization exposes system://utilization — the rich
// snapshot core's panel dashboard already reads. Thin proxy per ADR-0044:
// hands the agent's JWT (when present) straight to core, returns the
// upstream body verbatim, and on failure produces a structured
// `core_unavailable` payload pointing the agent at audit:// + journal://
// (which never need core to be up).
//
// proxy is nil-tolerant — a server constructed without a coreproxy
// (e.g. early tests) reports the resource as unavailable rather than
// crashing. Production wiring always passes a Client.
func registerSystemUtilization(s *mcp.Server, proxy *coreproxy.Client) {
	res := &mcp.Resource{
		URI:         systemUtilizationURI,
		Name:        "System utilization (rich)",
		Description: "CPU (percent / temperature / power / model), memory, and network — the rich snapshot proxied from core's /v1/sys/utilization. When core is down, returns a structured core_unavailable payload; audit:// and journal:// remain readable.",
		MIMEType:    "application/json",
	}
	s.AddResource(res, func(ctx context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		if proxy == nil {
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{textJSON(systemUtilizationURI, string(coreproxy.AsErrorPayload(&coreproxy.Error{Code: "core_unavailable", Detail: "coreproxy not configured — server built without a coreproxy client"})))}}, nil
		}
		// Bearer token forwarding is plumbed at a higher layer (the
		// MCP transport doesn't currently let an SDK handler see the
		// original Authorization header); for now MCP-to-core calls
		// rely on core's loopback skip, which works for the in-box
		// scenario this PR ships. LAN-to-LAN agent-identity
		// forwarding lands when the SDK exposes the request context
		// — tracked as a follow-up.
		body, err := proxy.Get(ctx, "/v1/sys/utilization", "")
		if err != nil {
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{textJSON(systemUtilizationURI, string(coreproxy.AsErrorPayload(err)))}}, nil
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{textJSON(systemUtilizationURI, string(body))}}, nil
	})
}
