// Package config loads powerlab-mcp's runtime configuration from
// /etc/powerlab/mcp.conf, a flat `Key = value` file written by the
// installer. The parser is intentionally forgiving: a missing file
// yields defaults (the service must boot on a fresh box before the
// installer writes the conf), comments and blank lines are skipped,
// and unknown keys are ignored so a newer installer's keys never break
// an older binary.
package config

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"strings"

	"github.com/neochaotic/powerlab/backend/common/utils/constants"
)

// Config is powerlab-mcp's resolved runtime configuration.
type Config struct {
	// ListenAddr is the address the HTTP+SSE server binds to.
	// Bound to all interfaces by default so the surface is reachable
	// from the LAN; the auth tiers (not the bind) gate access.
	ListenAddr string

	// AuditDir is the directory holding per-service JSONL audit files
	// (audit.jsonl) that the audit:// resources tail read-only.
	// Matches the path the audit middleware writes to (ADR-0035).
	AuditDir string

	// RuntimePath is the PowerLab runtime directory where the
	// user-service publishes its address file. The read-tier auth gate
	// resolves the JWT public key (JWKS) from there to validate LAN
	// callers' tokens. Defaults to the platform runtime path.
	RuntimePath string

	// Disabled is the operator kill-switch. When true the binary logs
	// a single notice and exits 0 *before* binding the listener —
	// systemd records the start as successful and never restarts it
	// (Restart=always honours exit code semantics: a clean exit doesn't
	// retry-loop). This gives any homelab/enterprise operator a
	// surgical opt-out without `systemctl mask` or hand-editing the
	// unit file: flip Disabled=true in /etc/powerlab/mcp.conf, run
	// `systemctl restart powerlab-mcp`, MCP stays down. The unit can
	// be re-enabled by flipping it back. Default false (ship enabled).
	Disabled bool

	// OpenAPIDir holds the bundled OpenAPI specs served by docs://
	// (ADR-0044). One file per PowerLab service named "<svc>.yaml".
	// Populated by package-linux.sh; on a dev box without the install
	// in place the directory may be absent — docs://api returns an
	// empty manifest in that case (graceful, no error).
	OpenAPIDir string

	// SystemdSystemDir is where journal://units enumerates the
	// powerlab-<svc>.service unit files installed by install.sh.
	// Overridable so a sandbox can point this at a fixture.
	SystemdSystemDir string

	// EnableDestructiveTools is the operator opt-in for the
	// install_app and uninstall_app MCP tools (ADR-0046 batch 3).
	// When false (the default), these tools are NOT registered —
	// they don't show up in tools/list and an agent cannot call
	// them. When true, they appear and the agent can install or
	// uninstall apps autonomously, gated only by the composevalidator
	// deny-list (install_app) and the JWT auth on /mcp.
	//
	// ADR-0046 §4 names this knob as one acceptable gate alongside
	// the panel-side "pending agent action" approval UI (which is
	// roadmap). For the homelab dogfood / autonomous-agent path,
	// the operator flips this to true with full understanding that
	// an authenticated agent can now mutate app state without a
	// per-action confirmation. Documented threat model.
	EnableDestructiveTools bool

	// ConceptsDir holds the PowerLab mkdocs concept files exposed
	// via docs://concepts/{name} (ADR-0048). One file per concept
	// (compose-conventions.md, security-model.md, glossary.md, etc.).
	// Staged by package-linux.sh; missing dir means docs://concepts
	// returns an empty index (graceful, no error).
	ConceptsDir string

	// CatalogDir holds the community-catalog the install ships.
	// catalog://app/{id} reads each app's docker-compose.yml from
	// <CatalogDir>/Apps/<id>/docker-compose.yml; catalog://index
	// enumerates valid <id> subdirectories. Missing dir means
	// catalog://index returns an empty list.
	CatalogDir string

	// EnableSensitiveTier is the operator opt-in for the sensitive
	// sysadmin resources (ADR-0049): journal://system/auth and
	// journal://system/failures. When false (the default), these
	// resources are NOT registered — they do not appear in
	// resources/list and an agent has no URI to address; the surface
	// effectively does not exist.
	//
	// When true, an authenticated agent can read SSH attempt logs
	// (usernames + source IPs of every probe), sudo invocations
	// (who ran what privileged command, when), and login session
	// events. That is legitimate enterprise observability — and a
	// real exposure if the JWT is compromised. The MESSAGE field is
	// kept intact (operators need the actual log line to reason); a
	// `sudo command --password=hunter2` invocation that logs via
	// pam_unix's LOG_INFO path WILL surface that argument in MESSAGE.
	// Documented limit; operators flipping this knob accept it.
	//
	// Same single-switch-for-whole-tier semantics as
	// EnableDestructiveTools — per-resource gates would compound
	// operator confusion ("which combination shows what?") for no
	// threat-model gain.
	EnableSensitiveTier bool
}

// Default returns the configuration used when no conf file is present
// or for any key the conf omits.
func Default() Config {
	return Config{
		ListenAddr:             ":9090",
		AuditDir:               "/var/log/powerlab",
		RuntimePath:            constants.DefaultRuntimePath,
		Disabled:               false,
		OpenAPIDir:             "/usr/share/powerlab/openapi",
		SystemdSystemDir:       "/etc/systemd/system",
		EnableDestructiveTools: false,
		ConceptsDir:            "/usr/share/powerlab/docs/concepts",
		CatalogDir:             "/var/lib/powerlab/community-catalog",
		EnableSensitiveTier:    false,
	}
}

// Load overlays the `Key = value` pairs found in the file at path onto
// Default(). A missing file is not an error — it returns Default() so a
// fresh box boots before the installer has written the conf. Malformed
// lines (no '=') and unknown keys are skipped.
func Load(path string) (Config, error) {
	cfg := Default()

	// #nosec G304 -- path is the operator-supplied --conf flag (default
	// /etc/powerlab/mcp.conf), a trusted local path, not user input.
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue // malformed — skip, don't abort the load
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch strings.ToLower(key) {
		case "listenaddr":
			cfg.ListenAddr = val
		case "auditdir":
			cfg.AuditDir = val
		case "runtimepath":
			cfg.RuntimePath = val
		case "disabled":
			cfg.Disabled = parseBool(val)
		case "openapidir":
			cfg.OpenAPIDir = val
		case "systemdsystemdir":
			cfg.SystemdSystemDir = val
		case "enabledestructivetools":
			cfg.EnableDestructiveTools = parseBool(val)
		case "conceptsdir":
			cfg.ConceptsDir = val
		case "catalogdir":
			cfg.CatalogDir = val
		case "enablesensitivetier":
			cfg.EnableSensitiveTier = parseBool(val)
			// unknown keys: ignored on purpose (forward-compatible)
		}
	}
	if err := sc.Err(); err != nil {
		return Default(), err
	}
	return cfg, nil
}

// parseBool accepts the standard truthy strings (case-insensitive) for
// the kill-switch key. Anything else is false — operators flipping
// `Disabled = 1` or `Disabled = on` get the intuitive result, anything
// ambiguous defaults to "service runs" rather than silently stopping it.
func parseBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "t", "yes", "y", "1", "on":
		return true
	}
	return false
}
