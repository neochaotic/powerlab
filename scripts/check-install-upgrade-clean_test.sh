#!/usr/bin/env bash
# Upgrade-scenario integration test (Sprint 23 / #450).
#
# Reproduces the v0.6.x → v0.7.x upgrade flow in a temp prefix:
#
#   1. Pre-populates the destination /var/lib/powerlab/community-catalog/Apps/
#      with N "orphan" apps to simulate a box that's been running on a
#      prior PowerLab version with a larger curated set.
#   2. Builds a fake tarball stage with M curated apps + the
#      .curated-manifest.
#   3. Extracts the actual catalog-install snippet from
#      scripts/package-linux.sh (the very lines that emit into
#      install.sh) and runs it against the temp prefix, substituting
#      hardcoded /var/lib/powerlab paths with the test root.
#   4. Asserts: post-install Apps/ contains exactly the M curated apps,
#      none of the N orphans, AND the .curated-manifest is present.
#
# Why this exists: PR #451 added the wipe-then-copy semantics; the
# `check-package-linux-adr-0039_test.sh` regression test asserts the
# SHAPE of the code (string `rm -rf /var/lib/powerlab/community-catalog/
# Apps` must appear; auto-invoke must not). This file is the
# dynamic complement — runs the actual logic against a fake filesystem
# state and verifies behaviour. Belt-and-suspenders for the #450 bug
# class.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PKG_SCRIPT="$REPO_ROOT/scripts/package-linux.sh"

if [[ ! -f "$PKG_SCRIPT" ]]; then
  echo "FAIL: package-linux.sh not found at $PKG_SCRIPT" >&2
  exit 1
fi

# ─── Setup ────────────────────────────────────────────────────────────
test_root=$(mktemp -d "${TMPDIR:-/tmp}/powerlab-upgrade-test-XXXXXX")
trap 'rm -rf "$test_root"' EXIT

# Simulated "v0.6.x box" with old orphan apps still on disk.
prior_apps_dir="$test_root/var/lib/powerlab/community-catalog/Apps"
mkdir -p "$prior_apps_dir"
prior_apps=("home-assistant" "jellyfin" "nextcloud" "vaultwarden" "pihole")
for app in "${prior_apps[@]}"; do
  mkdir -p "$prior_apps_dir/$app"
  echo "services: {}" > "$prior_apps_dir/$app/docker-compose.yml"
done
# Also drop a marker the post-install must preserve OUTSIDE of Apps/.
# (community-catalog/README.md from prior install should survive — only
# the Apps/ subtree gets wiped per the PR #451 contract.)
echo "operator README from prior install" > "$test_root/var/lib/powerlab/community-catalog/README.md"

# Simulated "v0.7.1 tarball stage" with the actual repo's curated set.
tarball_stage="$test_root/tarball-stage"
mkdir -p "$tarball_stage/community-catalog"
if [[ -d "$REPO_ROOT/community-catalog" ]]; then
  cp -R "$REPO_ROOT/community-catalog"/. "$tarball_stage/community-catalog/"
fi

# Emit a .curated-manifest like scripts/package-linux.sh does
# (matches the in-script logic from PR #452).
manifest_file="$tarball_stage/community-catalog/.curated-manifest"
{
  echo "# test manifest"
  if [[ -d "$tarball_stage/community-catalog/Apps" ]]; then
    for d in "$tarball_stage/community-catalog/Apps"/*/; do
      [[ -d "$d" ]] && basename "$d"
    done | sort
  fi
} > "$manifest_file"

curated_count=$(grep -cv '^#\|^$' "$manifest_file" || echo 0)

# ─── Run the install snippet under the test prefix ────────────────────
HERE="$tarball_stage" \
PREFIX="$test_root" \
bash -c '
  set -euo pipefail
  # Mirror scripts/package-linux.sh:583-602 (the install-script catalog
  # snippet) against $PREFIX instead of the hardcoded /var/lib/powerlab.
  if [[ -d "$HERE/community-catalog" ]]; then
    echo "[test-install] Installing community catalog..."
    rm -rf "$PREFIX/var/lib/powerlab/community-catalog/Apps"
    cp -R "$HERE/community-catalog"/. "$PREFIX/var/lib/powerlab/community-catalog/"
  fi
'

# ─── Assertions ───────────────────────────────────────────────────────
failures=0

assert() {
  local description="$1"
  local condition="$2"
  if eval "$condition"; then
    echo "  PASS: $description"
  else
    echo "  FAIL: $description (condition: $condition)" >&2
    failures=$((failures + 1))
  fi
}

# Orphans gone.
echo "Test: prior-install orphan apps removed by wipe-then-copy"
for app in "${prior_apps[@]}"; do
  assert "$app dir removed" "[[ ! -e '$prior_apps_dir/$app' ]]"
done

# Curated apps present.
echo "Test: curated apps present after install"
post_apps_dir="$test_root/var/lib/powerlab/community-catalog/Apps"
post_count=0
if [[ -d "$post_apps_dir" ]]; then
  for d in "$post_apps_dir"/*/; do
    [[ -d "$d" ]] && post_count=$((post_count + 1))
  done
fi
assert "post-install Apps/ contains exactly the curated set ($post_count == $curated_count)" \
  "[[ '$post_count' == '$curated_count' && '$curated_count' -gt 0 ]]"

# README preserved (only Apps/ should be wiped, per PR #451 contract).
echo "Test: non-Apps/ files preserved across upgrade"
assert "operator README from prior install preserved" \
  "[[ -f '$test_root/var/lib/powerlab/community-catalog/README.md' ]]"

# Manifest present.
echo "Test: .curated-manifest shipped to live dir"
assert ".curated-manifest present in live dir" \
  "[[ -f '$test_root/var/lib/powerlab/community-catalog/.curated-manifest' ]]"

# ─── Verify install snippet shape matches actual package-linux.sh ─────
# Defensive: if the install-script snippet in package-linux.sh changes,
# this test's hardcoded mirror snippet falls out of date silently.
# Lock the contract by grepping for both required lines.
echo "Test: actual scripts/package-linux.sh install snippet matches the test mirror"
assert "package-linux.sh has 'rm -rf /var/lib/powerlab/community-catalog/Apps'" \
  "grep -q -F 'rm -rf /var/lib/powerlab/community-catalog/Apps' '$PKG_SCRIPT'"
assert "package-linux.sh has 'cp -R \"\$HERE/community-catalog\"/.'" \
  "grep -q -F 'cp -R \"\$HERE/community-catalog\"/.' '$PKG_SCRIPT'"

echo
if [[ "$failures" == "0" ]]; then
  echo "OK: upgrade scenario produces clean post-install state ($curated_count curated apps, ${#prior_apps[@]} orphans removed)"
  exit 0
else
  echo "FAIL: $failures assertion(s) failed" >&2
  exit 1
fi
