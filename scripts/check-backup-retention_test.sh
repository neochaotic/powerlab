#!/usr/bin/env bash
# Regression tests for the snapshot retention logic added to install.sh
# in scripts/package-linux.sh (v0.5.4 retro item — user accumulated 4
# snapshots in one debugging session).
#
# We extract just the retention block and test it against fixture
# directories. The block is intentionally small + self-contained so a
# byte-for-byte copy of the production logic is trivial here.
#
# Usage:
#   ./scripts/check-backup-retention_test.sh
#
# Exit 0 = all assertions pass; non-zero = at least one failed.

set -euo pipefail

failures=0

# Mirror of the production retention block from package-linux.sh's
# install.sh. Keeping it here lets the test exercise the exact same
# logic without sourcing the whole install.sh (which has heavy
# side effects). If package-linux.sh's logic drifts, this test won't
# catch it — pair this with a manual diff at PR-review time.
prune_snapshots() {
  local backups_dir="$1"
  local keep="${2:-3}"
  if [[ "$keep" =~ ^[0-9]+$ ]] && (( keep > 0 )); then
    local pruned=0
    while IFS= read -r old_snap; do
      rm -rf "$old_snap"
      pruned=$((pruned + 1))
    done < <(ls -1dt "$backups_dir"/pre-upgrade-* 2>/dev/null | tail -n +$((keep + 1)))
    echo "$pruned"
  else
    echo "0"
  fi
}

assert_count() {
  local description="$1"
  local pattern="$2"
  local expected="$3"
  local actual
  actual=$(ls -d $pattern 2>/dev/null | wc -l | tr -d ' ')
  if [[ "$actual" == "$expected" ]]; then
    echo "  PASS: $description"
  else
    echo "  FAIL: $description (expected $expected, got $actual)" >&2
    failures=$((failures + 1))
  fi
}

assert_exists() {
  local description="$1"
  local path="$2"
  if [[ -e "$path" ]]; then
    echo "  PASS: $description"
  else
    echo "  FAIL: $description ($path missing)" >&2
    failures=$((failures + 1))
  fi
}

assert_no_exists() {
  local description="$1"
  local path="$2"
  if [[ ! -e "$path" ]]; then
    echo "  PASS: $description"
  else
    echo "  FAIL: $description ($path should not exist)" >&2
    failures=$((failures + 1))
  fi
}

# ── Test 1: 5 snapshots, keep 3 → 2 pruned ────────────────────────────
echo "Test 1: 5 snapshots, keep 3 → 2 oldest pruned"
SANDBOX=$(mktemp -d)
trap 'rm -rf "$SANDBOX"' EXIT
mkdir -p "$SANDBOX/backups"
# Create 5 snapshots with distinct mtimes (oldest first, sleep is overkill;
# touch -t with explicit timestamps is faster + deterministic).
for ts in 20260101T000000Z 20260201T000000Z 20260301T000000Z 20260401T000000Z 20260501T000000Z; do
  mkdir -p "$SANDBOX/backups/pre-upgrade-$ts"
  echo "marker" > "$SANDBOX/backups/pre-upgrade-$ts/file"
done
# Touch with explicit times so ls -t sorts deterministically.
touch -t 202601010000 "$SANDBOX/backups/pre-upgrade-20260101T000000Z"
touch -t 202602010000 "$SANDBOX/backups/pre-upgrade-20260201T000000Z"
touch -t 202603010000 "$SANDBOX/backups/pre-upgrade-20260301T000000Z"
touch -t 202604010000 "$SANDBOX/backups/pre-upgrade-20260401T000000Z"
touch -t 202605010000 "$SANDBOX/backups/pre-upgrade-20260501T000000Z"

pruned=$(prune_snapshots "$SANDBOX/backups" 3)
if [[ "$pruned" == "2" ]]; then
  echo "  PASS: prune count = 2"
else
  echo "  FAIL: prune count expected 2, got $pruned" >&2
  failures=$((failures + 1))
