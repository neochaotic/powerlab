package service

import (
	"net"
	"testing"
)

// isLANRange is the reason mDNS used to advertise Docker bridges and
// Tailscale CGNAT addresses on Linux hosts. It is the one piece of mdns
// logic that cannot be smoke-tested with `dns-sd` from a Mac, so we
// pin its decisions here. If you change the function, change this
// table — it is intentional regression bait.
func TestIsLANRange(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		// RFC 1918 (the whole point — these MUST advertise)
		{"192.168.1.10", true},
		{"192.168.255.255", true},
		{"10.0.0.1", true},
		{"10.255.255.254", true},
		{"172.16.0.1", true},
		{"172.31.255.254", true},

		// Just outside RFC 1918 (off-by-one regression bait)
		{"172.15.0.1", false},
		{"172.32.0.1", false},
		{"193.168.0.1", false}, // typo of 192.168
		{"11.0.0.1", false},

		// Public internet — never advertise
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"203.0.113.1", false}, // documentation prefix

		// Tailscale CGNAT 100.64/10 — deliberately NOT advertised
		// (see comment on lanIPs in mdns.go)
		{"100.64.0.1", false},
		{"100.92.66.20", false}, // a Tailscale IP we observed during the v0.2 review session

		// Loopback (handled separately by the FlagLoopback bit, but
		// belt-and-braces — must not advertise)
		{"127.0.0.1", false},

		// Link-local (handled separately by IsLinkLocalUnicast, but
		// must not advertise either)
		{"169.254.1.1", false},

		// IPv6 ULA fc00::/7 — advertise
		{"fd00::1", true},
		{"fc00::1", true},

		// IPv6 link-local fe80::/10 — must NOT advertise
		{"fe80::1", false},

		// IPv6 GUA — public, must NOT advertise
		{"2001:db8::1", false},

		// IPv6 loopback
		{"::1", false},
	}

	for _, c := range cases {
		t.Run(c.ip, func(t *testing.T) {
			ip := net.ParseIP(c.ip)
			if ip == nil {
				t.Fatalf("test data error: %q does not parse", c.ip)
			}
			got := isLANRange(ip)
			if got != c.want {
				t.Fatalf("isLANRange(%q) = %v, want %v", c.ip, got, c.want)
			}
		})
	}
}
