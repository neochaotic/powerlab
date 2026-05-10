#!/usr/bin/env bash
# Regression tests for scripts/migrate-casaos-data.sh — locks in the
# behavior that fixes issues #158 (v0.5.4 install.sh missed migrating
# casaos data) and #179 (v0.5.4 hot-fix wrote to wrong destination
# paths, creating split-brain risk).
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
# /var/lib/casaos has user.db (from a casaos install — note: no /db/
# subdir, casaos's user-service used <DataPath>/user.db just like
# PowerLab's does).
# /var/lib/powerlab/db EXISTS (created fresh by v0.5.4 message-bus
# startup) but /var/lib/powerlab/user.db is missing.
# Expectation: user.db migrated to canonical /var/lib/powerlab/user.db
# (NOT /var/lib/powerlab/db/user.db — the #179 bug fix).
echo "Test 1: v0.5.4 mishap scenario (user.db missing in powerlab)"
SANDBOX=$(mktemp -d)
trap 'rm -rf "$SANDBOX"' EXIT
mkdir -p "$SANDBOX/var/lib/casaos"
echo "user data from v0.5.3" > "$SANDBOX/var/lib/casaos/user.db"
mkdir -p "$SANDBOX/var/lib/casaos/db"
echo "message-bus data from v0.5.3" > "$SANDBOX/var/lib/casaos/db/message-bus.db"
mkdir -p "$SANDBOX/var/lib/powerlab/db"
echo "fresh v0.5.4 message-bus init" > "$SANDBOX/var/lib/powerlab/db/message-bus.db"

PREFIX="$SANDBOX" "$SCRIPT" >/dev/null

assert_file_exists "user.db migrated to canonical (no /db/)" \
  "$SANDBOX/var/lib/powerlab/user.db"
assert_file_content "user.db has v0.5.3 content" \
  "$SANDBOX/var/lib/powerlab/user.db" "user data from v0.5.3"
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
# Expectation: every file copied to its canonical destination per the
# per-service map in migrate-casaos-data.sh.
echo "Test 3: full upgrade (no /var/lib/powerlab)"
SANDBOX=$(mktemp -d)
trap 'rm -rf "$SANDBOX"' EXIT
mkdir -p "$SANDBOX/var/lib/casaos"/{db,apps,appstore,conf}
echo "user.db" > "$SANDBOX/var/lib/casaos/user.db"
echo "casaOS.db" > "$SANDBOX/var/lib/casaos/db/casaOS.db"
mkdir -p "$SANDBOX/var/lib/casaos/apps/myapp"
echo "compose-yml" > "$SANDBOX/var/lib/casaos/apps/myapp/docker-compose.yml"

PREFIX="$SANDBOX" "$SCRIPT" >/dev/null

assert_file_exists "user.db migrated to canonical" \
  "$SANDBOX/var/lib/powerlab/user.db"
assert_file_exists "core casaOS.db at legacy /db/ path (still used by core)" \
  "$SANDBOX/var/lib/powerlab/db/casaOS.db"
assert_file_exists "apps subtree" \
  "$SANDBOX/var/lib/powerlab/apps/myapp/docker-compose.yml"
rm -rf "$SANDBOX"

# ── Test 4: idempotent re-run ──────────────────────────────────────────
# After a successful migration, running again must be a no-op (no
# overwrites, no errors). Critical because install.sh is invoked on
# every upgrade.
echo "Test 4: idempotent re-run"
SANDBOX=$(mktemp -d)
trap 'rm -rf "$SANDBOX"' EXIT
mkdir -p "$SANDBOX/var/lib/casaos"
echo "from-casaos" > "$SANDBOX/var/lib/casaos/user.db"

PREFIX="$SANDBOX" "$SCRIPT" >/dev/null
# Now mutate the destination — second run must not overwrite.
echo "modified-by-user" > "$SANDBOX/var/lib/powerlab/user.db"
PREFIX="$SANDBOX" "$SCRIPT" >/dev/null

assert_file_content "user-modified content preserved on re-run" \
  "$SANDBOX/var/lib/powerlab/user.db" "modified-by-user"
rm -rf "$SANDBOX"

