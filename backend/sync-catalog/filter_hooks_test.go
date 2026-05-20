package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAppHasUnsupportedHookArtifacts locks ADR-0038 Sprint 22 PR 1:
// any catalog app that ships `hooks/` directory or `exports.sh` file
// is hard-rejected at sync time. PowerLab never executes upstream
// bash; apps that need it never land in PowerLab's catalog.
//
// The Stalwart example (`hooks/pre-start` running `curl | sh`)
// motivates this gate. Without it, every catalog sync is a remote-
// code-execution supply-chain channel.
func TestAppHasUnsupportedHookArtifacts(t *testing.T) {
	tmp := t.TempDir()

	// helper: create an app dir with optional artifacts.
	mkApp := func(id string, withHooks, withExports bool) {
		appDir := filepath.Join(tmp, id)
		if err := os.MkdirAll(appDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if withHooks {
			hookDir := filepath.Join(appDir, "hooks")
			if err := os.MkdirAll(hookDir, 0o755); err != nil {
				t.Fatal(err)
			}
			// Place a sentinel script so the dir isn't empty (matches
			// real catalog state).
			if err := os.WriteFile(filepath.Join(hookDir, "pre-start"), []byte("#!/bin/sh\n"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if withExports {
			if err := os.WriteFile(filepath.Join(appDir, "exports.sh"), []byte("export X=1\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}

	mkApp("clean-app", false, false)
	mkApp("hook-bearing-app", true, false)
	mkApp("exports-only-app", false, true)
	mkApp("hostile-app", true, true)

	cases := []struct {
		appID            string
		wantHasArtifacts bool
		wantReason       string // substring expected in reason when artifacts present
	}{
		{"clean-app", false, ""},
		{"hook-bearing-app", true, "hooks/"},
		{"exports-only-app", true, "exports.sh"},
		{"hostile-app", true, "hooks/"}, // hooks check fires first
		{"nonexistent-app", false, ""},  // missing dir → no artifacts, no panic
	}

	for _, tc := range cases {
		t.Run(tc.appID, func(t *testing.T) {
			has, reason := appHasUnsupportedHookArtifacts(tmp, tc.appID)
			if has != tc.wantHasArtifacts {
				t.Errorf("appID=%s: got hasArtifacts=%v, want %v", tc.appID, has, tc.wantHasArtifacts)
			}
			if tc.wantReason != "" && !contains(reason, tc.wantReason) {
				t.Errorf("appID=%s: expected reason to contain %q, got %q", tc.appID, tc.wantReason, reason)
			}
			if !tc.wantHasArtifacts && reason != "" {
				t.Errorf("appID=%s: expected empty reason when no artifacts, got %q", tc.appID, reason)
			}
		})
	}
}

func TestAppHasUnsupportedHookArtifacts_IgnoresEmptyHooksDir(t *testing.T) {
	// Edge case: a `hooks/` dir that exists but is EMPTY still counts
	// as "ships hooks" — the presence of the dir signals intent to
	// run host code, even if today's catalog snapshot is empty.
	// (Upstream maintainer may add a script tomorrow.)
	tmp := t.TempDir()
	appDir := filepath.Join(tmp, "empty-hooks-app")
	if err := os.MkdirAll(filepath.Join(appDir, "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	has, reason := appHasUnsupportedHookArtifacts(tmp, "empty-hooks-app")
	if !has {
		t.Errorf("empty hooks/ dir should still count as unsupported, got has=false")
	}
	if !contains(reason, "hooks/") {
		t.Errorf("expected reason to mention hooks/, got %q", reason)
	}
}

func TestAppHasUnsupportedHookArtifacts_FileNamedHooksIsNotADir(t *testing.T) {
	// Defensive: if upstream ships a FILE called `hooks` (not a dir),
	// don't classify as hook-bearing. The mechanism we're guarding
	// against is the dir-of-scripts pattern; a plain file with that
	// name is unrelated.
	tmp := t.TempDir()
	appDir := filepath.Join(tmp, "weird-file-app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "hooks"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	has, _ := appHasUnsupportedHookArtifacts(tmp, "weird-file-app")
	if has {
		t.Errorf("a FILE named hooks (not a dir) should NOT be flagged as hook-bearing, got has=true")
	}
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
