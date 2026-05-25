package ip_helper

import (
	"net"
	"testing"
)

// GetDeviceAllIPv4 must return a non-nil map of well-formed IPv4
// strings keyed by interface name, and never panic regardless of the
// host's interface set.
func TestGetDeviceAllIPv4(t *testing.T) {
	got := GetDeviceAllIPv4()
	if got == nil {
		t.Fatal("GetDeviceAllIPv4() returned a nil map")
	}
	for name, ip := range got {
		if name == "" {
			t.Errorf("empty interface name for ip %q", ip)
		}
		parsed := net.ParseIP(ip)
		if parsed == nil || parsed.To4() == nil {
			t.Errorf("interface %q has non-IPv4 address %q", name, ip)
		}
	}
}
