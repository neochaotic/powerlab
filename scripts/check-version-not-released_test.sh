#!/usr/bin/env bash
# Regression tests for check-version-not-released.sh — the published-version
# collision guard (prevents the v0.7.2 re-cut). Hermetic: uses the
# POWERLAB_PUBLISHED_RELEASES_OVERRIDE hook so no live `gh` call is made.
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="$HERE/check-version-not-released.sh"

pass=0
fail=0
check() {
  local desc="$1" want="$2"
  shift 2
  local got=0
  "$@" >/dev/null 2>&1 || got=$?
  if [[ "$got" == "$want" ]]; then
    echo "PASS: $desc"
    pass=$((pass + 1))
  else
    echo "FAIL: $desc (exit $got, want $want)"
    fail=$((fail + 1))
  fi
}

# A version that IS in the published set must be rejected (collision).
check "published version rejected (exit 1)" 1 \
  env POWERLAB_PUBLISHED_RELEASES_OVERRIDE="v0.7.1 v0.7.2 v0.7.3" bash "$SCRIPT" 0.7.2

# Leading-v input form is normalized and still caught.
check "published version rejected, leading-v form" 1 \
  env POWERLAB_PUBLISHED_RELEASES_OVERRIDE="v0.7.3" bash "$SCRIPT" v0.7.3

# A version NOT yet published is allowed.
check "fresh version passes (exit 0)" 0 \
  env POWERLAB_PUBLISHED_RELEASES_OVERRIDE="v0.7.1 v0.7.2 v0.7.3" bash "$SCRIPT" 0.7.4

# Empty published set (override present but empty) → nothing published → pass.
check "empty published set passes (exit 0)" 0 \
  env POWERLAB_PUBLISHED_RELEASES_OVERRIDE="" bash "$SCRIPT" 0.7.3

# Missing version argument → usage error (exit 2).
check "missing version arg → usage (exit 2)" 2 bash "$SCRIPT"

echo "----"
echo "$pass passed, $fail failed"
[[ "$fail" == 0 ]]
