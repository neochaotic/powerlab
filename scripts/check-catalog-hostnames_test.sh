#!/usr/bin/env bash
# Meta-test for check-catalog-hostnames.sh — exercises the gate's
# detection on synthetic fixtures + asserts both warn-only and
# strict mode behaviours.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LINT_SCRIPT="$SCRIPT_DIR/check-catalog-hostnames.sh"

if [[ ! -x "$LINT_SCRIPT" ]]; then
  echo "FAIL: $LINT_SCRIPT not found or not executable" >&2
  exit 1
fi

# Each test creates an isolated fixture file and runs the lint
# script against it (the script accepts a path argument that
# overrides the default catalog scan).

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

assert_findings() {
  local fixture="$1"
  local expected_lines="$2"
  local label="$3"
  local actual
  actual="$("$LINT_SCRIPT" "$fixture" 2>/dev/null | wc -l | tr -d ' ')"
  if [[ "$actual" != "$expected_lines" ]]; then
    echo "FAIL: $label — expected $expected_lines findings, got $actual" >&2
    return 1
  fi
  echo "OK: $label — $expected_lines finding(s)"
}

# Fixture 1: broken (legacy compose v1 underscore form)
cat > "$TMP/broken.yml" <<'EOF'
name: testapp
services:
    app:
        environment:
            DB_HOST: testapp_db_1
            REDIS_URL: redis://testapp_redis_1:6379
    db:
        image: postgres
    redis:
        image: redis
EOF
assert_findings "$TMP/broken.yml" "2" "broken fixture flags both DB_HOST and REDIS_URL"

# Fixture 2: clean (service-name aliases)
cat > "$TMP/clean.yml" <<'EOF'
name: cleanapp
services:
    app:
        environment:
            DB_HOST: db
            REDIS_URL: redis://redis:6379
    db:
        image: postgres
    redis:
        image: redis
EOF
assert_findings "$TMP/clean.yml" "0" "clean fixture produces no findings"

# Fixture 3: no `name:` line — script must not crash
cat > "$TMP/noname.yml" <<'EOF'
services:
    app:
        environment:
            DB_HOST: db
EOF
assert_findings "$TMP/noname.yml" "0" "missing name: header doesn't crash"

# Fixture 4: false-positive guard — project "blink" must NOT match
# a substring of "blinko_db_1" (project name token is anchored).
cat > "$TMP/blink.yml" <<'EOF'
name: blink
services:
    app:
        environment:
            DB_HOST: blinko_db_1
    blink_db:
        image: postgres
EOF
assert_findings "$TMP/blink.yml" "0" "false-positive guard — blink does NOT match blinko_db_1"

# Fixture 5: strict mode exits 1 on findings
if POWERLAB_CATALOG_LINT_STRICT=1 "$LINT_SCRIPT" "$TMP/broken.yml" >/dev/null 2>&1; then
  echo "FAIL: strict mode should exit non-zero on findings" >&2
  exit 1
fi
echo "OK: strict mode exits 1 when findings present"

# Fixture 6: strict mode exits 0 on clean
if ! POWERLAB_CATALOG_LINT_STRICT=1 "$LINT_SCRIPT" "$TMP/clean.yml" >/dev/null 2>&1; then
  echo "FAIL: strict mode should exit 0 on clean fixture" >&2
  exit 1
fi
echo "OK: strict mode exits 0 when clean"

echo ""
echo "ALL TESTS PASSED"
