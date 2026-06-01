package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/neochaotic/powerlab/backend/powerlab-mcp/coreproxy"
)

// P1.5 from the 2026-05-31 MCP-only chat-mode retro: a chat-mode agent
// asked "como está a saúde do sistema?" and couldn't answer without
// reading 4 separate resources and correlating thresholds. The smoke
// client encodes this correlation; the agent doesn't. This Tool
// bundles the correlation so a single tools/call answers the
// question and surfaces severities + warnings.

// healthCoreFixture stands up a fake core that serves the disk +
// services endpoints (which DO proxy to core) and pairs them with a
// canned apt runner output for the updates path (which DOES NOT
// proxy to core — see the regression note on evaluateUpdates).
// Returns the configured resourcesConfig + the aptRunner the
// fixture wants the registration to use.
type healthCoreFixture struct {
	diskBody     string
	servicesBody string
	// aptOutput is the raw `apt list --upgradable` text the
	// in-process aptRunner returns. Use aptOutputSecurity for the
	// security-flagged-update variant.
	aptOutput string
	// aptError, when non-nil, makes the aptRunner return that error
	// instead of any output — exercises the detected="none" path.
	aptError error
}

func (f healthCoreFixture) serve(t *testing.T) (resourcesConfig, aptRunner) {
	t.Helper()
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/sys/disk":
			_, _ = w.Write([]byte(f.diskBody))
		case "/v1/sys/services":
			_, _ = w.Write([]byte(f.servicesBody))
		default:
			t.Errorf("core received unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(core.Close)

	runtimeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(runtimeDir, coreproxy.CoreURLFile), []byte(core.URL), 0o600); err != nil {
		t.Fatalf("write .url: %v", err)
	}
	rc := resourcesConfig{
		procRoot:   writeProcFixtures(t),
		coreClient: coreproxy.NewClient(runtimeDir, core.Client()),
	}
	apt := func(_ context.Context) (string, error) {
		return f.aptOutput, f.aptError
	}
	return rc, apt
}

// aptOutputNPending builds canned `apt list --upgradable` output with
// n stable-pocket updates (none security-flagged).
func aptOutputNPending(n int) string {
	out := "Listing... Done\n"
	for i := 0; i < n; i++ {
		out += fmt.Sprintf("pkg%d/jammy 1.2.0 amd64 [upgradable from: 1.0.0]\n", i)
	}
	return out
}

// aptOutputWithSecurity builds canned apt output with stable +
// security entries — parser flags any entry whose pocket name
// contains "-security" as a security update.
func aptOutputWithSecurity(stable, security int) string {
	out := "Listing... Done\n"
	for i := 0; i < stable; i++ {
		out += fmt.Sprintf("pkg%d/jammy 1.2.0 amd64 [upgradable from: 1.0.0]\n", i)
	}
	for i := 0; i < security; i++ {
		out += fmt.Sprintf("secpkg%d/jammy-security 1.2.1 amd64 [upgradable from: 1.0.0]\n", i)
	}
	return out
}

// callGetSystemHealth invokes the Tool and decodes the structured output.
func callGetSystemHealth(t *testing.T, rc resourcesConfig, apt aptRunner) GetSystemHealthOutput {
	t.Helper()
	// Register the Tool with the test apt runner injected via the
	// testable seam (registerGetSystemHealthWith). The default
	// newMCPServer path uses execAptList, which would shell out to
	// /usr/bin/apt and fail on a Mac dev box.
	srv := newMCPServerForHealthTest(rc, apt)
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	res, err := cs.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "get_system_health",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool(get_system_health): %v", err)
	}
	if res.IsError {
		t.Fatalf("get_system_health errored: %+v", res.Content)
	}
	var out GetSystemHealthOutput
	b, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("decode: %v (raw=%s)", err, string(b))
	}
	return out
}

// newMCPServerForHealthTest mirrors newMCPServer but swaps the
// get_system_health registration for the injected-aptRunner variant.
// Keeps the rest of the surface identical so tools/list assertions
// don't drift between this and production wiring.
func newMCPServerForHealthTest(rc resourcesConfig, apt aptRunner) *mcp.Server {
	// Start with the full production server, then re-register the
	// one Tool whose runner we want to override. mcp.AddTool replaces
	// an existing registration by name, so this swap is clean.
	srv := newMCPServer(BuildInfo{Version: "test"}, rc, fixtureJournalRunner(""))
	registerGetSystemHealthWith(srv, rc.procRoot, rc.coreClient, apt)
	return srv
}

