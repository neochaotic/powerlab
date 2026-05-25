#!/usr/bin/env bash
# prepare-release.sh — stage all release-time edits atomically.
#
# Replaces the manual cut checklist:
#   1. changie batch v<X.Y.Z>
#   2. changie merge
#   3. cd ui && npm version <X.Y.Z> --no-git-tag-version
#   4. (manually) edit release-manifest.yaml summary
#   5. git add ...
#
# Now: ./scripts/prepare-release.sh 0.6.7
#
# What it does:
#   - changie batch + merge (produces .changes/v<X.Y.Z>.md + CHANGELOG.md
#     append; deletes consumed unreleased fragments)
#   - cd ui && npm version (so package.json's version stays in sync
#     with git tags — without this the L1+L2 defense from the v0.6.6
#     retro is half-blind)
#   - git stages all touched files
#
# What it does NOT do (intentional — release tagging is gated on
# user authorization per the project's release discipline):
#   - git commit
#   - git push
#   - git tag
#   - update release-manifest.yaml summary (semantic content, manual)
#
# After this script runs, the user reviews `git status`, edits the
# manifest summary, then commits + tags themselves.

set -euo pipefail

VERSION="${1:-}"
if [[ -z "$VERSION" ]]; then
  echo "usage: $0 <version>  (e.g. $0 0.6.7)" >&2
  exit 2
fi

# Strip leading 'v' if user passed it that way.
VERSION="${VERSION#v}"

# Validate semver-ish.
if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[A-Za-z0-9.-]+)?$ ]]; then
  echo "ERROR: '$VERSION' does not look like semver (e.g. 0.6.7 or 0.6.7-rc.1)." >&2
  exit 2
fi

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

log() { echo "[prepare-release] $*"; }

# Confirm clean tree on the relevant paths.
if ! git diff --quiet HEAD -- .changes/ ui/package.json CHANGELOG.md release-manifest.yaml 2>/dev/null; then
  echo "ERROR: working tree has uncommitted changes in release-touching paths." >&2
  echo "       Commit or stash before running prepare-release." >&2
  git status --short -- .changes/ ui/package.json CHANGELOG.md release-manifest.yaml >&2
  exit 1
fi

log "Preparing release v$VERSION..."

# Step 1: changie batch + merge.
if ! command -v changie >/dev/null; then
  echo "ERROR: 'changie' not found in PATH (install via 'brew install changie' or download from https://changie.dev)." >&2
  exit 1
fi

# Refuse to re-cut a version that is ALREADY a published release (the
# v0.7.2 collision: a published number kept accruing content via
# hand-edited changelog instead of bumping). `changie batch` below also
# refuses if .changes/vX.md exists, but that misses the
# "published + someone edited the batched file" path — check explicitly.
log "Verifying v$VERSION is not already a published release..."
bash "$REPO_ROOT/scripts/check-version-not-released.sh" "$VERSION"

log "Running 'changie batch v$VERSION'..."
changie batch "v$VERSION"

log "Running 'changie merge'..."
changie merge

# Step 2: bump ui/package.json so it tracks the release tag.
# `--allow-same-version` makes this safe to re-run.
log "Bumping ui/package.json to $VERSION..."
(cd ui && npm version "$VERSION" --no-git-tag-version --allow-same-version > /dev/null 2>&1 || true)

# Step 3: stage everything we touched.
git add .changes ui/package.json CHANGELOG.md release-manifest.yaml 2>/dev/null || true

log "Done. Next steps:"
log "  1. Edit release-manifest.yaml 'summary' field with the user-facing note for this cut"
log "  2. Review 'git diff --cached' and 'git status'"
log "  3. Commit (chore(release): v$VERSION — <hook>)"
log "  4. Push to main, await explicit user authorization, then tag + push tag"
log ""
log "Defense layers active:"
log "  L1 — scripts/package-linux.sh syncs pkg.json on every build"
log "  L1.5 — this script bumped pkg.json on main (above)"
log "  L2 — scripts/check-ui-package-version-fresh_test.sh gates CI on staleness"
log "  L3 — scripts/package-linux.sh sanity-greps built JS for the version literal"
