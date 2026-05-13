#!/usr/bin/env bash
# Verifies that a built UI bundle (build/_app/immutable/) contains
# the expected version literal. Called by scripts/package-linux.sh
# as the L3 defense-in-depth check (v0.6.6 retro — see
# docs/UPDATE_MANIFEST.md "Defense in depth" section).
#
# Usage:
#   check-built-ui-version.sh <VERSION> <BUILD_DIR>
#
# Exits 0 if the version literal is present in any chunk/node JS.
# Exits 1 with a descriptive error otherwise.
#
# Rollup's minifier strips JSON.stringify quotes around define()'d
# values, so the literal appears bare in the chunks (e.g.,
# `0.6.6` not `"0.6.6"`). The grep matches that bare form.
#
# Tested by scripts/check-built-ui-version_test.sh with both
# positive (correct version) and negative (injected wrong version)
# sandboxed build trees.

set -euo pipefail

VERSION="${1:-}"
BUILD_DIR="${2:-}"

if [[ -z "$VERSION" || -z "$BUILD_DIR" ]]; then
  echo "usage: $0 <VERSION> <BUILD_DIR>" >&2
  exit 2
fi

if [[ ! -d "$BUILD_DIR/_app/immutable" ]]; then
  echo "ERROR: $BUILD_DIR/_app/immutable not found. Did 'npm run build' run?" >&2
  exit 1
fi

if ! grep -rqF -- "$VERSION" "$BUILD_DIR/_app/immutable/chunks/" "$BUILD_DIR/_app/immutable/nodes/" 2>/dev/null; then
  echo "ERROR: Built UI bundle in $BUILD_DIR does not contain expected version literal '$VERSION'." >&2
  echo "       This means __APP_VERSION__ was stamped with a different value (probably stale pkg.json" >&2
  echo "       or POWERLAB_VERSION env got lost). Aborting before sealing tarball — see" >&2
  echo "       docs/UPDATE_MANIFEST.md 'Defense in depth' section." >&2
  exit 1
fi

echo "OK: built UI bundle contains version literal '$VERSION'."
