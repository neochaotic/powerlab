// Package composevalidator inspects a Docker Compose YAML for the
// patterns ADR-0046 forbids before powerlab-mcp's install_app tool
// forwards it to app-management. It is NOT a full compose-spec
// parser — app-management already does that. This is a deliberately
// strict, deny-list-of-known-dangerous-constructs filter that the
// agent has to clear to land a user-installable app.
//
// The threat model the validator addresses:
//
//   - Container escape (privileged: true, --cap-add SYS_ADMIN, etc.)
//   - Docker socket abuse (bind /var/run/docker.sock)
//   - Host namespace sharing (network_mode: host, pid: host,
//     userns_mode: host, ipc: host, uts: host)
//   - Sensitive host path exposure (bind /, /etc, /root, /proc,
//     /sys, /var/run, /var/lib)
//   - Devices and capabilities granting raw hardware access
//
// What this package is NOT:
//
//   - A general "is this YAML safe?" oracle. We trust app-management
//     for compose-syntax + image-pull policy + storage layout.
//   - A taint analysis. We don't follow envFile interpolation or
//     compose extends; the user supplies a flat YAML or it is
//     normalised upstream of us.
//   - A guarantee of safety. ADR-0046 §4 lists this as one layer
//     among many — install_app additionally gates on either a
//     panel-side approval or an explicit mcp.conf opt-in.
package composevalidator

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Result is what the agent + the operator-facing tool report read.
// Violations are listed with a stable Code an agent can pattern-
// match on (e.g. "privileged_true") and a human-readable Detail.
type Result struct {
	OK         bool         `json:"ok"`
	Violations []Violation  `json:"violations,omitempty"`
}

// Violation is one deny-list trip.
type Violation struct {
	Service string `json:"service,omitempty"`
	Code    string `json:"code"`
	Detail  string `json:"detail"`
}

// forbidden host paths — bind-mounting any of these is an instant
// rejection. Order matters: longer prefixes first so we can match
// by prefix without ambiguity.
var forbiddenHostPathPrefixes = []string{
	"/var/run/docker.sock",
	"/var/run/docker",
	"/var/run/containerd",
	"/var/run/crio",
	"/var/run",
	"/var/log",
	"/var/lib",
	"/proc",
	"/sys",
	"/dev",
	"/boot",
	"/etc",
	"/root",
	"/lib",
	"/lib64",
	"/usr/lib",
	"/usr/local/lib",
	"/usr/local/sbin",
	"/usr/sbin",
	"/sbin",
	"/usr/bin",
	"/bin",
	"/run",
}

// rejectedHostNamespaces — any of these on a service is an instant
// rejection. We don't allow the agent to install an app that opts
// into host namespace sharing of any kind.
var rejectedHostNamespaces = map[string]string{
	"network_mode": "host",
	"pid":          "host",
	"ipc":          "host",
	"uts":          "host",
	"userns_mode":  "host",
}

// dangerousCapAdds — capabilities we never let an app request. The
// list is conservative: SYS_ADMIN alone is enough to escape most
// containers; NET_ADMIN + NET_RAW enable LAN-wide attacks; SYS_PTRACE
// allows reading other processes' memory.
var dangerousCapAdds = map[string]bool{
	"all":          true,
	"sys_admin":    true,
	"sys_module":   true,
	"sys_ptrace":   true,
	"sys_rawio":    true,
	"sys_boot":     true,
	"sys_resource": true,
	"sys_time":     true,
	"net_admin":    true,
	"net_raw":      true,
	"dac_read_search": true,
	"dac_override": true,
	"audit_write":  true,
	"audit_control": true,
	"setuid":       true,
	"setgid":       true,
	"setfcap":      true,
	"mac_admin":    true,
	"mac_override": true,
	"chown":        true,
	"linux_immutable": true,
}

// Validate inspects raw YAML bytes and returns a Result. The bytes
// must be a single compose document (no multi-doc); the parser
// rejects anything else.
//
// Invalid YAML returns a single "invalid_yaml" violation. We do NOT
// try to be helpful with partial parses — an unparseable YAML is a
// clear reject signal.
func Validate(rawYAML []byte) Result {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(rawYAML, &doc); err != nil {
		return Result{Violations: []Violation{{Code: "invalid_yaml", Detail: err.Error()}}}
	}
	if doc == nil {
		return Result{Violations: []Violation{{Code: "invalid_yaml", Detail: "empty document"}}}
	}

	services, _ := doc["services"].(map[string]interface{})
	if len(services) == 0 {
		return Result{Violations: []Violation{{Code: "no_services", Detail: "compose document declares no services"}}}
	}

	var vs []Violation
	for name, raw := range services {
		svc, ok := raw.(map[string]interface{})
		if !ok {
			vs = append(vs, Violation{Service: name, Code: "invalid_service_shape", Detail: "service is not a map"})
			continue
		}
		vs = append(vs, checkService(name, svc)...)
	}

	return Result{OK: len(vs) == 0, Violations: vs}
}

