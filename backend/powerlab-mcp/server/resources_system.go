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
	systemServicesURI    = "system://services"

	// systemGPUURI imports common/external::GetGPUUtilization directly
	// — no network hop (core has no /v1/sys/gpu today; the panel's
	// dashboard widget calls the same external function). Same reuse
	// pattern as audittail importing audit.Record from common.
	systemGPUURI = "system://gpu"

	// Sysadmin-tier resources (low-sensitivity). Independent reads —
	// no core dependency, security-careful:
	//   - systemKernelURI:    /proc/version + /etc/os-release + uname
	//   - systemUpdatesURI:   apt-list (Debian-only; degrades gracefully)
	//   - systemProcessesURI: gopsutil/process aggregate + top-N
	//                         (NO raw cmdline — argv can leak secrets)
	systemKernelURI    = "system://kernel"
	systemUpdatesURI   = "system://updates"
	systemProcessesURI = "system://processes"
)

// systemSchemaDoc is the literal JSON document an agent reads ONCE to
// learn what every system://* resource returns. Updating any handler's
// wire shape MUST update this in lockstep — the smoke client pins the
// metrics field set; this doc is the public contract for the rest.
const systemSchemaDoc = `{
  "description": "PowerLab host observability — CPU, memory, disk, network, GPU, plus sysadmin tier (services, kernel, OS updates, processes). Mix of independent reads (always work), thin proxies to core (return a core_unavailable payload when core is down), and direct /proc reads.",
  "resources": {
    "system://schema": "this document",
    "system://metrics": "INDEPENDENT — /proc-direct snapshot of memory + load average + uptime. Always works when MCP is up.",
    "system://utilization": "PROXIED — core's /v1/sys/utilization: CPU percent / temperature / power / model, memory, network.",
    "system://disk": "PROXIED — core's /v1/sys/disk: {physical:[{name,model,serial,size_bytes,temperature_c,health_status}], mounts:[{path,fs_type,total,used,free,used_percent}]}. Both arrays always present (empty when no data); empty physical[].model means smartctl unavailable on this host — same graceful-degrade pattern as system://gpu's empty model = no GPU.",
    "system://network": "PROXIED — core's /v1/sys/network/interfaces: per-interface state + addresses.",
    "system://gpu": "INDEPENDENT IMPORT — common/external::GetGPUUtilization. Apple Silicon (ioreg) + Nvidia (nvidia-smi). Returns {percent, memoryUsed, model, temperature}; an empty model means 'no GPU detected'. No network hop.",
    "system://services": "PROXIED — core's /v1/sys/services: ActiveState + SubState per powerlab-* systemd unit (whitelisted in core; agent cannot query arbitrary units).",
    "system://kernel": "INDEPENDENT — kernel version + arch + distro + boot time. Reads /proc/version, /etc/os-release, and uname directly. Always works.",
    "system://updates": "INDEPENDENT — pending OS package updates (Debian: 'apt list --upgradable'). Returns {detected, count, packages[]} with security-flagged entries called out. Empty + warning on non-Debian distros.",
    "system://processes": "INDEPENDENT — gopsutil aggregate {total, top_by_cpu[], top_by_mem[]} with process NAME only (no cmdline — argv can leak secrets, by design)."
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
// Bearer token forwarding (ADR-0047): the SDK surfaces the original
// HTTP request headers via req.Extra.Header. AgentIdentity() pulls
// the Bearer token; when present (LAN call), we forward it to the
// upstream so core's audit middleware records the same user. When
// absent (loopback, trusted local agent), the token is empty and
// core's own loopback-skip applies — same behaviour as before.
func registerProxiedSystem(s *mcp.Server, proxy *coreproxy.Client, uri, name, description, corePath string) {
	s.AddResource(&mcp.Resource{
		URI:         uri,
		Name:        name,
		Description: description,
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		if proxy == nil {
			payload := coreproxy.AsErrorPayload(&coreproxy.Error{
				Code:   "core_unavailable",
				Detail: "coreproxy not configured — server built without a coreproxy client",
			})
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{textJSON(uri, string(payload))}}, nil
		}
		_, token, _ := tokenFromRequest(req)
		body, err := proxy.GetFrom(ctx, coreproxy.ServiceCore, corePath, token)
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
		"Physical disks (name/model/serial/size_bytes/temperature_c/health_status) + per-mount usage (path/fs_type/total/used/free/used_percent) — proxied from core's /v1/sys/disk. SMART fields are best-effort: empty model means smartctl unavailable on this host (same graceful-degrade as system://gpu). PowerLab's category differentiator: the dashboard widget for SMART health reads the same surface.",
		"/v1/sys/disk")
}

func registerSystemNetwork(s *mcp.Server, proxy *coreproxy.Client) {
	registerProxiedSystem(s, proxy, systemNetworkURI,
		"System network (per-interface)",
		"Per-interface state + addresses — proxied from core's /v1/sys/network/interfaces. Agent can answer 'is this NIC up?' or 'what's eth0's IP?' against the same data the panel shows.",
		"/v1/sys/network/interfaces")
}

func registerSystemServices(s *mcp.Server, proxy *coreproxy.Client) {
	registerProxiedSystem(s, proxy, systemServicesURI,
		"System services (PowerLab systemd units)",
		"ActiveState + SubState for every powerlab-* systemd unit — proxied from core's /v1/sys/services. The upstream whitelists the unit set (see backend/core/service/power_actions.go PowerLabServices); agent cannot query arbitrary host units.",
		"/v1/sys/services")
}

func registerSystemKernel(s *mcp.Server, proxy *coreproxy.Client) {
	registerProxiedSystem(s, proxy, systemKernelURI,
		"System kernel + OS identity",
		"Kernel release + architecture, hostname, distribution + version, boot time, uptime, virtualization detection — proxied from core's /v1/sys/host. ADR-0044 thin-proxy: core already collects this via gopsutil's host.Info() (backend/core/service/system.go::GetSysInfo); we now expose it via HTTP so MCP doesn't need its own gopsutil dep.",
		"/v1/sys/host")
}

func registerSystemProcesses(s *mcp.Server, proxy *coreproxy.Client) {
	registerProxiedSystem(s, proxy, systemProcessesURI,
		"System processes (aggregate + top consumers)",
		"Process count + the top 10 by CPU% and top 10 by mem% — proxied from core's /v1/sys/processes. Each entry exposes pid + name + cpu_percent + mem_percent + rss_kb + user. Cmdline (argv) is intentionally omitted at the source — argv routinely carries secrets (passwords passed as flags, signed URLs, JWT tokens via env expansion).",
		"/v1/sys/processes")
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
