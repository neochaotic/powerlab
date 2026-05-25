#!/usr/bin/env bash
# check-version-not-released.sh — refuse to (re)cut a version number that
# is ALREADY a published GitHub release.
#
# Background: v0.7.2 was batched + published, then a later work stream
# kept appending content to the same v0.7.2 changelog instead of bumping
# to v0.7.3 — re-using a published, immutable version number. The
# `changie batch` "version already exists" refusal was the signal to
# bump, but it was worked around by hand-editing the batched file. This
# gate makes the collision impossible to miss: if `v<version>` is a
# published release, prepare-release.sh aborts here and tells you to use
# the next number.
#
# Usage:  scripts/check-version-not-released.sh <version>   # e.g. 0.7.3
#
# Exit codes:
#   0  — version is NOT a published release (safe to cut)
#   1  — version IS already published (collision — bump to the next)
#   2  — usage error
#
# Test hook: set POWERLAB_PUBLISHED_RELEASES_OVERRIDE to a space-separated
# list of published tags (e.g. "v0.7.1 v0.7.2") to bypass the live `gh`
# lookup — used by check-version-not-released_test.sh for hermetic tests.

set -euo pipefail

VERSION="${1:-}"
if [[ -z "$VERSION" ]]; then
  echo "usage: $0 <version>  (e.g. $0 0.7.3)" >&2
  exit 2
fi
TAG="v${VERSION#v}"

# release_is_published <tag> — true (0) iff <tag> is a published (non-draft)
# GitHub release. Honors the test override; otherwise queries gh. If gh is
# unavailable (no network/auth), we cannot prove a collision — warn and
# treat as not-published so local dev without gh is not hard-blocked (the
# CI/maintainer path has gh and gets the real check).
release_is_published() {
  local tag="$1"
  if [[ -n "${POWERLAB_PUBLISHED_RELEASES_OVERRIDE+x}" ]]; then
    grep -qw -- "$tag" <<<"${POWERLAB_PUBLISHED_RELEASES_OVERRIDE}"
    return
  fi
  if ! command -v gh >/dev/null 2>&1; then
    echo "[check-version-not-released] WARNING: gh not available — cannot verify $tag against published releases; skipping." >&2
    return 1
  fi
  local draft
  draft="$(gh release view "$tag" --json isDraft -q .isDraft 2>/dev/null)" || return 1
  [[ "$draft" == "false" ]]
}

if release_is_published "$TAG"; then
  echo "ERROR: $TAG is already a published release — you cannot re-cut it." >&2
  echo "       A published tag is immutable. Bump to the next version (the" >&2
  echo "       changie 'version already exists' refusal means the same thing)." >&2
  exit 1
fi

echo "[check-version-not-released] OK: $TAG is not a published release."
