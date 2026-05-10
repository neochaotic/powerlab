#!/usr/bin/env bash
# Regression tests for scripts/package-linux.sh ldflag injection (issue
# #159). Locks in the version-stamp ldflags so the next "let's clean
# this up" refactor doesn't quietly resurrect the v0.5.4 mishap where:
#
#   - `main.version` was passed but each main.go uses `commit`/`date`
#     → ldflag silently no-op'd → binary kept default "private build"
#   - `github.com/IceWhaleTech/CasaOS/common.POWERLAB_VERSION` path was
#     used but #151 renamed everything to
#     `github.com/neochaotic/powerlab/backend/*` → also silently no-op
#
# Go's `-X` linker flag is fail-soft: if the target var doesn't exist,
# the build still succeeds. So this kind of bit-rot is invisible at
# build time. We catch it here.
#
# Usage:
#   ./scripts/check-package-linux-ldflags_test.sh
#
# Exit 0 = all assertions pass; exit 1 = at least one failed.

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

assert_no_grep() {
  local description="$1"
  local pattern="$2"
  if grep -q -F -- "$pattern" "$TARGET"; then
    echo "  FAIL: $description (pattern '$pattern' present, must be removed)" >&2
    failures=$((failures + 1))
  else
    echo "  PASS: $description"
  fi
}

echo "Test: required ldflag targets present"
assert_grep "main.commit ldflag set"        "-X main.commit=\$GIT_COMMIT"
assert_grep "main.date ldflag set"          "-X main.date=\$BUILD_DATE"
assert_grep "core POWERLAB_VERSION ldflag"  "-X github.com/neochaotic/powerlab/backend/core/common.POWERLAB_VERSION=\$VERSION"
assert_grep "core powerLabVersionAtCompileTime ldflag" \
  "-X github.com/neochaotic/powerlab/backend/core/route/v1.powerLabVersionAtCompileTime=\$VERSION"

echo "Test: deprecated ldflag targets absent (would silently no-op)"
assert_no_grep "old main.version target absent (var doesn't exist)"  "-X main.version="
assert_no_grep "old IceWhaleTech path absent (renamed in #151)"      "-X github.com/IceWhaleTech/CasaOS/common.POWERLAB_VERSION="

echo "Test: target Go vars actually exist in the source"
assert_grep_in_repo() {
  local description="$1"
  local file="$REPO_ROOT/$2"
  local pattern="$3"
  if [[ ! -f "$file" ]]; then
    echo "  FAIL: $description (file $file missing)" >&2
    failures=$((failures + 1))
    return
  fi
  if grep -q -E -- "$pattern" "$file"; then
    echo "  PASS: $description"
  else
    echo "  FAIL: $description (pattern '$pattern' not found in $file)" >&2
    failures=$((failures + 1))
  fi
}
assert_grep_in_repo "core declares POWERLAB_VERSION" \
  "backend/core/common/constants.go" "var POWERLAB_VERSION"
assert_grep_in_repo "core declares powerLabVersionAtCompileTime" \
  "backend/core/route/v1/powerlab_update.go" "var powerLabVersionAtCompileTime"

echo
if [[ "$failures" == "0" ]]; then
  echo "OK: all checks passed"
  exit 0
else
  echo "FAIL: $failures failure(s)" >&2
  exit 1
fi