fi
assert_count "3 snapshots remain" "$SANDBOX/backups/pre-upgrade-*" 3
# The 3 NEWEST should remain.
assert_exists "newest snapshot kept" "$SANDBOX/backups/pre-upgrade-20260501T000000Z"
assert_exists "2nd-newest kept"      "$SANDBOX/backups/pre-upgrade-20260401T000000Z"
assert_exists "3rd-newest kept"      "$SANDBOX/backups/pre-upgrade-20260301T000000Z"
assert_no_exists "oldest pruned"     "$SANDBOX/backups/pre-upgrade-20260101T000000Z"
assert_no_exists "2nd-oldest pruned" "$SANDBOX/backups/pre-upgrade-20260201T000000Z"
rm -rf "$SANDBOX"

# ── Test 2: only 2 snapshots, keep 3 → 0 pruned ──────────────────────
echo "Test 2: only 2 snapshots, keep 3 → no-op"
SANDBOX=$(mktemp -d)
trap 'rm -rf "$SANDBOX"' EXIT
mkdir -p "$SANDBOX/backups/pre-upgrade-A" "$SANDBOX/backups/pre-upgrade-B"

pruned=$(prune_snapshots "$SANDBOX/backups" 3)
if [[ "$pruned" == "0" ]]; then
  echo "  PASS: nothing to prune (count=0)"
else
  echo "  FAIL: expected 0 pruned, got $pruned" >&2
  failures=$((failures + 1))
fi
assert_count "both snapshots kept" "$SANDBOX/backups/pre-upgrade-*" 2
rm -rf "$SANDBOX"

# ── Test 3: empty backups dir → no-op, no error ──────────────────────
echo "Test 3: empty backups dir → no error, no prune"
SANDBOX=$(mktemp -d)
trap 'rm -rf "$SANDBOX"' EXIT
mkdir -p "$SANDBOX/backups"

pruned=$(prune_snapshots "$SANDBOX/backups" 3)
if [[ "$pruned" == "0" ]]; then
  echo "  PASS: empty dir returns 0 prunes"
else
  echo "  FAIL: expected 0 pruned for empty dir, got $pruned" >&2
  failures=$((failures + 1))
fi
rm -rf "$SANDBOX"

# ── Test 4: KEEP=0 (forensic mode) → no prune even with many snapshots
echo "Test 4: KEEP=0 (forensic mode) → no pruning"
SANDBOX=$(mktemp -d)
trap 'rm -rf "$SANDBOX"' EXIT
mkdir -p "$SANDBOX/backups"
for ts in A B C D E; do mkdir -p "$SANDBOX/backups/pre-upgrade-$ts"; done

pruned=$(prune_snapshots "$SANDBOX/backups" 0)
if [[ "$pruned" == "0" ]]; then
  echo "  PASS: KEEP=0 disables pruning"
else
  echo "  FAIL: expected 0 pruned with KEEP=0, got $pruned" >&2
  failures=$((failures + 1))
fi
assert_count "all 5 snapshots kept" "$SANDBOX/backups/pre-upgrade-*" 5
rm -rf "$SANDBOX"

# ── Test 5: non-snapshot dirs in backups → not pruned ────────────────
echo "Test 5: only files matching pre-upgrade-* glob are pruned"
SANDBOX=$(mktemp -d)
trap 'rm -rf "$SANDBOX"' EXIT
mkdir -p "$SANDBOX/backups/pre-upgrade-A" "$SANDBOX/backups/pre-upgrade-B" \
         "$SANDBOX/backups/pre-upgrade-C" "$SANDBOX/backups/pre-upgrade-D"
mkdir -p "$SANDBOX/backups/manual-export-2026"
echo "human readme" > "$SANDBOX/backups/README.txt"

prune_snapshots "$SANDBOX/backups" 2 >/dev/null

assert_exists "manual-export-* untouched" "$SANDBOX/backups/manual-export-2026"
assert_exists "README.txt untouched"      "$SANDBOX/backups/README.txt"
rm -rf "$SANDBOX"

echo
if [[ "$failures" == "0" ]]; then
  echo "OK: all checks passed"
  exit 0
else
  echo "FAIL: $failures failure(s)" >&2
  exit 1
fi
