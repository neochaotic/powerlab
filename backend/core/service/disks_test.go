package service

import (
	"testing"
)

// GetDisks must always return at least one mount (the host's root
// filesystem). The wire contract is `{physical, mounts}`; an agent
// reading system://disk reasonably assumes Mounts is populated on
// any UNIX-like host that PowerLab runs on.
//
// Physical is best-effort: smartctl is not installed in CI images
// and macOS dev hosts return zero SMART data; we therefore only
// assert the slice is non-nil (so JSON marshals to `[]` not `null`).
func TestGetDisks_RootMountAlwaysPresent(t *testing.T) {
	svc := &systemService{}

	info := svc.GetDisks()

	if info.Mounts == nil {
		t.Fatal("Mounts is nil — must be a non-nil slice so JSON marshals to []")
	}
	if info.Physical == nil {
		t.Fatal("Physical is nil — must be a non-nil slice so JSON marshals to []")
	}
	if len(info.Mounts) == 0 {
		t.Fatal("Mounts is empty — at least the root mount should always be present")
	}

	var rootSeen bool
	for _, m := range info.Mounts {
		if m.Path == "/" {
			rootSeen = true
			if m.Total == 0 {
				t.Errorf("root mount has Total=0 — gopsutil should never return that on a real filesystem")
			}
			// UsedPercent is rounded to 1 decimal; assert it's in
			// the legal range — anything outside [0,100] points at
			// the formatter being skipped on the new code path.
			if m.UsedPercent < 0 || m.UsedPercent > 100 {
				t.Errorf("root mount UsedPercent=%v out of [0,100]", m.UsedPercent)
			}
		}
	}
	if !rootSeen {
		t.Errorf("no entry for root path '/' in mounts: %+v", info.Mounts)
	}
}

// roundPercent is a pure helper — locks the 1-decimal rounding the
// pre-existing GetDiskInfo used so MCP/dashboard format isn't drift-
// prone between the two code paths.
func TestRoundPercent_OneDecimal(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		{12.345, 12.3},
		{12.355, 12.4},
		{0, 0},
		{100, 100},
		{99.99, 100.0},
	}
	for _, tc := range cases {
		got := roundPercent(tc.in)
		if got != tc.want {
			t.Errorf("roundPercent(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
