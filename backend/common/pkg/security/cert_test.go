package security

import (
	"net"
	"testing"
)

// TestShouldIncludeIP nails down the SAN-classification rules
// from ADR 0001. Specifically, it pins down the CGNAT gate added
// in the same commit as this test: 100.64/10 is allowed ONLY when
// the carrying interface looks like a mesh-VPN tunnel (tailscale*
// on Linux, utun* on macOS). A regression that re-broadens it
// would let any VPN-client utun address leak into the SAN; a
// regression that narrows it further would lock mesh-VPN users
// out after they confirm trust on LAN.
func TestShouldIncludeIP(t *testing.T) {
	cases := []struct {
		name      string
		iface     string
		ip        string
		want      bool
	}{
		// RFC1918 — always allowed regardless of iface name
		{"10/8 on eth0", "eth0", "10.0.0.5", true},
		{"172.16/12 on en0", "en0", "172.16.42.1", true},
		{"172.31 boundary on en0", "en0", "172.31.255.254", true},
		{"172.32 just outside", "en0", "172.32.0.1", false},
		{"192.168/16 on wlan0", "wlan0", "192.168.1.10", true},

		// CGNAT — allowed only on mesh-VPN-shaped ifaces
		{"CGNAT on tunnel iface (Linux)", "tailscale0", "100.100.0.5", true},
		{"CGNAT on utun4 (macOS mesh-VPN)", "utun4", "100.64.0.1", true},
		{"CGNAT upper bound on utun7", "utun7", "100.127.255.254", true},
		{"CGNAT just outside upper", "tailscale0", "100.128.0.1", false},
		{"CGNAT on eth0 (NOT a tunnel)", "eth0", "100.100.0.5", false},
		{"CGNAT on en0 (NOT a tunnel)", "en0", "100.100.0.5", false},

		// IPv6 ULA fc00::/7
		{"ULA fd00:: on eth0", "eth0", "fd00::1", true},
		{"ULA fcff:: on en0", "en0", "fcff::1", true},
		{"GUA 2001:db8:: rejected", "eth0", "2001:db8::1", false},
		{"link-local fe80:: rejected", "eth0", "fe80::1", false},

		// Public IPv4 always rejected
		{"public 8.8.8.8 rejected", "eth0", "8.8.8.8", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ip := net.ParseIP(c.ip)
			if ip == nil {
				t.Fatalf("invalid test IP: %s", c.ip)
			}
			got := ShouldIncludeIP(c.iface, ip)
			if got != c.want {
				t.Errorf("ShouldIncludeIP(%q, %s) = %v, want %v", c.iface, c.ip, got, c.want)
			}
		})
	}
}
