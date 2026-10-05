package service

import (
	"errors"
	"net"
	"testing"
)

// stubHostProbes swaps the netInterfaces/osHostname seams for the
// duration of a test. Not safe with t.Parallel (package-level vars).
func stubHostProbes(t *testing.T, ifaces func() ([]net.Interface, error), hostname func() (string, error)) {
	t.Helper()
	origIfaces, origHostname := netInterfaces, osHostname
	t.Cleanup(func() { netInterfaces, osHostname = origIfaces, origHostname })
	netInterfaces = ifaces
	osHostname = hostname
}

func ifacesErr() ([]net.Interface, error) { return nil, errors.New("netlink unavailable") }

func hostnameErr() (string, error) { return "", errors.New("no hostname") }

func hostnameOf(name string) func() (string, error) {
	return func() (string, error) { return name, nil }
}

func ifacesOf(list ...net.Interface) func() ([]net.Interface, error) {
	return func() ([]net.Interface, error) { return list, nil }
}

// loopbackAsLAN returns the host's loopback interface with its flags
// rewritten to look like an up, non-loopback NIC. Its Addrs() still
// come from the kernel (127.0.0.1 and, where enabled, ::1), which lets
// a test drive firstLanIPv4's address-selection arm deterministically.
func loopbackAsLAN(t *testing.T, name string) net.Interface {
	t.Helper()
	all, err := net.Interfaces()
	if err != nil {
		t.Skipf("net.Interfaces unavailable: %v", err)
	}
	for _, ifc := range all {
		if ifc.Flags&net.FlagLoopback != 0 {
			ifc.Name = name
			ifc.Flags = net.FlagUp
			return ifc
		}
	}
	t.Skip("no loopback interface on this host")
	return net.Interface{}
}

// noAddrIface is an up, non-loopback interface whose index matches no
// kernel interface, so Addrs() returns no addresses.
func noAddrIface(name string) net.Interface {
	return net.Interface{Index: 1 << 30, Name: name, Flags: net.FlagUp}
}

func TestFirstLanIPv4_InterfacesError(t *testing.T) {
	stubHostProbes(t, ifacesErr, hostnameErr)
	if got := firstLanIPv4(); got != "" {
		t.Errorf("firstLanIPv4() = %q, want \"\" when net.Interfaces fails", got)
	}
}

func TestFirstLanIPv4_NoUsableInterface(t *testing.T) {
	stubHostProbes(t, ifacesOf(
		net.Interface{Index: 1 << 30, Name: "eth0"},                                     // down
		net.Interface{Index: 1 << 30, Name: "lo", Flags: net.FlagUp | net.FlagLoopback}, // loopback
		noAddrIface("wlan0"), // up, no addresses
	), hostnameErr)
	if got := firstLanIPv4(); got != "" {
		t.Errorf("firstLanIPv4() = %q, want \"\" with no up non-loopback IPv4", got)
	}
}

func TestFirstLanIPv4_EmptyList(t *testing.T) {
	stubHostProbes(t, ifacesOf(), hostnameErr)
	if got := firstLanIPv4(); got != "" {
		t.Errorf("firstLanIPv4() = %q, want \"\" with no interfaces", got)
	}
}

func TestFirstLanIPv4_PicksIPv4FromSortedUpInterface(t *testing.T) {
	lan := loopbackAsLAN(t, "eth1")
	// Listed out of order: "eth0" sorts first but has no addresses, so
	// the probe must move on to "eth1" and return its IPv4 address
	// (skipping ::1 if the kernel lists it).
	stubHostProbes(t, ifacesOf(lan, noAddrIface("eth0")), hostnameErr)
	if got := firstLanIPv4(); got != "127.0.0.1" {
		t.Errorf("firstLanIPv4() = %q, want 127.0.0.1", got)
	}
}

func TestResolveHost_Fallbacks(t *testing.T) {
	cases := []struct {
		name     string
		hint     string
		ifaces   func() ([]net.Interface, error)
		hostname func() (string, error)
		want     string
	}{
		{"hint wins over probes", "nas.lan:8765", ifacesErr, hostnameOf("box"), "nas.lan"},
		{"hostname.local when no LAN IP", "", ifacesErr, hostnameOf("box"), "box.local"},
		{"empty when hostname errors", "", ifacesErr, hostnameErr, ""},
		{"empty when hostname is empty", "", ifacesOf(), hostnameOf(""), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stubHostProbes(t, tc.ifaces, tc.hostname)
			if got := resolveHost(tc.hint); got != tc.want {
				t.Errorf("resolveHost(%q) = %q, want %q", tc.hint, got, tc.want)
			}
		})
	}
}

func TestResolveHost_LanIPBeforeHostname(t *testing.T) {
	lan := loopbackAsLAN(t, "eth0")
	stubHostProbes(t, ifacesOf(lan), hostnameOf("box"))
	if got := resolveHost(""); got != "127.0.0.1" {
		t.Errorf("resolveHost(\"\") = %q, want the LAN IP 127.0.0.1", got)
	}
}

func TestSubstituteHostPlaceholders_HostnameFallback(t *testing.T) {
	stubHostProbes(t, ifacesErr, hostnameOf("box"))
	in := []byte("APP_URL: http://${DEVICE_DOMAIN_NAME}:8770\n")
	want := "APP_URL: http://box.local:8770\n"
	if got := string(SubstituteHostPlaceholders(in, "")); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSubstituteHostPlaceholders_LeavesYAMLWhenNothingResolves(t *testing.T) {
	stubHostProbes(t, ifacesErr, hostnameErr)
	in := []byte("APP_URL: http://${DEVICE_DOMAIN_NAME}:8770\n")
	if got := string(SubstituteHostPlaceholders(in, "")); got != string(in) {
		t.Errorf("expected YAML unchanged when no host resolves, got %q", got)
	}
}
