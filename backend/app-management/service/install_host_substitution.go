package service

import (
	"net"
	"os"
	"regexp"
	"strings"
)

// SubstituteHostPlaceholders resolves the Umbrel host-identity
// placeholders that appear URL-embedded inside env-var values:
//
//   - ${DEVICE_DOMAIN_NAME}    (umbrel: app's reachable .local hostname)
//   - ${DEVICE_HOSTNAME}       (umbrel: host's system hostname)
//   - ${APP_DOMAIN}            (umbrel: per-app domain override)
//   - ${APP_<NAME>_LOCAL_IPS}  (umbrel: list of LAN IPs to accept)
//
// At sync-time these are deliberately preserved (sync-catalog's
// substituteHostValidationEnvVars only touches pure list-form values
// — substituting URL-embedded with `*` produces invalid URLs like
// `http://*:8770`). At install time we KNOW the operator's actual
// reachable host (from the install request's Host header), so we
// substitute with that.
//
// Without this fix, 44 catalog apps (50 URL refs) crash with
// "Invalid URI" or equivalent — Laravel's Request::create rejects
// `http://:8770`; Django's CSRF_TRUSTED_ORIGINS rejects the same;
// SvelteKit returns 400 "Bad request" for empty host.
//
// Resolution order for the substitution value:
//
//  1. hostHint (typically the install request's `Host:` header, port
//     stripped). This is the operator's chosen address — guaranteed
//     correct for that operator.
//  2. First non-loopback IPv4 from `net.Interfaces()` — works for LAN
//     access even when the request didn't carry a Host header.
//  3. `os.Hostname()` + `.local` — works iff mDNS resolves it.
//     Last-resort fallback.
//
// The function is a pure byte transform — read placeholders, replace,
// return. No I/O beyond the system-info reads.
var hostPlaceholderRE = regexp.MustCompile(`\$\{(DEVICE_DOMAIN_NAME|DEVICE_HOSTNAME|APP_DOMAIN|APP_[A-Z0-9_]+_LOCAL_IPS)\}`)

// SubstituteHostPlaceholders is the public entry point. See file doc.
func SubstituteHostPlaceholders(yaml []byte, hostHint string) []byte {
	if !hostPlaceholderRE.Match(yaml) {
		return yaml
	}
	host := resolveHost(hostHint)
	if host == "" {
		// Truly nothing resolved — leave the YAML alone rather than
		// substitute the empty string and produce `http://:8770`.
		return yaml
	}
	return hostPlaceholderRE.ReplaceAll(yaml, []byte(host))
}

// resolveHost picks the best host string to use for the substitution.
// Tries the explicit hint first, then a LAN IP probe, then the system
// hostname + ".local". Returns "" when every path fails (rare —
// usually the LAN IP probe succeeds even on container-only hosts).
func resolveHost(hint string) string {
	// 1. Caller-supplied (typically request Host header).
	if h := stripPort(hint); h != "" {
		return h
	}
	// 2. First non-loopback IPv4 from any UP interface.
	if ip := firstLanIPv4(); ip != "" {
		return ip
	}
	// 3. System hostname + .local (mDNS dependency).
	if hn, err := os.Hostname(); err == nil && hn != "" {
		return hn + ".local"
	}
	return ""
}

// stripPort drops a trailing ":port" if present. Returns input
// unchanged if there's no colon. Empty input → empty output (no
// panic). Defensive against IPv6-bracketed forms.
func stripPort(s string) string {
	if s == "" {
		return ""
	}
	// IPv6 with brackets: [::1]:8765 → ::1
	if strings.HasPrefix(s, "[") {
		if end := strings.Index(s, "]"); end >= 0 {
			return s[1:end]
		}
		return s
	}
	// IPv4 / hostname with colon-port: 192.168.1.1:8765 → 192.168.1.1.
	// A bare IPv6 (multiple colons, no brackets) trips LastIndex, so
	// only strip when the suffix is purely digits.
	if i := strings.LastIndex(s, ":"); i >= 0 {
		port := s[i+1:]
		allDigits := port != ""
		for _, c := range port {
			if c < '0' || c > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			return s[:i]
		}
	}
	return s
}

// firstLanIPv4 probes net.Interfaces() for the first non-loopback,
// up IPv4 address. Deterministic across runs of the same process
// (sorted by interface name) — Sprint 13 audit caught random
// iteration order causing the "main service" field to flip between
// catalog refreshes; same risk class here.
func firstLanIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	names := make([]string, 0, len(ifaces))
	byName := map[string]net.Interface{}
	for _, i := range ifaces {
		names = append(names, i.Name)
		byName[i.Name] = i
	}
	// Bubble sort — small N, no extra import for "sort".
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[j] < names[i] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	for _, n := range names {
		ifc := byName[n]
		if ifc.Flags&net.FlagUp == 0 {
			continue
		}
		if ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipNet.IP.To4()
			if ip == nil {
				continue
			}
			return ip.String()
		}
	}
	return ""
}
