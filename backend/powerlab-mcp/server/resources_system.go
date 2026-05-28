package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/neochaotic/powerlab/backend/common/external"
	"github.com/neochaotic/powerlab/backend/powerlab-mcp/coreproxy"
	"github.com/neochaotic/powerlab/backend/powerlab-mcp/metrics"
)

// system://* URIs — kept together so the schema document below is the
// single source of truth for what's published.
const (
	// systemSchemaURI is the self-describing schema for the system://*
	// resource family.
	systemSchemaURI = "system://schema"

	// systemMetricsURI is the /proc-direct snapshot — independent of
	// core (survives core down).
	systemMetricsURI = "system://metrics"

	// Proxied resources (ADR-0044 hybrid). Each one round-trips a core
	// endpoint via coreproxy.Client; when core is down they serve the
	// canonical core_unavailable payload with a fallback hint.
	systemUtilizationURI = "system://utilization"
	systemDiskURI        = "system://disk"
	systemNetworkURI     = "system://network"

	// systemGPUURI imports common/external::GetGPUUtilization directly
	// — no network hop (core has no /v1/sys/gpu today; the panel's
	// dashboard widget calls the same external function). Same reuse
	// pattern as audittail importing audit.Record from common.
	systemGPUURI = "system://gpu"
)

// systemSchemaDoc is the literal JSON document an agent reads ONCE to
// learn what every system://* resource returns. Updating any handler's
// wire shape MUST update this in lockstep — the smoke client pins the
// metrics field set; this doc is the public contract for the rest.
const systemSchemaDoc = `{
  "description": "PowerLab host observability — CPU, memory, disk, network, GPU. Mix of independent reads (always work) and thin proxies to core (return a core_unavailable payload when core is down).",
  "resources": {
    "system://schema": "this document",
    "system://metrics": "INDEPENDENT — /proc-direct snapshot of memory + load average + uptime. Always works when MCP is up.",
    "system://utilization": "PROXIED — core's /v1/sys/utilization: CPU percent / temperature / power / model, memory, network.",
    "system://disk": "PROXIED — core's /v1/sys/disk: physical disks + per-mount usage + SMART metadata (model, serial, temperature). Same surface the panel reads.",
    "system://network": "PROXIED — core's /v1/sys/net: per-interface throughput counters + state (up/down/connected).",
    "system://gpu": "INDEPENDENT IMPORT — common/external::GetGPUUtilization. Apple Silicon (ioreg) + Nvidia (nvidia-smi). Returns {percent, memoryUsed, model, temperature}; an empty model means 'no GPU detected'. No network hop."
  },
  "fields_metrics": {
    "mem_total_kb": "total physical memory, KiB",
    "mem_available_kb": "available to userspace (/proc/meminfo MemAvailable), KiB",
    "mem_used_percent": "rounded 0..100",
    "load1 / load5 / load15": "load averages (1m / 5m / 15m)",
    "cpu_cores": "logical CPU count",
    "uptime_seconds": "host uptime in seconds"
  },
  "fields_gpu": {
    "percent": "0..100 utilization (Nvidia: nvidia-smi --query-gpu=utilization.gpu; Apple Silicon: derived from IOAccelerator perf state)",
    "memoryUsed": "bytes (normalised across SMI and ioreg)",
    "model": "GPU model string ('Apple Silicon GPU', 'NVIDIA GeForce …', or empty when no GPU detected)",
    "temperature": "Celsius integer (0 on macOS where IOAccelerator does not surface it)"
  },
  "proxy_error_shape": {
    "error": "core_unavailable | core_status_NNN — pattern-match this and pivot to audit:// + journal:// reads",
    "detail": "human-readable cause (e.g. core's .url file missing, transport failure, upstream non-2xx)",
    "fallback": "always mentions audit + journal — the resources that don't need core"
  }
}`

// registerSystemMetrics exposes system://metrics — a /proc-direct host
// snapshot. Independent of core (ADR-0044 calls this out as one of the
// two surfaces that survive any other service being down).
func registerSystemMetrics(s *mcp.Server, procRoot string) {
	s.AddResource(&mcp.Resource{
		URI:         systemMetricsURI,
		Name:        "System metrics",
		Description: "Point-in-time host memory (total/available/used%), load average (1/5/15m), and uptime, read directly from /proc — independent of the rest of PowerLab.",
		MIMEType:    "application/json",
	}, func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
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

// registerSystemSchema serves systemSchemaDoc. Tiny but mandatory —
// agents read schemas first to learn what's there before driving
// concrete reads.
func registerSystemSchema(s *mcp.Server) {
	s.AddResource(&mcp.Resource{
		URI:         systemSchemaURI,
		Name:        "System resources schema",
		Description: "Self-describing index of every system:// resource — what each one returns, which are independent vs. proxied, and the proxy error shape.",
		MIMEType:    "application/json",
	}, func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{textJSON(systemSchemaURI, systemSchemaDoc)}}, nil
	})
}

