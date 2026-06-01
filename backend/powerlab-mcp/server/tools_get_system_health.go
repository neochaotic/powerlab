package server

import (
	"context"
	"encoding/json"
	"strconv"
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
	registerGetSystemHealthWith(s, procRoot, proxy, execAptList)
}

// registerGetSystemHealthWith is the testable seam. Production wires
// execAptList; tests inject a canned-output runner so the assertions
// don't depend on the host having apt at all.
func registerGetSystemHealthWith(s *mcp.Server, procRoot string, proxy *coreproxy.Client, aptRun aptRunner) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_system_health",
		Description: "READ ONLY — aggregate system health across memory, disk, PowerLab services and pending OS updates. Returns a per-category severity (ok | warn | critical | unknown) plus an overall verdict that escalates to the worst component. Use this FIRST when an operator asks 'how's the system' instead of reading 4 separate system:// resources — same data, threshold-correlated, single call.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ getSystemHealthInput) (*mcp.CallToolResult, GetSystemHealthOutput, error) {
		out := computeSystemHealth(ctx, procRoot, proxy, aptRun)
		return nil, out, nil
	})
}

func computeSystemHealth(ctx context.Context, procRoot string, proxy *coreproxy.Client, aptRun aptRunner) GetSystemHealthOutput {
	out := GetSystemHealthOutput{
		Warnings: []SystemHealthWarning{},
	}
	out.Memory, out.Warnings = evaluateMemory(procRoot, out.Warnings)
	out.Disk, out.Warnings = evaluateDisk(ctx, proxy, out.Warnings)
	out.Services, out.Warnings = evaluateServices(ctx, proxy, out.Warnings)
	out.Updates, out.Warnings = evaluateUpdates(ctx, aptRun, out.Warnings)
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

// unwrapPowerLabEnvelope decodes the canonical `{success, message, data}`
// shape every PowerLab service uses (legacy CasaOS convention preserved
// during the rebrand). Falls back to a direct decode when the envelope
// isn't present, so a future endpoint variant that returns the raw
// payload still works.
//
// REGRESSION (2026-06-01 end-to-end test, this PR): the original
// evaluateDisk and evaluateServices decoded the raw body directly,
// missing the envelope. Real core returns
// `{"success":200,"message":"ok","data":{...}}` for disk and
// `{"success":200,"message":"ok","data":[...]}` for services — both
// silently parsed as empty payloads, so disk reported worst=0% and
// services reported "all healthy" regardless of reality. The bug
// was caught when an agent in Claude Code asked for the system
// health on Lima (88.7% used) and got back "worst mount: 0.0%".
func unwrapPowerLabEnvelope(body []byte, target any) error {
	var e struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &e); err == nil && len(e.Data) > 0 && string(e.Data) != "null" {
		return json.Unmarshal(e.Data, target)
	}
	return json.Unmarshal(body, target)
}

// diskEntry matches both shapes core uses: physical disks key on
// "mount" while mounts key on "path". Both UsedPercent fields are
// consistent. The evaluator reads whichever name is non-empty.
type diskEntry struct {
	Path        string  `json:"path"`
	Mount       string  `json:"mount"`
	UsedPercent float64 `json:"used_percent"`
}

func (e diskEntry) Name() string {
	if e.Mount != "" {
		return e.Mount
	}
	return e.Path
}

type diskPayload struct {
	Physical []diskEntry `json:"physical"`
	Mounts   []diskEntry `json:"mounts"`
}

