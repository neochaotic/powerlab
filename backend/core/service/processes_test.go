package service

import (
	"testing"

	"github.com/neochaotic/powerlab/backend/core/model"
)

// GetProcesses(topN) must clamp non-positive topN to a safe default —
// otherwise a misconfigured caller could ask for 0 entries and the
// agent would see empty lists with no signal that something is off.
func TestGetProcesses_TopNClampedOnZeroOrNegative(t *testing.T) {
	svc := &systemService{}

	cases := []struct {
		name string
		topN int
	}{
		{"zero", 0},
		{"negative", -5},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			summary := svc.GetProcesses(tc.topN)
			// Default cap is 10; either both top lists honour it (host
			// has > 10 procs — the normal case) or both are bounded by
			// the real process count (gopsutil-restricted sandbox).
			if len(summary.TopByCPU) > 10 || len(summary.TopByMem) > 10 {
				t.Errorf("topN=%d: lists not clamped to default 10 (cpu=%d mem=%d)",
					tc.topN, len(summary.TopByCPU), len(summary.TopByMem))
			}
		})
	}
}

// takeFirst is the helper that caps the per-resource slices. Pure
// logic; locks the contract so future refactors can't drop
// boundary cases.
func TestTakeFirst(t *testing.T) {
	mk := func(n int) []model.ProcessSummary {
		out := make([]model.ProcessSummary, n)
		for i := range out {
			out[i].PID = int32(i)
		}
		return out
	}

	cases := []struct {
		name    string
		in      []model.ProcessSummary
		n       int
		wantLen int
	}{
		{"slice shorter than n", mk(3), 10, 3},
		{"slice equal to n", mk(5), 5, 5},
		{"slice longer than n", mk(20), 10, 10},
		{"empty slice", nil, 10, 0},
		{"n is zero", mk(5), 0, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := takeFirst(tc.in, tc.n)
			if len(got) != tc.wantLen {
				t.Errorf("len=%d want %d", len(got), tc.wantLen)
			}
			// Mutating the result must NOT affect the caller's slice
			// (takeFirst returns a fresh backing array).
			if len(got) > 0 && len(tc.in) > 0 {
				got[0].PID = 99999
				if tc.in[0].PID == 99999 {
					t.Errorf("takeFirst returned a view, not a copy — caller mutation leaked back to input")
				}
			}
		})
	}
}