// Healthy box: all surfaces report ok; overall MUST be ok and no
// warnings should be emitted.
func TestGetSystemHealth_HealthyBoxReportsOK(t *testing.T) {
	rc, apt := healthCoreFixture{
		diskBody:     `{"success":200,"message":"ok","data":{"physical":[{"mount":"/","used_percent":40}]}}`,
		servicesBody: `{"success":200,"message":"ok","data":[{"name":"powerlab-mcp.service","active_state":"active"}]}`,
		aptOutput:    aptOutputNPending(3),
	}.serve(t)

	out := callGetSystemHealth(t, rc, apt)
	if out.Overall != "ok" {
		t.Fatalf("overall=%q; want ok. out=%+v", out.Overall, out)
	}
	for area, got := range map[string]SystemHealthArea{
		"disk":     out.Disk,
		"services": out.Services,
		"updates":  out.Updates,
	} {
		if got.Severity != "ok" {
			t.Errorf("%s.severity=%q; want ok", area, got.Severity)
		}
	}
	if len(out.Warnings) != 0 {
		t.Errorf("got %d warnings on a healthy box; want 0. warnings=%+v", len(out.Warnings), out.Warnings)
	}
}

// Disk at 92% used → warn (between 90 and 95 thresholds). Overall
// inherits the highest severity across areas.
func TestGetSystemHealth_DiskWarnThreshold(t *testing.T) {
	rc, apt := healthCoreFixture{
		diskBody:     `{"success":200,"message":"ok","data":{"physical":[{"mount":"/","used_percent":92}]}}`,
		servicesBody: `{"success":200,"message":"ok","data":[{"name":"powerlab-mcp.service","active_state":"active"}]}`,
		aptOutput:    aptOutputNPending(0),
	}.serve(t)
	out := callGetSystemHealth(t, rc, apt)
	if out.Disk.Severity != "warn" {
		t.Fatalf("disk.severity=%q; want warn for 92%% used", out.Disk.Severity)
	}
	if out.Overall != "warn" {
		t.Fatalf("overall=%q; want warn (cascaded from disk)", out.Overall)
	}
	if !findWarning(out.Warnings, "disk", "warn") {
		t.Errorf("expected disk/warn warning. warnings=%+v", out.Warnings)
	}
}

// Disk at 97% used → critical. Overall MUST escalate to critical
// regardless of any warn-level surfaces.
func TestGetSystemHealth_DiskCriticalEscalates(t *testing.T) {
	rc, apt := healthCoreFixture{
		diskBody:     `{"success":200,"message":"ok","data":{"physical":[{"mount":"/","used_percent":97}]}}`,
		servicesBody: `{"success":200,"message":"ok","data":[{"name":"powerlab-mcp.service","active_state":"active"}]}`,
		aptOutput:    aptOutputWithSecurity(2-1, 1),
	}.serve(t)
	out := callGetSystemHealth(t, rc, apt)
	if out.Disk.Severity != "critical" {
		t.Fatalf("disk.severity=%q; want critical for 97%% used", out.Disk.Severity)
	}
	if out.Overall != "critical" {
		t.Fatalf("overall=%q; want critical", out.Overall)
	}
}

// Security-flagged update count > 0 → updates warn (informational,
// not a system breaker but agent should surface).
func TestGetSystemHealth_SecurityUpdatesWarn(t *testing.T) {
	rc, apt := healthCoreFixture{
		diskBody:     `{"success":200,"message":"ok","data":{"physical":[{"mount":"/","used_percent":30}]}}`,
		servicesBody: `{"success":200,"message":"ok","data":[{"name":"powerlab-mcp.service","active_state":"active"}]}`,
		aptOutput:    aptOutputWithSecurity(47-5, 5),
	}.serve(t)
	out := callGetSystemHealth(t, rc, apt)
	if out.Updates.Severity != "warn" {
		t.Fatalf("updates.severity=%q; want warn (5 security-flagged)", out.Updates.Severity)
	}
	if !findWarning(out.Warnings, "updates", "warn") {
		t.Errorf("expected updates/warn warning. warnings=%+v", out.Warnings)
	}
}

// powerlab-mcp itself not active → critical for services area
// (self-aware: the agent is currently talking through this service,
// so its degraded state is a genuine critical signal even though it
// somehow responded).
func TestGetSystemHealth_McpServiceDownIsCritical(t *testing.T) {
	rc, apt := healthCoreFixture{
		diskBody:     `{"success":200,"message":"ok","data":{"physical":[{"mount":"/","used_percent":30}]}}`,
		servicesBody: `{"success":200,"message":"ok","data":[{"name":"powerlab-mcp.service","active_state":"failed"}]}`,
		aptOutput:    aptOutputNPending(0),
	}.serve(t)
	out := callGetSystemHealth(t, rc, apt)
	if out.Services.Severity != "critical" {
		t.Fatalf("services.severity=%q; want critical when powerlab-mcp is failed", out.Services.Severity)
	}
}