func evaluateDisk(ctx context.Context, proxy *coreproxy.Client, ws []SystemHealthWarning) (SystemHealthArea, []SystemHealthWarning) {
	body, err := proxy.GetFrom(ctx, coreproxy.ServiceCore, "/v1/sys/disk", "")
	if err != nil {
		return SystemHealthArea{Severity: "unknown", Summary: "disk unavailable: " + err.Error()}, ws
	}
	var dp diskPayload
	if jerr := unwrapPowerLabEnvelope(body, &dp); jerr != nil {
		return SystemHealthArea{Severity: "unknown", Summary: "disk parse failed: " + jerr.Error()}, ws
	}
	// Prefer physical disks when present (real metal); fall back to
	// mounts otherwise (VMs / containers expose only mount points).
	entries := dp.Physical
	if len(entries) == 0 {
		entries = dp.Mounts
	}
	worst := 0.0
	worstMount := ""
	for _, d := range entries {
		if d.UsedPercent > worst {
			worst = d.UsedPercent
			worstMount = d.Name()
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

// serviceEntry matches core's `data[]` shape. Real /v1/sys/services
// returns the array directly under `data` (no wrapping object), and
// the name field carries the unit basename WITHOUT the `.service`
// suffix (e.g. "powerlab-mcp" not "powerlab-mcp.service").
type serviceEntry struct {
	Name        string `json:"name"`
	ActiveState string `json:"active_state"`
}

// canonicalServiceName strips the `.service` suffix if present so
// equality checks against the unit short names work regardless of
// whether core was returning the short or full form. The real
// returns "powerlab-mcp" today; future-proofing against a refactor.
func canonicalServiceName(n string) string {
	return strings.TrimSuffix(n, ".service")
}

func evaluateServices(ctx context.Context, proxy *coreproxy.Client, ws []SystemHealthWarning) (SystemHealthArea, []SystemHealthWarning) {
	body, err := proxy.GetFrom(ctx, coreproxy.ServiceCore, "/v1/sys/services", "")
	if err != nil {
		return SystemHealthArea{Severity: "unknown", Summary: "services unavailable: " + err.Error()}, ws
	}
	var services []serviceEntry
	if jerr := unwrapPowerLabEnvelope(body, &services); jerr != nil {
		return SystemHealthArea{Severity: "unknown", Summary: "services parse failed: " + jerr.Error()}, ws
	}
	mcpDegraded := false
	otherDegraded := []string{}
	for _, svc := range services {
		name := canonicalServiceName(svc.Name)
		if !strings.HasPrefix(name, "powerlab-") {
			continue
		}
		if svc.ActiveState == "active" {
			continue
		}
		if name == "powerlab-mcp" {
			mcpDegraded = true
			continue
		}
		otherDegraded = append(otherDegraded, name)
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

func evaluateUpdates(ctx context.Context, aptRun aptRunner, ws []SystemHealthWarning) (SystemHealthArea, []SystemHealthWarning) {
	// REGRESSION (2026-06-01): the original implementation called
	// `coreproxy.GetFrom(ctx, ServiceCore, "/v1/sys/updates", ...)`,
	// but that endpoint does NOT exist on core — `system://updates`
	// reads `apt list --upgradable` output directly via the apt
	// runner in resources_system_updates.go. The proxy call always
	// failed with 404 on every Linux host, reporting `updates=unknown`
	// forever. Smoke (added in PR #662) caught it on Lima. The fix:
	// reuse the same `collectUpdates` helper the resource uses, so
	// the Tool and the resource see the same data.
	pl := collectUpdates(ctx, aptRun)
	if pl.Detected == "none" {
		return SystemHealthArea{Severity: "unknown", Summary: "updates unavailable: " + pl.Note}, ws
	}
	if pl.SecurityCount > 0 {
		ws = append(ws, SystemHealthWarning{
			Area: "updates", Severity: "warn",
			Message: "security-flagged OS updates pending",
			Hint:    "schedule apt upgrade (or system://updates for details)",
		})
		return SystemHealthArea{Severity: "warn", Summary: pluralPending(pl.Count) + " (" + pluralSecurity(pl.SecurityCount) + ")"}, ws
	}
	return SystemHealthArea{Severity: "ok", Summary: pluralPending(pl.Count)}, ws
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
	return strconv.FormatFloat(pct, 'f', 1, 64) + "%"
}

func pluralPending(n int) string {
	if n == 1 {
		return "1 update pending"
	}
	return strconv.Itoa(n) + " updates pending"
}

func pluralSecurity(n int) string {
	if n == 1 {
		return "1 security-flagged"
	}
	return strconv.Itoa(n) + " security-flagged"
}
