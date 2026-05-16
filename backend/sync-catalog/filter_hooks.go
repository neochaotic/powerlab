package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// appHasUnsupportedHookArtifacts checks whether the upstream app dir
// contains `hooks/` (a directory) or `exports.sh` (a file). Both are
// the umbrelOS host-execution surface — `hooks/*` scripts are run as
// the orchestrator's user via `legacy-compat/app-script`, and
// `exports.sh` is dot-sourced into the host shell. PowerLab never
// executes either; ADR-0038 makes this a hard filter at sync time
// rather than a runtime concern.
//
// Returns (true, "reason") when the app must be filtered, and
// (false, "") when the app is clean. The reason string is the
// human-readable trigger for the sync log.
//
// Edge cases handled:
//   - Missing app dir → (false, "") so the caller can still fall
//     through to other parse/error paths without panicking.
//   - `hooks` as a FILE (not a dir) → not flagged; the mechanism
//     we're guarding is the dir-of-scripts pattern, not arbitrary
//     filenames.
//   - Empty `hooks/` dir → STILL flagged. The dir's presence
//     signals upstream intent to ship hooks; an empty snapshot
//     today may have a script tomorrow.
//
// This function is the security gate referenced by ADR-0038 — keep
// the behavior locked by tests. Any change here is a security review
// surface.
func appHasUnsupportedHookArtifacts(upstreamDir, appID string) (bool, string) {
	appDir := filepath.Join(upstreamDir, appID)

	hooksPath := filepath.Join(appDir, "hooks")
	if info, err := os.Stat(hooksPath); err == nil && info.IsDir() {
		return true, fmt.Sprintf("ships hooks/ directory (PowerLab does not execute upstream host scripts — ADR-0038)")
	}

	exportsPath := filepath.Join(appDir, "exports.sh")
	if info, err := os.Stat(exportsPath); err == nil && !info.IsDir() {
		return true, fmt.Sprintf("ships exports.sh file (PowerLab does not dot-source upstream shell — ADR-0038)")
	}

	return false, ""
}
