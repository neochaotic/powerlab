#!/usr/bin/env bash
# Regression tests for scripts/check-manifest-fresh.sh.
#
# Locks in the behavior that catches the v0.5.4 mishap: the script
# must exit non-zero when release-manifest.yaml summary is identical
# to a previously released manifest.json's summary.
#
# Usage:
#   ./scripts/check-manifest-fresh_test.sh
#
# Exit 0 = all assertions pass; exit 1 = at least one failed.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO_ROOT/scripts/check-manifest-fresh.sh"
FIXTURES="$(mktemp -d)"
trap 'rm -rf "$FIXTURES"' EXIT

failures=0

assert_exit() {
  local description="$1"
  local expected_code="$2"
  shift 2
  set +e
  "$@" >/dev/null 2>&1
  local actual=$?
  set -e
  if [[ "$actual" == "$expected_code" ]]; then
    echo "  PASS: $description (exit=$actual)"
  else
    echo "  FAIL: $description (expected exit=$expected_code, got $actual)" >&2
    failures=$((failures + 1))
  fi
}

# ── Fixture 1: identical summary ─────────────────────────────────────
# Read the LIVE release-manifest.yaml summary and emit a fixture
# manifest.json with the same text. The script MUST detect this.
local_summary=$(awk '
  /^summary:[[:space:]]*\|/ { capturing = 1; next }
  capturing && /^[[:space:]]/ { sub(/^[[:space:]]+/, ""); printf "%s ", $0; next }
  capturing && /^[^[:space:]]/ { exit }
' "$REPO_ROOT/release-manifest.yaml")

cat > "$FIXTURES/identical.json" <<EOF
{"summary": $(printf '%s' "$local_summary" | python3 -c 'import sys, json; print(json.dumps(sys.stdin.read().strip()))')}
EOF

echo "Test: identical summary (should fail with exit 1)"
assert_exit "exit 1 when YAML matches latest release" 1 "$SCRIPT" "$FIXTURES/identical.json"

# ── Fixture 2: different summary ─────────────────────────────────────
cat > "$FIXTURES/different.json" <<'EOF'
{"summary": "Some completely different prior-release summary that doesn't match the YAML at all — proves the diff path."}
EOF

echo "Test: different summary (should pass with exit 0)"
assert_exit "exit 0 when YAML differs from latest release" 0 "$SCRIPT" "$FIXTURES/different.json"

# ── Fixture 3: empty summary in fixture (e.g. malformed manifest) ───
cat > "$FIXTURES/empty.json" <<'EOF'
{"version": "0.0.0"}
EOF

echo "Test: missing summary in fixture (should pass with exit 0 — nothing to compare)"
# Empty remote summary will not match local, so script exits 0.
assert_exit "exit 0 when fixture has no summary field" 0 "$SCRIPT" "$FIXTURES/empty.json"

# ── Fixture 4: nonexistent fixture path ─────────────────────────────
echo "Test: nonexistent fixture path (should fail with exit 2)"
assert_exit "exit 2 when fixture path does not exist" 2 "$SCRIPT" "$FIXTURES/does-not-exist.json"

echo
if [[ "$failures" == "0" ]]; then
  echo "OK: all $failures failures (out of 4 tests)"
  exit 0
else
  echo "FAIL: $failures failure(s) (out of 4 tests)" >&2
  exit 1
fi
