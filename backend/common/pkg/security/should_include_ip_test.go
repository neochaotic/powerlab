package security_test

import (
	"net"
	"testing"

	"github.com/neochaotic/powerlab/backend/common/pkg/security"
)

// TestShouldIncludeIP locks the cert-SAN IP classification rules (ADR-0001).
// A wrong answer here is a security issue — either leaking an arbitrary VPN
// peer address into the cert, or omitting a legitimate LAN address. The
// boundaries (172.16–31, 100.64–127, ULA fc/fd vs link-local fe80) are the
// mutation-prone spots, so they get explicit cases.
func TestShouldIncludeIP(t *testing.T) {
	cases := []struct {
		name  string
		iface string
		ip    string
		want  bool
	}{
		// RFC1918 — always included, any interface.
		{"10/8", "eth0", "10.0.0.1", true},
		{"172.16 low boundary", "eth0", "172.16.0.0", true},
		{"172.31 high boundary", "eth0", "172.31.255.255", true},
		{"172.15 just below", "eth0", "172.15.255.255", false},
		{"172.32 just above", "eth0", "172.32.0.1", false},
		{"192.168", "eth0", "192.168.1.1", true},
		{"192.167 just below", "eth0", "192.167.1.1", false},
		{"192.169 just above", "eth0", "192.169.1.1", false},

		// CGNAT 100.64.0.0/10 — only on mesh-VPN interfaces.
		{"cgnat on tailscale", "tailscale0", "100.64.0.1", true},
		{"cgnat on utun", "utun3", "100.100.100.100", true},
		{"cgnat on eth0 (reject)", "eth0", "100.64.0.1", false},
		{"cgnat low boundary off-mesh", "eth0", "100.64.0.0", false},
		{"100.63 just below cgnat", "tailscale0", "100.63.255.255", false},
		{"100.128 just above cgnat", "tailscale0", "100.128.0.1", false},

		// Public IPv4 — never.
		{"public", "eth0", "8.8.8.8", false},

		// IPv6: ULA fc00::/7 included; link-local / global excluded.
		{"ula fc00", "eth0", "fc00::1", true},
		{"ula fd00", "eth0", "fd12:3456::1", true},
		{"link-local fe80", "eth0", "fe80::1", false},
		{"global v6", "eth0", "2001:db8::1", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ip := net.ParseIP(c.ip)
			if ip == nil {
				t.Fatalf("bad test IP %q", c.ip)
			}
			if got := security.ShouldIncludeIP(c.iface, ip); got != c.want {
				t.Errorf("ShouldIncludeIP(%q, %s) = %v, want %v", c.iface, c.ip, got, c.want)
			}
		})
	}

	// nil IP must not panic and must be excluded.
	if security.ShouldIncludeIP("eth0", nil) {
		t.Error("ShouldIncludeIP(nil) should be false")
	}
}
