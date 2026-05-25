#!/usr/bin/env bash
# Hermetic tests for check-coverage-ratchet.sh — uses the PCT + baseline
# override hooks so no `go test` / coverage profile is needed.
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="$HERE/check-coverage-ratchet.sh"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
BL="$tmp/baseline.tsv"
printf '# comment\ngateway\t38\ncore\t6\n' > "$BL"

pass=0
fail=0
check() {
  local desc="$1" want="$2"
  shift 2
  local got=0
  "$@" >/dev/null 2>&1 || got=$?
  if [[ "$got" == "$want" ]]; then echo "PASS: $desc"; pass=$((pass + 1));
  else echo "FAIL: $desc (exit $got, want $want)"; fail=$((fail + 1)); fi
}

# Above floor → OK.
check "above floor passes" 0 \
  env POWERLAB_COVERAGE_BASELINE_FILE="$BL" POWERLAB_COVERAGE_PCT_OVERRIDE="40.7" bash "$SCRIPT" gateway
# Exactly at floor → OK (not below).
check "exactly at floor passes" 0 \
  env POWERLAB_COVERAGE_BASELINE_FILE="$BL" POWERLAB_COVERAGE_PCT_OVERRIDE="38" bash "$SCRIPT" gateway
# Below floor → regression (exit 1).
check "below floor fails" 1 \
  env POWERLAB_COVERAGE_BASELINE_FILE="$BL" POWERLAB_COVERAGE_PCT_OVERRIDE="37.9" bash "$SCRIPT" gateway
# Service with no floor → advisory skip (exit 0).
check "no floor → skip" 0 \
  env POWERLAB_COVERAGE_BASELINE_FILE="$BL" POWERLAB_COVERAGE_PCT_OVERRIDE="0.1" bash "$SCRIPT" pkg
# Missing arg → usage (exit 2).
check "missing service arg → usage" 2 bash "$SCRIPT"

echo "----"
echo "$pass passed, $fail failed"
[[ "$fail" == 0 ]]