// registerProxiedSystem registers a thin-proxy system:// resource:
// reads corePath from core via the supplied coreproxy.Client and
// round-trips the body verbatim. On failure (transport, non-2xx,
// nil proxy) it serves the canonical core_unavailable payload —
// the agent sees a real resource value either way and pattern-matches
// on the `error` field to pivot.
//
// Bearer token forwarding intentionally absent in this iteration:
// the MCP SDK's resource handler context does not currently surface
// the original Authorization header. MCP-to-core calls rely on core's
// loopback skip (MCP runs on the same box). LAN agent-identity
// forwarding is a follow-up tracked separately.
func registerProxiedSystem(s *mcp.Server, proxy *coreproxy.Client, uri, name, description, corePath string) {
	s.AddResource(&mcp.Resource{
		URI:         uri,
		Name:        name,
		Description: description,
		MIMEType:    "application/json",
	}, func(ctx context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		if proxy == nil {
			payload := coreproxy.AsErrorPayload(&coreproxy.Error{
				Code:   "core_unavailable",
				Detail: "coreproxy not configured — server built without a coreproxy client",
			})
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{textJSON(uri, string(payload))}}, nil
		}
		body, err := proxy.GetFrom(ctx, coreproxy.ServiceCore, corePath, "")
		if err != nil {
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{textJSON(uri, string(coreproxy.AsErrorPayload(err)))}}, nil
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{textJSON(uri, string(body))}}, nil
	})
}

// registerSystemUtilization, registerSystemDisk, registerSystemNetwork
// are the proxied resources. Each one's description is the contract the
// agent reads — keep them precise.
func registerSystemUtilization(s *mcp.Server, proxy *coreproxy.Client) {
	registerProxiedSystem(s, proxy, systemUtilizationURI,
		"System utilization (rich)",
		"CPU (percent / temperature / power / model), memory, and network — the rich snapshot proxied from core's /v1/sys/utilization. Same surface the panel dashboard reads.",
		"/v1/sys/utilization")
}

func registerSystemDisk(s *mcp.Server, proxy *coreproxy.Client) {
	registerProxiedSystem(s, proxy, systemDiskURI,
		"System disk (physical + per-mount + SMART)",
		"Physical disks, per-mount usage, SMART metadata (model, serial, temperature) — proxied from core's /v1/sys/disk. PowerLab's category differentiator: the dashboard widget for SMART health reads the same surface.",
		"/v1/sys/disk")
}

func registerSystemNetwork(s *mcp.Server, proxy *coreproxy.Client) {
	registerProxiedSystem(s, proxy, systemNetworkURI,
		"System network (per-interface)",
		"Per-interface throughput counters + state — proxied from core's /v1/sys/net. Agent can answer 'is this NIC up?' or 'how much traffic on eth0 right now?' against the same data the panel shows.",
		"/v1/sys/net")
}

// registerSystemGPU exposes system://gpu using common/external's
// GetGPUUtilization directly — no network hop, same code path the panel
// dashboard widget runs (ADR-0044 "reuse existing observability layer"
// note). Returns the {percent, memoryUsed, model, temperature} shape
// regardless of vendor (Apple Silicon via ioreg, Nvidia via nvidia-smi);
// an empty Model string means "no GPU detected" — not a failure.
//
// Importing direct instead of proxying matches the audittail pattern
// (reuse audit.Record from common, read the file ourselves) and keeps
// system://gpu working when core is down — GPU detection has no upstream
// dependency.
func registerSystemGPU(s *mcp.Server) {
	s.AddResource(&mcp.Resource{
		URI:         systemGPUURI,
		Name:        "System GPU",
		Description: "Live GPU utilisation snapshot: percent / memoryUsed (bytes) / model / temperature. Apple Silicon (ioreg) + Nvidia (nvidia-smi). An empty model means 'no GPU detected' — not an error. Independent of core.",
		MIMEType:    "application/json",
	}, func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		// GetGPUUtilization returns a *GPUUtilization; nil-safe here
		// because a no-GPU box gives us an empty struct rather than
		// nil in practice, but defend anyway so the agent never reads
		// the literal "null".
		snap := external.GetGPUUtilization()
		if snap == nil {
			snap = &external.GPUUtilization{}
		}
		b, err := json.Marshal(snap)
		if err != nil {
			return nil, fmt.Errorf("marshal gpu utilization: %w", err)
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{textJSON(systemGPUURI, string(b))}}, nil
	})
}
