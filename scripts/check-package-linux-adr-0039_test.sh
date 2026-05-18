#!/usr/bin/env bash
# ADR-0039 enforcement (Sprint 23, issue #450).
#
# v0.7.0 declared "PowerLab ships a native curated catalog, no upstream
# tracking." Two install-path bugs broke this:
#
#   1. cp -R "$HERE/community-catalog"/. /var/lib/powerlab/community-catalog/
#      was an OVERLAY copy. On upgrade from v0.6.x (which shipped 241 apps),
#      the 4 v0.7.x curated apps got added but the 237 prior apps remained
#      orphaned on disk.
#
#   2. Post-install AUTO-invoked `powerlab-sync-catalog` with default
#      `--upstream https://github.com/getumbrel/umbrel-apps.git`. Every
#      install/upgrade re-pulled Umbrel + repopulated the catalog with
#      whatever passed the hook filter — flipping ADR-0039 off in
#      practice.
#
# This test locks the post-fix contract: install ships ONLY the bundled
# curated set; post-install does NOT silently sync from any upstream.
# Operators who want to mirror an upstream catalog must register a custom
# catalog source explicitly via Settings → Catalog (the ADR-0039 escape
# hatch) or run /usr/bin/powerlab-sync-catalog by hand with their chosen
# upstream + output.

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

assert_not_grep() {
  local description="$1"
  local pattern="$2"
  if grep -q -F -- "$pattern" "$TARGET"; then
    echo "  FAIL: $description (pattern '$pattern' should NOT appear)" >&2
    failures=$((failures + 1))
  else
    echo "  PASS: $description"
  fi
}

echo "Test: community-catalog install wipes Apps/ before copying (ADR-0039)"
# Wipe-then-copy semantics: the install script must delete the
# previously-installed Apps/ tree before re-populating from the tarball.
# Without this, an upgrade from v0.6.x (241 apps) leaves 237 orphans
# even after the curated set drops to 4.
assert_grep "wipes /var/lib/powerlab/community-catalog/Apps before copy" \
  'rm -rf /var/lib/powerlab/community-catalog/Apps'

echo "Test: post-install does NOT auto-invoke powerlab-sync-catalog (ADR-0039)"
# Auto-invocation re-pulled Umbrel upstream every install and re-populated
# the curated catalog dir with the filtered upstream contents. ADR-0039
# requires the curated set be the sole source of truth on a default
# install. The binary stays shipped for explicit operator use, but the
# install script must NOT call it.
assert_not_grep "no automatic timeout 60 sync invocation" \
  'timeout 60 /usr/bin/powerlab-sync-catalog'
assert_not_grep "no POWERLAB_SKIP_SYNC env knob" \
  'POWERLAB_SKIP_SYNC:-0'
assert_not_grep "no 'community catalog refreshed' echo from auto-sync" \
  'community catalog refreshed'

echo "Test: powerlab-sync-catalog binary is still shipped for maintainer/operator use"
# We do NOT remove the binary — operators can register a custom catalog
# source via the UI escape hatch, and maintainers use it locally for the
# curation pipeline. Just the auto-invocation goes away.
assert_grep "sync-catalog cross-compile step still present" \
  "cd \"\$ROOT/backend/sync-catalog\""
assert_grep "sync-catalog binary still output to STAGE/bin" \
  "-o \"\$STAGE/bin/powerlab-sync-catalog\""

echo "Test: backend/sync-catalog package still compiles"
if (cd "$REPO_ROOT/backend/sync-catalog" && go build -o /dev/null . 2>/dev/null); then
  echo "  PASS: backend/sync-catalog compiles"
else
  echo "  FAIL: backend/sync-catalog failed to compile" >&2
  failures=$((failures + 1))
fi

echo
if [[ "$failures" == "0" ]]; then
  echo "OK: all checks passed"
  exit 0
else
  echo "FAIL: $failures failure(s)" >&2
  exit 1
fi
