package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/neochaotic/powerlab/backend/powerlab-mcp/composevalidator"
	"github.com/neochaotic/powerlab/backend/powerlab-mcp/coreproxy"
)

// ADR-0046 batch 3 — destructive tools. These are NOT registered
// unless the operator explicitly opts in via cfg.EnableDestructiveTools.
// The opt-in is the threat-model gate ADR-0046 §4 calls out — until
// the panel-side "pending agent action" approval UI ships, the
// mcp.conf knob is the operator's documented consent that an
// authenticated agent can now mutate app state autonomously.

// InstallAppInput accepts the proposed custom-app compose YAML plus
// an optional dry-run flag the agent can use to validate without
// touching the box. Per ADR-0046 §4, install_app MUST run the YAML
// through composevalidator BEFORE forwarding to app-management —
// the upstream's own checks (compose syntax, image policy) are
// orthogonal.
type InstallAppInput struct {
	// ComposeYAML is the raw Docker Compose YAML the agent proposes
	// to install. composevalidator inspects it for the deny-list
	// patterns BEFORE this body ever reaches app-management.
	ComposeYAML string `json:"compose_yaml" jsonschema:"Raw Docker Compose YAML document — must NOT include privileged: true; no Docker socket binds; no host namespace sharing; no dangerous cap_add; no raw /dev passthrough; no sensitive host path binds"`

	// DryRun maps to app-management's ?dry_run=true query — the
	// upstream validates the YAML against its own checks without
	// actually creating containers. Combined with our local
	// composevalidator pass an agent gets a full "would this
	// install?" answer.
	DryRun bool `json:"dry_run,omitempty" jsonschema:"If true the upstream validates compose-syntax + image policy without installing; composevalidator deny-list still runs first"`
}

// InstallAppOutput carries either the upstream's success body OR the
// list of composevalidator violations when the YAML fails our local
// pass.
type InstallAppOutput struct {
	Status       string                    `json:"status"`
	DryRun       bool                      `json:"dry_run"`
	Violations   []composevalidator.Violation `json:"violations,omitempty"`
	UpstreamBody string                    `json:"upstream_body,omitempty"`
}

// UninstallAppInput is the typed input for uninstall_app — the
// compose app id is the same one apps://list publishes.
type UninstallAppInput struct {
	ID string `json:"id" jsonschema:"Compose app id — exactly as it appears in apps://list (e.g. 'plex' or 'jellyfin')"`
}

// UninstallAppOutput is what the agent reads after a successful
// uninstall. Status is human-readable for the agent to echo back.
type UninstallAppOutput struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// registerDestructiveTools wires install_app + uninstall_app. The
// caller is expected to honour cfg.EnableDestructiveTools and skip
// this call entirely when the flag is false — that's the gate.
// proxy is the coreproxy.Client used for the upstream PUT/POST/DELETE.
func registerDestructiveTools(s *mcp.Server, proxy *coreproxy.Client) {
	registerInstallApp(s, proxy)
	registerUninstallApp(s, proxy)
}

