package service

import "testing"

// isSystemPath gates whether install-time volume prep will mkdir a bind
// source. Kernel/OS-owned mount points (/dev/net/tun for Tailscale,
// /proc, /sys, /run, /host) must be reported true so we skip mkdir and
// let Docker surface its own error — mkdir'ing them fails with EPERM and
// is never correct. The boundary matters: a path that merely shares a
// prefix with a system root ("/devices", "/runner") is a normal path and
// MUST be treated as non-system, or we'd silently skip preparing a real
// app data dir.
func TestIsSystemPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		// exact system roots
		{"/dev", true},
		{"/proc", true},
		{"/sys", true},
		{"/run", true},
		{"/host", true},
		// descendants of system roots
		{"/dev/net/tun", true},
		{"/proc/1/mem", true},
		{"/sys/fs/cgroup", true},
		{"/run/docker.sock", true},
		{"/host/var/lib", true},
		// prefix-but-not-subdir boundary — these are NOT system paths
		{"/devices", false},
		{"/dev-shm", false},
		{"/runner/work", false},
		{"/system", false},
		{"/proceeds", false},
		// ordinary app data paths
		{"/DATA/PowerLabAppData/booklore/data", false},
		{"/var/lib/powerlab", false},
		{"/home/user/dev", false},
		// degenerate inputs
		{"", false},
		{"/", false},
		{"dev/net/tun", false}, // relative, not anchored at a system root
	}
	for _, c := range cases {
		if got := isSystemPath(c.path); got != c.want {
			t.Errorf("isSystemPath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}
