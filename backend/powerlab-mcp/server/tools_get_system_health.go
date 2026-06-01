package server

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/neochaotic/powerlab/backend/powerlab-mcp/coreproxy"
	"github.com/neochaotic/powerlab/backend/powerlab-mcp/metrics"
)

// P1.5 from the 2026-05-31 MCP-only chat-mode test: agents asked
// "how's the system health?" had to read system://metrics +
// system://disk + system://services + system://updates separately,
// then encode the threshold correlations themselves. The smoke
// client already has that knowledge baked in; this Tool exposes the
// same correlation to chat-mode agents in one call.
//
// Side-effect class: READ ONLY. Aggregates four upstream reads and
// applies fixed thresholds. The thresholds match the panel's own
// "system health" dashboard so an agent's surfaced severity tracks
// what an operator would see in the UI.

// SystemHealthArea is the per-category state. Severity climbs ok →
// warn → critical → unknown (the last reserved for fetch failures —
// the agent learns the data is unavailable, not that it's healthy).
type SystemHealthArea struct {
	Severity string `json:"severity"`
	Summary  string `json:"summary"`
}

// SystemHealthWarning is a structured message the agent surfaces to
// the operator. Area names the category, Hint suggests a remediation.
type SystemHealthWarning struct {
	Area     string `json:"area"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Hint     string `json:"hint,omitempty"`
}

// GetSystemHealthOutput is the structured Tool response. Overall
// inherits the highest severity across all areas.
type GetSystemHealthOutput struct {
	Overall  string                `json:"overall"`
	Memory   SystemHealthArea     `json:"memory"`
	Disk     SystemHealthArea     `json:"disk"`
	Services SystemHealthArea     `json:"services"`
	Updates  SystemHealthArea     `json:"updates"`
	Warnings []SystemHealthWarning `json:"warnings,omitempty"`
}

type getSystemHealthInput struct{}

// Thresholds chosen to match the panel dashboard's traffic-light
// model. Changing them is a UX decision (the operator's mental
// model is the panel); coordinate with frontend before tightening.
const (
	memWarnPct      = 85.0
	memCriticalPct  = 95.0
	diskWarnPct     = 90.0
	diskCriticalPct = 95.0
)

func registerGetSystemHealth(s *mcp.Server, procRoot string, proxy *coreproxy.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_system_health",
		Description: "READ ONLY — aggregate system health across memory, disk, PowerLab services and pending OS updates. Returns a per-category severity (ok | warn | critical | unknown) plus an overall verdict that escalates to the worst component. Use this FIRST when an operator asks 'how's the system' instead of reading 4 separate system:// resources — same data, threshold-correlated, single call.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ getSystemHealthInput) (*mcp.CallToolResult, GetSystemHealthOutput, error) {
		out := computeSystemHealth(ctx, procRoot, proxy)
		return nil, out, nil
	})
}

func computeSystemHealth(ctx context.Context, procRoot string, proxy *coreproxy.Client) GetSystemHealthOutput {
	out := GetSystemHealthOutput{
		Warnings: []SystemHealthWarning{},
	}
	out.Memory, out.Warnings = evaluateMemory(procRoot, out.Warnings)
	out.Disk, out.Warnings = evaluateDisk(ctx, proxy, out.Warnings)
	out.Services, out.Warnings = evaluateServices(ctx, proxy, out.Warnings)
	out.Updates, out.Warnings = evaluateUpdates(ctx, proxy, out.Warnings)
	out.Overall = highestSeverity(out.Memory, out.Disk, out.Services, out.Updates)
	return out
}

func evaluateMemory(procRoot string, ws []SystemHealthWarning) (SystemHealthArea, []SystemHealthWarning) {
	m, err := metrics.Collect(procRoot)
	if err != nil {
		return SystemHealthArea{Severity: "unknown", Summary: "metrics unavailable: " + err.Error()}, ws
	}
	pct := m.MemUsedPercent
	switch {
	case pct >= memCriticalPct:
		ws = append(ws, SystemHealthWarning{
			Area: "memory", Severity: "critical",
			Message: "memory used over critical threshold",
			Hint:    "free RAM by stopping or restarting heavy apps; consider adding swap",
		})
		return SystemHealthArea{Severity: "critical", Summary: percentSummary("memory", pct)}, ws
	case pct >= memWarnPct:
		ws = append(ws, SystemHealthWarning{
			Area: "memory", Severity: "warn",
			Message: "memory used above warn threshold",
			Hint:    "monitor memory headroom; tune container limits if this persists",
		})
		return SystemHealthArea{Severity: "warn", Summary: percentSummary("memory", pct)}, ws
	}
	return SystemHealthArea{Severity: "ok", Summary: percentSummary("memory", pct)}, ws
}

type diskPayload struct {
	Physical []struct {
		Mount       string  `json:"mount"`
		UsedPercent float64 `json:"used_percent"`
	} `json:"physical"`
}

func evaluateDisk(ctx context.Context, proxy *coreproxy.Client, ws []SystemHealthWarning) (SystemHealthArea, []SystemHealthWarning) {
	body, err := proxy.GetFrom(ctx, coreproxy.ServiceCore, "/v1/sys/disk", "")
	if err != nil {
		return SystemHealthArea{Severity: "unknown", Summary: "disk unavailable: " + err.Error()}, ws
	}
	var dp diskPayload
	if jerr := json.Unmarshal(body, &dp); jerr != nil {
		return SystemHealthArea{Severity: "unknown", Summary: "disk parse failed: " + jerr.Error()}, ws
	}
	worst := 0.0
	worstMount := ""
	for _, d := range dp.Physical {
		if d.UsedPercent > worst {
			worst = d.UsedPercent
			worstMount = d.Mount
		}
	}
	switch {
	case worst >= diskCriticalPct:
		ws = append(ws, SystemHealthWarning{
			Area: "disk", Severity: "critical",
			Message: "filesystem " + worstMount + " over critical threshold",
			Hint:    "free disk on " + worstMount + " (prune docker images / clear app caches); upgrade may fail until resolved",
		})
		return SystemHealthArea{Severity: "critical", Summary: "worst mount " + worstMount + ": " + pctToString(worst)}, ws
	case worst >= diskWarnPct:
		ws = append(ws, SystemHealthWarning{
			Area: "disk", Severity: "warn",
			Message: "filesystem " + worstMount + " above warn threshold",
			Hint:    "schedule cleanup on " + worstMount,
		})
		return SystemHealthArea{Severity: "warn", Summary: "worst mount " + worstMount + ": " + pctToString(worst)}, ws
	}
	return SystemHealthArea{Severity: "ok", Summary: "worst mount " + worstMount + ": " + pctToString(worst)}, ws
}

type servicesPayload struct {
	Services []struct {
		Name        string `json:"name"`
		ActiveState string `json:"active_state"`
	} `json:"services"`
}

func evaluateServices(ctx context.Context, proxy *coreproxy.Client, ws []SystemHealthWarning) (SystemHealthArea, []SystemHealthWarning) {
	body, err := proxy.GetFrom(ctx, coreproxy.ServiceCore, "/v1/sys/services", "")
	if err != nil {
		return SystemHealthArea{Severity: "unknown", Summary: "services unavailable: " + err.Error()}, ws
	}
	var sp servicesPayload
	if jerr := json.Unmarshal(body, &sp); jerr != nil {
		return SystemHealthArea{Severity: "unknown", Summary: "services parse failed: " + jerr.Error()}, ws
	}
	mcpDegraded := false
	otherDegraded := []string{}
	for _, svc := range sp.Services {
		if !strings.HasPrefix(svc.Name, "powerlab-") {
			continue
		}
		if svc.ActiveState == "active" {
			continue
		}
		if svc.Name == "powerlab-mcp.service" {
			mcpDegraded = true
			continue
		}
		otherDegraded = append(otherDegraded, svc.Name)
	}
	switch {
	case mcpDegraded:
		// Self-signal: the agent IS reaching us so the report itself
		// is evidence the binary is responsive, but the recorded
		// state says otherwise — surface as critical regardless.
		ws = append(ws, SystemHealthWarning{
			Area: "services", Severity: "critical",
			Message: "powerlab-mcp service reports non-active state",
			Hint:    "systemctl status powerlab-mcp; check journalctl -u powerlab-mcp -n 50",
		})
		return SystemHealthArea{Severity: "critical", Summary: "powerlab-mcp itself is not active"}, ws
	case len(otherDegraded) > 0:
		ws = append(ws, SystemHealthWarning{
			Area: "services", Severity: "warn",
			Message: "non-active PowerLab services: " + strings.Join(otherDegraded, ", "),
			Hint:    "systemctl status <name> for each; journalctl -u <name> -n 50",
		})
		return SystemHealthArea{Severity: "warn", Summary: "degraded: " + strings.Join(otherDegraded, ", ")}, ws
	}
	return SystemHealthArea{Severity: "ok", Summary: "all PowerLab services active"}, ws
}

// updatesHealthPayload mirrors only the two fields the health
// aggregator needs from /v1/sys/updates — the canonical type
// (updatesPayload in resources_system_updates.go) carries the
// richer package list the system://updates resource serves.
type updatesHealthPayload struct {
	Pending  int `json:"pending"`
	Security int `json:"security"`
}

func evaluateUpdates(ctx context.Context, proxy *coreproxy.Client, ws []SystemHealthWarning) (SystemHealthArea, []SystemHealthWarning) {
	body, err := proxy.GetFrom(ctx, coreproxy.ServiceCore, "/v1/sys/updates", "")
	if err != nil {
		return SystemHealthArea{Severity: "unknown", Summary: "updates unavailable: " + err.Error()}, ws
	}
	var up updatesHealthPayload
	if jerr := json.Unmarshal(body, &up); jerr != nil {
		return SystemHealthArea{Severity: "unknown", Summary: "updates parse failed: " + jerr.Error()}, ws
	}
	if up.Security > 0 {
		ws = append(ws, SystemHealthWarning{
			Area: "updates", Severity: "warn",
			Message: "security-flagged OS updates pending",
			Hint:    "schedule apt upgrade (or system://updates for details)",
		})
		return SystemHealthArea{Severity: "warn", Summary: pluralPending(up.Pending) + " (" + pluralSecurity(up.Security) + ")"}, ws
	}
	return SystemHealthArea{Severity: "ok", Summary: pluralPending(up.Pending)}, ws
}

// highestSeverity picks the worst across areas. Ordering:
// critical > warn > ok > unknown (unknown is "data missing" rather
// than "data confirmed safe" — never escalate as critical from it,
// but never let it mask a real critical from another area).
func highestSeverity(areas ...SystemHealthArea) string {
	rank := func(s string) int {
		switch s {
		case "critical":
			return 3
		case "warn":
			return 2
		case "ok":
			return 1
		default: // unknown
			return 0
		}
	}
	worst := "ok"
	for _, a := range areas {
		if rank(a.Severity) > rank(worst) {
			worst = a.Severity
		}
	}
	return worst
}

func percentSummary(area string, pct float64) string {
	return area + " used: " + pctToString(pct)
}

func pctToString(pct float64) string {
	// One decimal place is enough for an agent to surface; the
	// resource still carries the raw float for fine-grained reads.
	return formatFloat(pct, 1) + "%"
}

func formatFloat(v float64, decimals int) string {
	mult := 1.0
	for i := 0; i < decimals; i++ {
		mult *= 10
	}
	rounded := float64(int(v*mult+0.5)) / mult
	// Manual format to avoid pulling in strconv just for this one
	// helper; the precision is bounded so int conversion is safe.
	intPart := int(rounded)
	fracPart := int((rounded - float64(intPart)) * mult)
	if decimals == 0 {
		return itoa(intPart)
	}
	frac := itoa(fracPart)
	for len(frac) < decimals {
		frac = "0" + frac
	}
	return itoa(intPart) + "." + frac
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	out := ""
	for v > 0 {
		out = string(rune('0'+v%10)) + out
		v /= 10
	}
	if neg {
		out = "-" + out
	}
	return out
}

func pluralPending(n int) string {
	if n == 1 {
		return "1 update pending"
	}
	return itoa(n) + " updates pending"
}

func pluralSecurity(n int) string {
	if n == 1 {
		return "1 security-flagged"
	}
	return itoa(n) + " security-flagged"
}
