#!/usr/bin/env bash
# Regression tests for scripts/package-linux.sh sync-catalog packaging
# (Sprint 13.3, #248). The v0.6.x cycle (v0.6.1 → v0.6.2 → v0.6.3 → v0.6.4)
# spent 4 releases fighting a single class of bug: the tarball's bundled
# community-catalog/ kept being stale relative to the binary's transform
# logic. The fix: ship the sync-catalog binary itself in /usr/bin so
# install.sh can refresh the catalog post-install, decoupling catalog
# freshness from tarball freshness.
#
# These tests lock the package-script wiring so a future refactor
# doesn't silently break the self-healing flow.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TARGET="$REPO_ROOT/scripts/package-linux.sh"

failures=0

assert_grep() {
  local description="$1"
  local pattern="$2"
  if grep -q -F -- "$pattern" "$TARGET"; then
    echo "  PASS: $description"
  else
    echo "  FAIL: $description (pattern '$pattern' not found)" >&2
    failures=$((failures + 1))
  fi
}

echo "Test: sync-catalog cross-compile step present"
assert_grep "go build for sync-catalog"     "cd \"\$ROOT/backend/sync-catalog\""
assert_grep "sync-catalog binary output"    "-o \"\$STAGE/bin/powerlab-sync-catalog\""

echo "Test: post-install catalog refresh wired in"
assert_grep "command -v git guard"          "command -v git"
assert_grep "command -v sync-catalog guard" "command -v /usr/bin/powerlab-sync-catalog"
assert_grep "timeout-bounded sync"          "timeout 60 /usr/bin/powerlab-sync-catalog"
assert_grep "best-effort fallback message"  "bundled catalog will be used"
assert_grep "POWERLAB_SKIP_SYNC env escape"  "POWERLAB_SKIP_SYNC:-0"

echo "Test: backend/sync-catalog package compiles"
if (cd "$REPO_ROOT/backend/sync-catalog" && go build -o /dev/null . 2>/dev/null); then
  echo "  PASS: backend/sync-catalog compiles"
else
  echo "  FAIL: backend/sync-catalog failed to compile" >&2
  failures=$((failures + 1))
fi

echo
if [[ "$failures" == "0" ]]; then
  echo "OK: all checks passed"
  exit 0
else
  echo "FAIL: $failures failure(s)" >&2
  exit 1
fi
