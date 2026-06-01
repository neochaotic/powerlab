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
		diskBody:     `{"physical":[{"mount":"/","used_percent":40}]}`,
		servicesBody: `{"services":[{"name":"powerlab-mcp.service","active_state":"active"}]}`,
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
		diskBody:     `{"physical":[{"mount":"/","used_percent":92}]}`,
		servicesBody: `{"services":[{"name":"powerlab-mcp.service","active_state":"active"}]}`,
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
		diskBody:     `{"physical":[{"mount":"/","used_percent":97}]}`,
		servicesBody: `{"services":[{"name":"powerlab-mcp.service","active_state":"active"}]}`,
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
		diskBody:     `{"physical":[{"mount":"/","used_percent":30}]}`,
		servicesBody: `{"services":[{"name":"powerlab-mcp.service","active_state":"active"}]}`,
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
		diskBody:     `{"physical":[{"mount":"/","used_percent":30}]}`,
		servicesBody: `{"services":[{"name":"powerlab-mcp.service","active_state":"failed"}]}`,
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
		diskBody:     `{"physical":[{"mount":"/","used_percent":30}]}`,
		servicesBody: `{"services":[{"name":"powerlab-mcp.service","active_state":"active"},{"name":"powerlab-gateway.service","active_state":"failed"}]}`,
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