# ── Test 5: source preservation (rollback safety) ──────────────────────
# After migration, the /var/lib/casaos source MUST still be there so
# a sysadmin can manually roll back.
echo "Test 5: source preservation"
SANDBOX=$(mktemp -d)
trap 'rm -rf "$SANDBOX"' EXIT
mkdir -p "$SANDBOX/var/lib/casaos"
echo "original" > "$SANDBOX/var/lib/casaos/user.db"

PREFIX="$SANDBOX" "$SCRIPT" >/dev/null

assert_file_exists "casaos source still there" \
  "$SANDBOX/var/lib/casaos/user.db"
assert_file_content "casaos source unchanged" \
  "$SANDBOX/var/lib/casaos/user.db" "original"
rm -rf "$SANDBOX"

# ── Test 6: NO split-brain after migration ─────────────────────────────
# This is THE regression for #179. After running the migration on a
# host with casaos data, we must end up with EXACTLY ONE copy of
# user.db at the canonical path — NOT a duplicate at the legacy
# /var/lib/powerlab/db/user.db hot-fix path.
echo "Test 6: no split-brain after migration (THE #179 regression)"
SANDBOX=$(mktemp -d)
trap 'rm -rf "$SANDBOX"' EXIT
mkdir -p "$SANDBOX/var/lib/casaos"
echo "real user data" > "$SANDBOX/var/lib/casaos/user.db"

PREFIX="$SANDBOX" "$SCRIPT" >/dev/null

assert_file_exists "user.db at CANONICAL path" \
  "$SANDBOX/var/lib/powerlab/user.db"
assert_no_file "user.db NOT at legacy /db/ path (split-brain prevention)" \
  "$SANDBOX/var/lib/powerlab/db/user.db"

# Same assertion for local-storage.db
mkdir -p "$SANDBOX/var/lib/casaos"
echo "local-storage data" > "$SANDBOX/var/lib/casaos/local-storage.db"
PREFIX="$SANDBOX" "$SCRIPT" >/dev/null
assert_file_exists "local-storage.db at CANONICAL path" \
  "$SANDBOX/var/lib/powerlab/local-storage.db"
assert_no_file "local-storage.db NOT at legacy /db/ path" \
  "$SANDBOX/var/lib/powerlab/db/local-storage.db"
rm -rf "$SANDBOX"

# ── Test 7: pre-existing legacy hot-fix copy is left alone ─────────────
# Scenario: a host that ran the v0.5.4-v0.5.6 broken hot-fix already
# has /var/lib/powerlab/db/user.db (the wrong-path duplicate). When
# v0.5.7+ migration runs, it MUST NOT delete or move that file
# automatically (operator chose to leave it; might be debugging).
# Instead, the canonical copy IS created, and the boot-time
# split-brain check (in user-service main.go) refuses to start with
# a clear recovery message. This test asserts the migration's role:
# create the canonical, don't touch the legacy duplicate.
echo "Test 7: pre-existing legacy hot-fix copy preserved (boot check handles)"
SANDBOX=$(mktemp -d)
trap 'rm -rf "$SANDBOX"' EXIT
mkdir -p "$SANDBOX/var/lib/casaos"
echo "real source" > "$SANDBOX/var/lib/casaos/user.db"
mkdir -p "$SANDBOX/var/lib/powerlab/db"
echo "stale hot-fix copy" > "$SANDBOX/var/lib/powerlab/db/user.db"

PREFIX="$SANDBOX" "$SCRIPT" >/dev/null

assert_file_exists "canonical user.db created" \
  "$SANDBOX/var/lib/powerlab/user.db"
assert_file_content "canonical has fresh source content" \
  "$SANDBOX/var/lib/powerlab/user.db" "real source"
assert_file_exists "stale legacy copy NOT touched (operator must clean up)" \
  "$SANDBOX/var/lib/powerlab/db/user.db"
assert_file_content "stale legacy copy unchanged" \
  "$SANDBOX/var/lib/powerlab/db/user.db" "stale hot-fix copy"
rm -rf "$SANDBOX"

echo
if [[ "$failures" == "0" ]]; then
  echo "OK: all checks passed"
  exit 0
else
  echo "FAIL: $failures failure(s)" >&2
  exit 1
fi