// Other powerlab-* service not active → warn (not critical, but
// agent surfaces).
func TestGetSystemHealth_NonMcpServiceDownIsWarn(t *testing.T) {
	rc, apt := healthCoreFixture{
		diskBody:     `{"success":200,"message":"ok","data":{"physical":[{"mount":"/","used_percent":30}]}}`,
		servicesBody: `{"success":200,"message":"ok","data":[{"name":"powerlab-mcp.service","active_state":"active"},{"name":"powerlab-gateway.service","active_state":"failed"}]}`,
		aptOutput:    aptOutputNPending(0),
	}.serve(t)
	out := callGetSystemHealth(t, rc, apt)
	if out.Services.Severity != "warn" {
		t.Fatalf("services.severity=%q; want warn (gateway failed but mcp active)", out.Services.Severity)
	}
}

// Tool must appear in tools/list so chat-mode agents discover it.
func TestGetSystemHealth_AdvertisedInToolsList(t *testing.T) {
	srv := newMCPServer(BuildInfo{Version: "test"},
		resourcesConfig{procRoot: t.TempDir()},
		fixtureJournalRunner(""))
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	list, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	for _, tool := range list.Tools {
		if tool.Name == "get_system_health" {
			if !strings.Contains(tool.Description, "READ") {
				t.Errorf("description must carry READ side-effect class")
			}
			return
		}
	}
	t.Fatalf("get_system_health not advertised in tools/list")
}

func findWarning(ws []SystemHealthWarning, area, severity string) bool {
	for _, w := range ws {
		if w.Area == area && w.Severity == severity {
			return true
		}
	}
	return false
}

var _ = fmt.Sprintf // keep fmt referenced for failure-message authoring

// REGRESSION (2026-06-01 end-to-end discovery): real core wraps every
// response in `{"success":200,"message":"ok","data":{...}}` and VM
// hosts (Lima, Docker Desktop) report disk in `mounts[]` because
// `physical[]` is empty. Pre-fix, evaluateDisk decoded the body
// directly and missed BOTH the envelope and the mounts fallback,
// reporting worst=0% regardless of the real fill level.
func TestGetSystemHealth_DiskParsesEnvelopeAndMountsFallback(t *testing.T) {
	rc, apt := healthCoreFixture{
		// Real core shape: envelope + empty physical + populated mounts.
		// Lima's actual response at 88.7% used on /.
		diskBody:     `{"success":200,"message":"ok","data":{"physical":[],"mounts":[{"path":"/","fs_type":"ext4","total":19682557952,"used":17437450240,"free":2228330496,"used_percent":88.7}]}}`,
		servicesBody: `{"success":200,"message":"ok","data":[{"name":"powerlab-mcp","active_state":"active"}]}`,
		aptOutput:    aptOutputNPending(0),
	}.serve(t)

	out := callGetSystemHealth(t, rc, apt)
	if out.Disk.Severity != "ok" {
		t.Fatalf("disk.severity=%q; want ok (88.7%% is below 90%% warn threshold)", out.Disk.Severity)
	}
	if !strings.Contains(out.Disk.Summary, "88.7") {
		t.Fatalf("disk.summary=%q; want it to surface the actual 88.7%% (pre-fix bug: summary said '0.0%%' because envelope was not unwrapped + mounts fallback was missing)", out.Disk.Summary)
	}
	if !strings.Contains(out.Disk.Summary, "/") {
		t.Errorf("disk.summary=%q; want it to name the worst mount path", out.Disk.Summary)
	}
}

// REGRESSION (same end-to-end): services data is an ARRAY at top
// level under `data`, not an object containing a `services` field.
// And names omit the `.service` suffix. Pre-fix, the parser couldn't
// see any services so it always reported "all healthy" regardless
// of the real state.
func TestGetSystemHealth_ServicesParsesArrayEnvelopeAndShortNames(t *testing.T) {
	rc, apt := healthCoreFixture{
		diskBody: `{"success":200,"message":"ok","data":{"physical":[],"mounts":[{"path":"/","used_percent":30}]}}`,
		// Real shape: data is array; names without .service suffix;
		// one degraded sibling service.
		servicesBody: `{"success":200,"message":"ok","data":[{"name":"powerlab-mcp","active_state":"active"},{"name":"powerlab-gateway","active_state":"failed"}]}`,
		aptOutput:    aptOutputNPending(0),
	}.serve(t)

	out := callGetSystemHealth(t, rc, apt)
	if out.Services.Severity != "warn" {
		t.Fatalf("services.severity=%q; want warn (powerlab-gateway failed)", out.Services.Severity)
	}
	if !strings.Contains(out.Services.Summary, "powerlab-gateway") {
		t.Errorf("services.summary=%q; want it to name powerlab-gateway", out.Services.Summary)
	}
}