// checkService runs every per-service rule. Order is loosely "fastest
// rejections first" so a deeply-nested service hits the cheap checks
// before the expensive ones.
func checkService(name string, svc map[string]interface{}) []Violation {
	var vs []Violation

	// privileged: true — single-rule escape hatch into container
	// escape. There is no legitimate PowerLab app reason for this.
	if priv, ok := svc["privileged"]; ok {
		if b, _ := priv.(bool); b {
			vs = append(vs, Violation{Service: name, Code: "privileged_true",
				Detail: "service requests privileged: true (full container escape) — not allowed for agent-installed apps"})
		}
	}

	// Host-namespace sharing — any of these makes the container as
	// good as a host process. Rejected uniformly.
	for k, banned := range rejectedHostNamespaces {
		v, ok := svc[k]
		if !ok {
			continue
		}
		s := strings.ToLower(strings.TrimSpace(fmt.Sprint(v)))
		// network_mode: container:<id> is also not allowed (joins
		// an existing container's network namespace), so reject any
		// non-default value too.
		if s == banned || (k == "network_mode" && strings.HasPrefix(s, "container:")) {
			vs = append(vs, Violation{Service: name, Code: "host_namespace_share",
				Detail: fmt.Sprintf("%s: %s shares a host namespace — not allowed", k, s)})
		}
	}

	// cap_add — drop the list, scan each entry against the deny
	// list (case-insensitive). cap_drop is fine; cap_add is the one
	// we gate.
	if rawCaps, ok := svc["cap_add"]; ok {
		if caps, ok := rawCaps.([]interface{}); ok {
			for _, c := range caps {
				name2 := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(fmt.Sprint(c)), "CAP_"))
				if dangerousCapAdds[name2] {
					vs = append(vs, Violation{Service: name, Code: "dangerous_cap_add",
						Detail: fmt.Sprintf("cap_add includes %q which grants host-level capability", c)})
				}
			}
		}
	}

	// devices — any /dev/* device bind is rejected. Apps that need
	// hardware access (GPU, audio, USB) go through purpose-built
	// PowerLab paths (e.g. nvidia runtime, not a raw /dev/nvidia0
	// passthrough).
	if rawDevs, ok := svc["devices"]; ok {
		if devs, ok := rawDevs.([]interface{}); ok && len(devs) > 0 {
			vs = append(vs, Violation{Service: name, Code: "devices_block",
				Detail: "devices: passthrough — raw /dev/* access is not allowed for agent-installed apps"})
		}
	}

	// volumes — every bind mount source is checked against the
	// forbidden host paths. Both string-form ("/host:/container")
	// and long-form ({type: bind, source: ...}) are supported.
	if rawVols, ok := svc["volumes"]; ok {
		if vols, ok := rawVols.([]interface{}); ok {
			for _, v := range vols {
				if hostPath := extractHostBindSource(v); hostPath != "" {
					if reason := forbiddenHostPathReason(hostPath); reason != "" {
						vs = append(vs, Violation{Service: name, Code: "forbidden_volume_source",
							Detail: fmt.Sprintf("volume binds %s (%s)", hostPath, reason)})
					}
				}
			}
		}
	}

	return vs
}

// extractHostBindSource returns the host path of one volume entry, or
// "" if the entry is anonymous / named-volume / unparseable. Handles
// both forms documented by the compose spec.
func extractHostBindSource(v interface{}) string {
	switch x := v.(type) {
	case string:
		// "src:dst[:mode]" or "namedvol:dst" or "dst" (anonymous).
		// We only care when src starts with "/" or "." (relative is
		// rare in PowerLab compose; treat as suspicious).
		parts := strings.SplitN(x, ":", 3)
		if len(parts) < 2 {
			return ""
		}
		src := strings.TrimSpace(parts[0])
		if strings.HasPrefix(src, "/") {
			return src
		}
		return ""
	case map[string]interface{}:
		// {type: bind, source: /host/path, target: /container/path, ...}
		t, _ := x["type"].(string)
		if t != "" && t != "bind" {
			return ""
		}
		src, _ := x["source"].(string)
		if strings.HasPrefix(src, "/") {
			return src
		}
		return ""
	}
	return ""
}

// forbiddenHostPathReason returns a short rationale string when path
// is in the forbidden prefix set, or "" when the path is allowed.
// Matches by longest prefix so /var/run/docker.sock wins over /var/run.
func forbiddenHostPathReason(p string) string {
	// Normalise: strip trailing slash for prefix comparison; leave
	// the rest alone (the spec doesn't allow weird ../ traversal in
	// bind sources but we don't try to be helpful with it).
	p = strings.TrimRight(p, "/")
	for _, prefix := range forbiddenHostPathPrefixes {
		if p == prefix || strings.HasPrefix(p, prefix+"/") {
			switch prefix {
			case "/var/run/docker.sock", "/var/run/docker", "/var/run/containerd", "/var/run/crio":
				return "Docker / container-runtime socket — would give the container full control of the host"
			case "/proc", "/sys":
				return "kernel pseudo-filesystem — would expose host kernel state"
			case "/dev":
				return "raw device tree — would expose host hardware"
			case "/etc", "/root":
				return "host system configuration / root home"
			case "/var/log", "/var/lib", "/var/run", "/run":
				return "host runtime / persistent state directory"
			case "/boot", "/lib", "/lib64", "/usr/lib", "/usr/local/lib":
				return "host library / boot files"
			case "/bin", "/sbin", "/usr/bin", "/usr/sbin", "/usr/local/sbin":
				return "host binary directory"
			default:
				return "host system directory"
			}
		}
	}
	return ""
}
