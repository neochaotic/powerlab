#!/usr/bin/env bash
# Regression tests for scripts/package-linux.sh defense-in-depth checks
# against the v0.6.6 UI version mismatch bug (deployed UI bundle
# carrying a stale __APP_VERSION__ literal = "0.3.1" while server was
# v0.6.5; surface a non-dismissable banner that reload couldn't fix).
#
# Three layers, all locked here:
#   L1 — npm version $VERSION called before the build so pkg.json is
#        always in sync with the release tag.
#   L3 — final sanity grep of build/_app/* for the version literal
#        before sealing the tarball.
#
# L2 lives in scripts/check-ui-package-version-fresh_test.sh (separate
# script because it runs as a CI gate before any release, not as part
# of the package build itself).

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

echo "Test: L1 npm version sync wired in"
assert_grep "npm version pre-build" "npm version \"\$VERSION\" --no-git-tag-version --allow-same-version"

echo "Test: L3 sanity grep wired in"
assert_grep "grep against built chunks"    "grep -rqF -- \"\$VERSION\" build/_app/immutable/chunks/ build/_app/immutable/nodes/"
assert_grep "abort message"                "Aborting before sealing tarball"

echo
if [[ "$failures" == "0" ]]; then
  echo "OK: all checks passed"
  exit 0
else
  echo "FAIL: $failures failure(s)" >&2
  exit 1
fi
