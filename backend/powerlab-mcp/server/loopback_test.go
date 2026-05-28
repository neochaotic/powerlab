package server

import "testing"

// isLoopbackAddr decides whether the proxy-guard grants loopback trust,
// so it must recognise every loopback form — including IPv6 "[::1]:port",
// which the previous manual ':'-split mishandled (it left the brackets
// and never matched "::1").
func TestIsLoopbackAddr(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:9090", true},
		{"[::1]:9090", true},
		{"::1", true},       // host with no port
		{"127.0.0.1", true}, // host with no port
		{"192.0.2.10:5000", false},
		{"[2001:db8::1]:443", false},
		{"10.0.0.5:22", false},
	}
	for _, c := range cases {
		if got := isLoopbackAddr(c.addr); got != c.want {
			t.Errorf("isLoopbackAddr(%q) = %v; want %v", c.addr, got, c.want)
		}
	}
}
