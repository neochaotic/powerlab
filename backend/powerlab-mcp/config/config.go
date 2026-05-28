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
}

// Default returns the configuration used when no conf file is present
// or for any key the conf omits.
func Default() Config {
	return Config{
		ListenAddr:       ":9090",
		AuditDir:         "/var/log/powerlab",
		RuntimePath:      constants.DefaultRuntimePath,
		Disabled:         false,
		OpenAPIDir:       "/usr/share/powerlab/openapi",
		SystemdSystemDir: "/etc/systemd/system",
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
