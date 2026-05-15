package service

import "testing"

// Regression suite for the orphan-cleanup bug class (Sprint 17 C1).
//
// Repro that surfaced live on .142 with 2fauth reinstall on v0.6.11:
//   - User installs 2fauth, uninstalls, reinstalls
//   - First install attempt of the second cycle fails with
//     `Error during creation: Error response from daemon: No such
//     container: <sha256>`
//   - Retry succeeds because the failed attempt's cleanup catches
//     the orphan; user is left with a confusing "click twice" UX
//
// Root cause: docker auto-renames a name-conflicting container to
// `<12-hex>_<original-name>` when a recreate runs. compose-go's
// `RemoveOrphans: true` doesn't catch that pattern because the
// renamed container's compose.project label still matches but the
// service-name field doesn't. The recreate then races with the
// stale orphan and compose-go references its now-dead container ID.

func TestIsAutoRenamedOrphan_MatchesDockerPattern(t *testing.T) {
	tests := []struct {
		name    string
		project string
		input   string
		want    bool
	}{
		// The actual sample from the .142 incident:
		{"2fauth incident sample", "2fauth", "aed494cc4d0d_2fauth", true},

		// Other plausible auto-renames Docker emits:
		{"all hex chars", "blinko", "0123456789ab_blinko", true},
		{"upper-hex digits ARE rejected (Docker uses lowercase)",
			"blinko", "ABCDEF123456_blinko", false},

		// Negative cases — must NOT trigger a remove:
		{"normal compose container", "2fauth", "2fauth", false},
		{"compose multi-service container", "blinko", "blinko-db-1", false},
		{"compose multi-service with project prefix", "2fauth", "2fauth-app-1", false},

		// Edge cases that look hex-like but aren't 12 chars:
		{"too few hex chars", "2fauth", "abc1234_2fauth", false},
		{"too many hex chars", "2fauth", "abcdef1234567_2fauth", false},

		// Edge cases of underscore positioning:
		{"hex prefix with NO underscore", "2fauth", "abcdef1234562fauth", false},
		{"hex prefix with double underscore", "2fauth", "abcdef123456__2fauth", false},

		// Project-name suffix mismatch (should not match — different project):
		{"different project after the prefix", "2fauth", "abcdef123456_blinko", false},

		// Hex-like project names — Docker does emit these:
		{"hex-named project", "ab12cd34", "abcdef123456_ab12cd34", true},

		// Empty / pathological:
		{"empty input", "2fauth", "", false},
		{"empty project", "", "abcdef123456_", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAutoRenamedOrphan(tt.input, tt.project)
			if got != tt.want {
				t.Errorf("isAutoRenamedOrphan(%q, project=%q) = %v, want %v",
					tt.input, tt.project, got, tt.want)
			}
		})
	}
}
