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
}

// Default returns the configuration used when no conf file is present
// or for any key the conf omits.
func Default() Config {
	return Config{
		ListenAddr:  ":9090",
		AuditDir:    "/var/log/powerlab",
		RuntimePath: constants.DefaultRuntimePath,
	}
}

// Load overlays the `Key = value` pairs found in the file at path onto
// Default(). A missing file is not an error — it returns Default() so a
// fresh box boots before the installer has written the conf. Malformed
// lines (no '=') and unknown keys are skipped.
func Load(path string) (Config, error) {
	cfg := Default()

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
			// unknown keys: ignored on purpose (forward-compatible)
		}
	}
	if err := sc.Err(); err != nil {
		return Default(), err
	}
	return cfg, nil
}
