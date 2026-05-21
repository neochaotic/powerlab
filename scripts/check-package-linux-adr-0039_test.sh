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
#
# Follow-up (ADR-0039 enforcement pass): the Umbrel ingestion tooling
# (backend/sync-catalog + the weekly sync workflow) was removed entirely —
# the catalog is now sourced solely from the powerlab-store repo via
# scripts/bundle-store.sh at release time. Operators who want a different
# catalog register a custom source via Settings → Catalog. This test now
# also asserts the packaging script no longer builds/ships sync-catalog.

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

echo "Test: post-install does NOT auto-invoke any catalog sync (ADR-0039)"
# Auto-invocation re-pulled Umbrel upstream every install and re-populated
# the curated catalog dir with the filtered upstream contents. ADR-0039
# requires the bundled curated set be the sole source of truth on a
# default install.
assert_not_grep "no automatic timeout 60 sync invocation" \
  'timeout 60 /usr/bin/powerlab-sync-catalog'
assert_not_grep "no POWERLAB_SKIP_SYNC env knob" \
  'POWERLAB_SKIP_SYNC:-0'
assert_not_grep "no 'community catalog refreshed' echo from auto-sync" \
  'community catalog refreshed'

echo "Test: sync-catalog is fully removed from packaging (ADR-0039 enforcement)"
# The Umbrel ingestion binary was removed — the catalog is sourced from
# the powerlab-store repo via bundle-store.sh, not built/shipped here.
assert_not_grep "no sync-catalog cross-compile step" \
  "backend/sync-catalog"
assert_not_grep "no sync-catalog binary output to STAGE/bin" \
  "powerlab-sync-catalog"

echo "Test: backend/sync-catalog module no longer exists"
if [[ -d "$REPO_ROOT/backend/sync-catalog" ]]; then
  echo "  FAIL: backend/sync-catalog still present — should be removed" >&2
  failures=$((failures + 1))
else
  echo "  PASS: backend/sync-catalog removed"
fi

echo
if [[ "$failures" == "0" ]]; then
  echo "OK: all checks passed"
  exit 0
else
  echo "FAIL: $failures failure(s)" >&2
  exit 1
fi