// registerInstallApp wires the install_app tool. composevalidator
// runs FIRST — if any deny-list rule trips we never call app-management
// at all and the agent reads the structured violation list.
func registerInstallApp(s *mcp.Server, proxy *coreproxy.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "install_app",
		Description: "Install a custom Docker Compose app on PowerLab. DESTRUCTIVE — creates new containers + storage. The YAML is validated locally against the ADR-0046 deny-list (no privileged; no Docker socket; no host namespace sharing; no dangerous cap_add; no raw devices; no sensitive host path binds) BEFORE app-management sees it. Set dry_run=true to validate without installing.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in InstallAppInput) (*mcp.CallToolResult, InstallAppOutput, error) {
		// Empty-check on the trimmed form so whitespace-only input is
		// rejected, but the upstream gets the YAML bytes verbatim —
		// preserving terminal newlines + indentation that some YAML
		// parsers care about.
		if strings.TrimSpace(in.ComposeYAML) == "" {
			return nil, InstallAppOutput{}, errors.New("compose_yaml is required")
		}
		yamlBody := in.ComposeYAML
		// Local validation first — the agent gets a clear "this YAML
		// is forbidden because X" before any upstream call. Reuses
		// the same composevalidator the standalone CLI exposes.
		result := composevalidator.Validate([]byte(yamlBody))
		if !result.OK {
			out := InstallAppOutput{
				Status:     "rejected_by_validator",
				DryRun:     in.DryRun,
				Violations: result.Violations,
			}
			// IsError signals "this didn't work" to the agent; the
			// structured violations payload is what the agent reads
			// to know *why*. Same shape as the apps_unavailable +
			// other typed errors so the agent's failure handler
			// doesn't need a tool-specific branch.
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: mustJSON(out)}},
			}, out, nil
		}
		if proxy == nil {
			return nil, InstallAppOutput{}, errors.New("apps_unavailable: coreproxy not configured")
		}

		path := "/v2/app_management/compose"
		if in.DryRun {
			path += "?dry_run=true"
		}
		// app-management's POST /v2/app_management/compose expects
		// Content-Type: application/yaml + the YAML body verbatim
		// (per its OpenAPI spec).
		upstream, err := proxy.RequestFrom(ctx, http.MethodPost, coreproxy.ServiceApps, path, "", []byte(yamlBody), "application/yaml")
		if err != nil {
			payload := coreproxy.AsErrorPayload(err)
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: string(payload)}},
			}, InstallAppOutput{Status: "upstream_error", DryRun: in.DryRun}, nil
		}
		status := "installed"
		if in.DryRun {
			status = "dry_run_passed"
		}
		return nil, InstallAppOutput{
			Status:       status,
			DryRun:       in.DryRun,
			UpstreamBody: string(upstream),
		}, nil
	})
}

// registerUninstallApp wires uninstall_app. Reuses the same
// validation discipline as restart_app for the id, plus DELETE
// verb support that landed in ADR-0046 batch 2's RequestFrom
// generalisation.
func registerUninstallApp(s *mcp.Server, proxy *coreproxy.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "uninstall_app",
		Description: "Uninstall a PowerLab app — removes its containers + (per app-management config) may remove its persistent data. DESTRUCTIVE — may cause data loss; not reversible without a backup. Use apps://list to discover valid ids.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in UninstallAppInput) (*mcp.CallToolResult, UninstallAppOutput, error) {
		id := strings.TrimSpace(in.ID)
		if id == "" {
			return nil, UninstallAppOutput{}, errors.New("id is required (see apps://list)")
		}
		if strings.ContainsAny(id, "/\\.?#") {
			return nil, UninstallAppOutput{}, fmt.Errorf("invalid app id %q (must match the apps://list manifest)", id)
		}
		if proxy == nil {
			return nil, UninstallAppOutput{}, errors.New("apps_unavailable: coreproxy not configured")
		}
		path := "/v2/app_management/compose/" + url.PathEscape(id)
		if _, err := proxy.RequestFrom(ctx, http.MethodDelete, coreproxy.ServiceApps, path, "", nil, ""); err != nil {
			payload := coreproxy.AsErrorPayload(err)
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: string(payload)}},
			}, UninstallAppOutput{ID: id, Status: "upstream_error"}, nil
		}
		return nil, UninstallAppOutput{ID: id, Status: "uninstall requested"}, nil
	})
}

// mustJSON marshals v to a JSON string; panics only if the type is
// genuinely un-encodable (which our typed structs are not). Used at
// the IsError content boundary where the agent reads a JSON-shaped
// text content.
func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		// Last-resort: surface the error itself so the agent at
		// least sees something parseable rather than empty content.
		return fmt.Sprintf(`{"error":"marshal_failed","detail":%q}`, err.Error())
	}
	return string(b)
}
