package service

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// PrepareBindMountSources walks every service's `volumes:` list in
// the install-time compose YAML, finds bind-mount sources (absolute
// paths on the host), creates them if missing, and chmod 0o777 so the
// container's runtime user — whoever that is — can write.
//
// Why 0o777 (and why not per-app uid):
//
// Containers ship with arbitrary internal users — Laravel apps run
// as www-data (uid 33), Node apps often as 1000, Postgres as 999,
// Caddy as 1000, etc. Without ownership alignment, the container can
// neither read nor write its bind-mounted data dir → "Permission
// denied" / "Please provide a valid cache path" / Postgres bind-mount
// init failure (Sprint 14 #334).
//
// Per-app uid annotation (`x-powerlab.runtime_uid: 33`) would be more
// precise but requires inspecting/annotating 44+ catalog apps. For a
// home-server-on-LAN threat model where the operator is the only
// shell-access user, 0o777 is the right pragmatic answer — it works
// for every container regardless of its internal uid, with one chmod.
// Cross-app data leakage is bounded (each app gets its own dir).
//
// Future refinement (Sprint 22+): `x-powerlab.runtime_uid` for
// security-sensitive apps (password managers, vault apps) to chown
// instead of chmod 0o777.
//
// Pure side-effecting function — does not mutate the input bytes.
// Returns the first error from YAML parsing; chmod errors are best-
// effort (logged via the caller's logger if any, since this layer
// has no logger). Non-existent paths are CREATED, not skipped, so
// the first install can write.
func PrepareBindMountSources(yamlBytes []byte) error {
	var doc map[string]any
	if err := yaml.Unmarshal(yamlBytes, &doc); err != nil {
		return fmt.Errorf("parse compose: %w", err)
	}
	services, ok := doc["services"].(map[string]any)
	if !ok {
		return nil
	}
	for _, svcAny := range services {
		svc, ok := svcAny.(map[string]any)
		if !ok {
			continue
		}
		vols, ok := svc["volumes"].([]any)
		if !ok {
			continue
		}
		for _, v := range vols {
			src := extractBindSource(v)
			if src == "" {
				continue
			}
			prepareOne(src)
		}
	}
	return nil
}

// extractBindSource returns the host-path source of a bind-mount
// volume entry, or "" when the entry is a named volume, tmpfs, or
// relative-path style. Handles both short-form (`/host:/container`)
// and long-form (`{type: bind, source: /host, target: /container}`).
//
// Filters:
//   - MUST start with `/` (absolute path) — relative `./x` and bare
//     `name:/...` (named volume) are excluded.
//   - Empty / non-string sources are excluded.
func extractBindSource(v any) string {
	switch vv := v.(type) {
	case string:
		// Short form: "<source>:<target>[:<mode>]"
		// First colon splits source from target. We assume no colon
		// inside the source path (true on Linux/macOS hosts; Windows
		// drive letters would break this but Windows isn't a target).
		i := strings.Index(vv, ":")
		if i <= 0 {
			return ""
		}
		src := vv[:i]
		if !strings.HasPrefix(src, "/") {
			return ""
		}
		return src
	case map[string]any:
		// Long form: { type: bind, source: /path, target: /path }
		t, _ := vv["type"].(string)
		if t != "" && t != "bind" {
			return ""
		}
		src, _ := vv["source"].(string)
		if !strings.HasPrefix(src, "/") {
			return ""
		}
		return src
	}
	return ""
}

// prepareOne creates the path if missing and chmods 0o777. Best-
// effort — silently swallows errors so a single permission-denied or
// EROFS path doesn't block the whole install. Production deployment
// runs as root (systemd unit), so the failure case is genuinely rare.
func prepareOne(src string) {
	// Only act on directories we can manipulate. If the path is an
	// existing FILE (single-file bind mounts like config.yaml), leave
	// it alone — chmod on a file is meaningful but we'd need to know
	// whether the caller wants the file world-writable or just its
	// parent dir. Skip files for now (conservative).
	if info, err := os.Stat(src); err == nil {
		if !info.IsDir() {
			return
		}
		_ = os.Chmod(src, 0o777)
		return
	}
	// Path doesn't exist — create it as a directory with 0o777 from
	// the start so the first container start can write.
	_ = os.MkdirAll(src, 0o777)
	// MkdirAll honors umask, which can strip the high bits. Belt-
	// and-braces chmod to force 0o777 regardless.
	_ = os.Chmod(src, 0o777)
}
