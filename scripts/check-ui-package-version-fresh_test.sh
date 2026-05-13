#!/usr/bin/env bash
# CI gate: ui/package.json `version` must not be stale relative to the
# latest git tag. Prevents the v0.6.6 retro bug where pkg.json had
# stayed at "0.3.1" across 30+ tags because no automated step kept it
# in sync — and a `npm run build` (no POWERLAB_VERSION env) picked
# up the stale fallback, baking "0.3.1" into the production bundle.
#
# Rule: ui/package.json version SHOULD match the most-recent git tag
# (with leading `v` stripped). Tolerance: if the latest tag is
# unavailable (clean checkout, no tags yet) the check passes; if
# pkg.version is "0.0.0-dev" or similar dev marker, the check passes
# (intentional dev-only setup); otherwise pkg.version must equal the
# latest semver tag.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PKG="$REPO_ROOT/ui/package.json"

if [[ ! -f "$PKG" ]]; then
  echo "FAIL: $PKG not found"
  exit 1
fi

PKG_VERSION=$(grep -E '"version"' "$PKG" | head -1 | sed -E 's/.*"version"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')

if [[ -z "$PKG_VERSION" ]]; then
  echo "FAIL: could not parse version from $PKG"
  exit 1
fi

# Dev-only escape: explicit "0.0.0-dev" or similar marker is fine.
if [[ "$PKG_VERSION" == "0.0.0-dev" ]] || [[ "$PKG_VERSION" == "dev" ]]; then
  echo "OK: ui/package.json is in explicit dev mode ($PKG_VERSION) — skipping freshness gate."
  exit 0
fi

# Use git tags as truth. If no tags exist (shallow clone in CI, fresh
# repo), we can't compare — pass with a warning.
LATEST_TAG=$(cd "$REPO_ROOT" && git tag -l 'v*' --sort=-version:refname | head -1)

if [[ -z "$LATEST_TAG" ]]; then
  echo "WARN: no v* git tags found — skipping freshness gate (likely shallow CI clone)."
  exit 0
fi

LATEST_VERSION="${LATEST_TAG#v}"

if [[ "$PKG_VERSION" == "$LATEST_VERSION" ]]; then
  echo "OK: ui/package.json version ($PKG_VERSION) matches latest tag ($LATEST_TAG)."
  exit 0
fi

# `sort -V` does versionsort: stable, semver-aware. The HIGHER of the
# two versions is the second line. If pkg.version is the highest, the
# release-prep commit has bumped pkg.json AHEAD of the latest tag —
# that's the expected state between the bump-and-commit step and the
# tag-and-push step of a cut. Allow it; the tag will catch up in
# seconds.
HIGHER=$(printf '%s\n%s\n' "$PKG_VERSION" "$LATEST_VERSION" | sort -V | tail -1)
if [[ "$HIGHER" == "$PKG_VERSION" ]]; then
  echo "OK: ui/package.json version ($PKG_VERSION) is ahead of latest tag ($LATEST_TAG) — release-prep state."
  exit 0
fi

# pkg.version is BELOW the latest tag — fail loud with the fix
# instructions. This is the bug class the gate is designed to catch.
echo "FAIL: ui/package.json version is behind the latest tag." >&2
echo "      pkg.json:   $PKG_VERSION" >&2
echo "      latest tag: $LATEST_VERSION ($LATEST_TAG)" >&2
echo "      Fix: cd ui && npm version $LATEST_VERSION --no-git-tag-version --allow-same-version" >&2
echo "      Or rebase main; pkg.json may have been bumped after the tag." >&2
exit 1
