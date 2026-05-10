#!/usr/bin/env bash
# Regression tests for scripts/migrate-casaos-data.sh — locks in the
# behavior that fixes issue #158 (v0.5.4 prod incident: install.sh
# didn't migrate /var/lib/casaos/* to /var/lib/powerlab/*, login broke).
#
# Tests run against a sandbox via PREFIX env var so they touch no
# real /var/lib paths. Per memory rule: bug fix lands with a regression
# test that locks the behavior.
#
# Usage:
#   ./scripts/migrate-casaos-data_test.sh
#
# Exit 0 = all assertions pass; non-zero = at least one failed.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO_ROOT/scripts/migrate-casaos-data.sh"

failures=0

assert_file_exists() {
  local description="$1"
  local path="$2"
  if [[ -f "$path" ]]; then
    echo "  PASS: $description"
  else
    echo "  FAIL: $description ($path missing)" >&2
    failures=$((failures + 1))
  fi
}

assert_file_content() {
  local description="$1"
  local path="$2"
  local expected="$3"
  if [[ ! -f "$path" ]]; then
    echo "  FAIL: $description ($path missing)" >&2
    failures=$((failures + 1))
    return
  fi
  local actual
  actual=$(cat "$path")
  if [[ "$actual" == "$expected" ]]; then
    echo "  PASS: $description"
  else
    echo "  FAIL: $description (got '$actual', want '$expected')" >&2
    failures=$((failures + 1))
  fi
}

assert_no_file() {
  local description="$1"
  local path="$2"
  if [[ ! -e "$path" ]]; then
    echo "  PASS: $description"
  else
    echo "  FAIL: $description ($path should not exist)" >&2
    failures=$((failures + 1))
  fi
}

# ── Test 1: the v0.5.4 mishap scenario ──────────────────────────────────
# /var/lib/casaos/db has user.db (from v0.5.3 install).
# /var/lib/powerlab/db EXISTS (created fresh by v0.5.4 message-bus
# startup) but has only message-bus.db, no user.db.
# Expectation: user.db migrated, message-bus.db NOT overwritten.
echo "Test 1: v0.5.4 mishap scenario (user.db missing in powerlab)"
SANDBOX=$(mktemp -d)
trap 'rm -rf "$SANDBOX"' EXIT
mkdir -p "$SANDBOX/var/lib/casaos/db"
echo "user data from v0.5.3" > "$SANDBOX/var/lib/casaos/db/user.db"
echo "message-bus data from v0.5.3" > "$SANDBOX/var/lib/casaos/db/message-bus.db"
mkdir -p "$SANDBOX/var/lib/powerlab/db"
echo "fresh v0.5.4 message-bus init" > "$SANDBOX/var/lib/powerlab/db/message-bus.db"

PREFIX="$SANDBOX" "$SCRIPT" >/dev/null

assert_file_exists "user.db migrated" "$SANDBOX/var/lib/powerlab/db/user.db"
assert_file_content "user.db has v0.5.3 content" "$SANDBOX/var/lib/powerlab/db/user.db" "user data from v0.5.3"
assert_file_content "message-bus.db NOT overwritten (fresh v0.5.4 wins)" \
  "$SANDBOX/var/lib/powerlab/db/message-bus.db" "fresh v0.5.4 message-bus init"
rm -rf "$SANDBOX"

# ── Test 2: fresh install (no /var/lib/casaos at all) ───────────────────
# Expectation: no error, nothing created in powerlab.
echo "Test 2: fresh install (no /var/lib/casaos)"
SANDBOX=$(mktemp -d)
trap 'rm -rf "$SANDBOX"' EXIT
mkdir -p "$SANDBOX/var/lib"

PREFIX="$SANDBOX" "$SCRIPT" >/dev/null

assert_no_file "no /var/lib/powerlab/db dir created on fresh install" \
  "$SANDBOX/var/lib/powerlab/db"
rm -rf "$SANDBOX"

# ── Test 3: full upgrade (no /var/lib/powerlab yet) ─────────────────────
# CasaOS install with all standard subdirs, PowerLab dir doesn't exist.
# Expectation: every subdir copied verbatim.
echo "Test 3: full upgrade (no /var/lib/powerlab)"
SANDBOX=$(mktemp -d)
trap 'rm -rf "$SANDBOX"' EXIT
mkdir -p "$SANDBOX/var/lib/casaos"/{db,apps,appstore,conf}
echo "user.db" > "$SANDBOX/var/lib/casaos/db/user.db"
echo "casaOS.db" > "$SANDBOX/var/lib/casaos/db/casaOS.db"
mkdir -p "$SANDBOX/var/lib/casaos/apps/myapp"
echo "compose-yml" > "$SANDBOX/var/lib/casaos/apps/myapp/docker-compose.yml"

PREFIX="$SANDBOX" "$SCRIPT" >/dev/null

assert_file_exists "db migrated"      "$SANDBOX/var/lib/powerlab/db/user.db"
assert_file_exists "db casaOS.db"     "$SANDBOX/var/lib/powerlab/db/casaOS.db"
assert_file_exists "apps subtree"     "$SANDBOX/var/lib/powerlab/apps/myapp/docker-compose.yml"
rm -rf "$SANDBOX"

# ── Test 4: idempotent re-run ──────────────────────────────────────────
# After a successful migration, running again must be a no-op (no
# overwrites, no errors). Critical because install.sh is invoked on
# every upgrade.
echo "Test 4: idempotent re-run"
SANDBOX=$(mktemp -d)
trap 'rm -rf "$SANDBOX"' EXIT
mkdir -p "$SANDBOX/var/lib/casaos/db"
echo "from-casaos" > "$SANDBOX/var/lib/casaos/db/user.db"

PREFIX="$SANDBOX" "$SCRIPT" >/dev/null
# Now mutate the destination — second run must not overwrite.
echo "modified-by-user" > "$SANDBOX/var/lib/powerlab/db/user.db"
PREFIX="$SANDBOX" "$SCRIPT" >/dev/null

assert_file_content "user-modified content preserved on re-run" \
  "$SANDBOX/var/lib/powerlab/db/user.db" "modified-by-user"
rm -rf "$SANDBOX"

# ── Test 5: source preservation (rollback safety) ──────────────────────
# After migration, the /var/lib/casaos source MUST still be there so
# a sysadmin can manually roll back.
echo "Test 5: source preservation"
SANDBOX=$(mktemp -d)
trap 'rm -rf "$SANDBOX"' EXIT
mkdir -p "$SANDBOX/var/lib/casaos/db"
echo "original" > "$SANDBOX/var/lib/casaos/db/user.db"

PREFIX="$SANDBOX" "$SCRIPT" >/dev/null

assert_file_exists "casaos source still there" \
  "$SANDBOX/var/lib/casaos/db/user.db"
assert_file_content "casaos source unchanged" \
  "$SANDBOX/var/lib/casaos/db/user.db" "original"
rm -rf "$SANDBOX"

echo
if [[ "$failures" == "0" ]]; then
  echo "OK: all checks passed"
  exit 0
else
  echo "FAIL: $failures failure(s)" >&2
  exit 1
fi
