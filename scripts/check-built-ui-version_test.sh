#!/usr/bin/env bash
# Injection-based regression test for scripts/check-built-ui-version.sh
# — the L3 sanity grep that aborts a release if the built UI bundle
# doesn't contain the expected version literal (v0.6.6 retro).
#
# Why injection: a structural grep on the script proves the wiring
# is present but doesn't prove the gate ACTUALLY CATCHES the bug
# class. This test builds sandbox `build/` trees with both correct
# and intentionally-wrong version literals, runs the script against
# them, and asserts exit codes match expectations.
#
# Cases covered:
#   1. PASS — bundle contains the requested version literal
#   2. FAIL — bundle contains a DIFFERENT version literal (the
#      original bug class: pkg.json stale + POWERLAB_VERSION lost
#      → bundle stamped with the wrong literal)
#   3. FAIL — bundle has NO version literal (corrupted build)
#   4. FAIL — build directory does not exist (build skipped)
#   5. FAIL — missing args
#   6. PASS — bare-literal form (minifier-stripped quotes,
#             the case that motivated the v0.6.6 fix where the
#             quoted-form grep `"$VERSION"` was wrong)
#   7. PASS — quoted-literal form (defensive — some bundlers
#             keep the JSON.stringify quotes)
#
# These match the actual failure modes we'd see in production.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO_ROOT/scripts/check-built-ui-version.sh"

failures=0

assert_exit() {
  local description="$1"
  local expected_exit="$2"
  shift 2
  local actual_exit=0
  "$SCRIPT" "$@" >/dev/null 2>&1 || actual_exit=$?
  if [[ "$actual_exit" == "$expected_exit" ]]; then
    echo "  PASS: $description"
  else
    echo "  FAIL: $description (expected exit $expected_exit, got $actual_exit)" >&2
    failures=$((failures + 1))
  fi
}

SANDBOX=$(mktemp -d)
trap 'rm -rf "$SANDBOX"' EXIT

make_fake_bundle() {
  local build_dir="$1"
  local content="$2"
  rm -rf "$build_dir"
  mkdir -p "$build_dir/_app/immutable/chunks" "$build_dir/_app/immutable/nodes"
  printf '%s\n' "$content" > "$build_dir/_app/immutable/chunks/test.js"
}

echo "Case 1 — bundle stamped with the requested version (bare literal)"
make_fake_bundle "$SANDBOX/case1" "uiVersion=0.6.6;"
assert_exit "exit 0 when grep finds the literal" 0 "0.6.6" "$SANDBOX/case1"

echo
echo "Case 2 — bundle stamped with a DIFFERENT version (the bug class)"
make_fake_bundle "$SANDBOX/case2" "uiVersion=0.3.1;"
assert_exit "exit 1 when bundle has wrong version 0.3.1, expecting 0.6.6" 1 "0.6.6" "$SANDBOX/case2"

echo
echo "Case 3 — bundle has NO version literal at all (corrupted build)"
make_fake_bundle "$SANDBOX/case3" "// totally unrelated chunk content"
assert_exit "exit 1 when bundle has no version literal" 1 "0.6.6" "$SANDBOX/case3"

echo
echo "Case 4 — build directory does not exist (build step skipped)"
assert_exit "exit 1 when build_dir missing" 1 "0.6.6" "$SANDBOX/does-not-exist"

echo
echo "Case 5 — missing args"
assert_exit "exit 2 with no args" 2
assert_exit "exit 2 with only version" 2 "0.6.6"

echo
echo "Case 6 — bare literal form (Rollup minifier output)"
make_fake_bundle "$SANDBOX/case6" "const v=0.0.0-ci;"
assert_exit "exit 0 when bare literal matches" 0 "0.0.0-ci" "$SANDBOX/case6"

echo
echo "Case 7 — quoted literal form (defensive)"
make_fake_bundle "$SANDBOX/case7" 'const v="0.6.7-rc.1";'
assert_exit "exit 0 when quoted literal matches" 0 "0.6.7-rc.1" "$SANDBOX/case7"

echo
echo "Case 8 — version literal lives in /nodes/ not /chunks/"
rm -rf "$SANDBOX/case8"
mkdir -p "$SANDBOX/case8/_app/immutable/chunks" "$SANDBOX/case8/_app/immutable/nodes"
echo "const v=1.2.3;" > "$SANDBOX/case8/_app/immutable/nodes/0.layout.js"
assert_exit "exit 0 when literal found in nodes/ subdir" 0 "1.2.3" "$SANDBOX/case8"

echo
if [[ "$failures" == "0" ]]; then
  echo "OK: all injection cases passed (gate behaves correctly under positive AND negative scenarios)"
  exit 0
else
  echo "FAIL: $failures failure(s)" >&2
  exit 1
fi