// powerlab-mcp self-degradation MUST still escalate to critical even
// when core returns the canonical short name without the .service suffix.
func TestGetSystemHealth_McpSelfDegradeStillCriticalAcrossNameForms(t *testing.T) {
	for _, name := range []string{"powerlab-mcp", "powerlab-mcp.service"} {
		t.Run("name="+name, func(t *testing.T) {
			rc, apt := healthCoreFixture{
				diskBody:     `{"success":200,"message":"ok","data":{"physical":[],"mounts":[{"path":"/","used_percent":30}]}}`,
				servicesBody: `{"success":200,"message":"ok","data":[{"name":"` + name + `","active_state":"failed"}]}`,
				aptOutput:    aptOutputNPending(0),
			}.serve(t)
			out := callGetSystemHealth(t, rc, apt)
			if out.Services.Severity != "critical" {
				t.Fatalf("services.severity=%q with name=%q; want critical", out.Services.Severity, name)
			}
		})
	}
}

// REGRESSION (2026-06-01 end-to-end, Claude Code conversation
// against Lima): /v1/sys/disk reported /mnt/lima-cidata at 100% used
// (it's a read-only iso9660 cloud-init seed, always full by design).
// Without filtering pseudo-filesystems, the aggregator graded the
// disk as critical even though the actual ext4 root was at 88.7%
// (below the 90% warn threshold). Lock the contract: pseudo fs
// types and Lima's cidata path are excluded from the worst-mount
// computation.
func TestGetSystemHealth_DiskExcludesPseudoFsFromWorstComputation(t *testing.T) {
	rc, apt := healthCoreFixture{
		// Lima's actual mount layout — ext4 root + boot + read-only
		// cidata seed at 100%. The pseudo cidata should be ignored;
		// the real worst mount is / at 88.7%, which is below warn.
		diskBody: `{"success":200,"message":"ok","data":{"physical":[],"mounts":[
			{"path":"/","fs_type":"ext4","used_percent":88.7},
			{"path":"/boot","fs_type":"ext4","used_percent":13.6},
			{"path":"/boot/efi","fs_type":"vfat","used_percent":6.5},
			{"path":"/mnt/lima-cidata","fs_type":"iso9660","used_percent":100}
		]}}`,
		servicesBody: `{"success":200,"message":"ok","data":[{"name":"powerlab-mcp","active_state":"active"}]}`,
		aptOutput:    aptOutputNPending(0),
	}.serve(t)

	out := callGetSystemHealth(t, rc, apt)
	if out.Disk.Severity != "ok" {
		t.Fatalf("disk.severity=%q; want ok (88.7%% real-mount usage is below 90%% warn; cidata 100%% must be excluded). summary=%q",
			out.Disk.Severity, out.Disk.Summary)
	}
	if strings.Contains(out.Disk.Summary, "lima-cidata") {
		t.Errorf("disk.summary=%q; want it to NOT name lima-cidata as the worst mount", out.Disk.Summary)
	}
	if !strings.Contains(out.Disk.Summary, "88.7") {
		t.Errorf("disk.summary=%q; want it to name the actual 88.7%% reading from the real root mount", out.Disk.Summary)
	}
}

// Path-heuristic fallback: when fs_type is missing (older core build
// or partial data), the path-based check still catches lima-cidata
// and /snap/* mounts. Same lesson: the agent shouldn't grade these
// as disk pressure.
func TestGetSystemHealth_DiskPathHeuristicFallback(t *testing.T) {
	rc, apt := healthCoreFixture{
		// Missing fs_type intentionally — path heuristic should still kick in
		diskBody: `{"success":200,"message":"ok","data":{"physical":[],"mounts":[
			{"path":"/","used_percent":40},
			{"path":"/snap/firefox/123","used_percent":100},
			{"path":"/mnt/lima-cidata","used_percent":100}
		]}}`,
		servicesBody: `{"success":200,"message":"ok","data":[{"name":"powerlab-mcp","active_state":"active"}]}`,
		aptOutput:    aptOutputNPending(0),
	}.serve(t)
	out := callGetSystemHealth(t, rc, apt)
	if out.Disk.Severity != "ok" {
		t.Fatalf("disk.severity=%q; want ok (only / is real and it's at 40%%)", out.Disk.Severity)
	}
	if strings.Contains(out.Disk.Summary, "snap") || strings.Contains(out.Disk.Summary, "lima-cidata") {
		t.Errorf("disk.summary=%q; want it to NOT mention pseudo paths", out.Disk.Summary)
	}
}
