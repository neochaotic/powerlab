#!/usr/bin/env bash
# Regenerate docs/img/<name>.png screenshots by driving Playwright
# through the mocked UI. Outputs land in docs/img/ — review the diff
# with `git diff --stat docs/img/` before committing.
#
# This is the convenience wrapper around `npm run screenshots`. Use
# it when:
#   - A UI change touches one of the pages the README/docs reference
#   - Before opening a PR that updates README copy (so the visuals
#     match the new copy)
#   - On a periodic basis (~release cadence) for visual freshness
#
# SCOPE — what gets regenerated:
#   docs/img/login.png       — unauthenticated login screen
#   docs/img/dashboard.png   — authenticated dashboard
#   docs/img/apps.png        — apps page (uses baseline mock — installed apps surface)
#   docs/img/files.png       — file manager
#   docs/img/about.png       — Settings → About pane
#   docs/img/launchpad.png   — top-level launchpad
#
# What this DOES NOT regenerate:
#   - Store/catalog screenshots — the community-catalog is an opt-in
#     install step; meaningful store-pane screenshots need either a
#     seeded mock or a real install with catalog enabled. Tracked as
#     a follow-up to this screenshot infra.
#   - GPU dashboard close-up (gpu_dashboard.png) — needs real GPU
#     telemetry; capture manually after running on a GPU-equipped host.
#   - social-preview.png — designer-generated, not derived from UI.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if ! command -v npx &> /dev/null; then
  echo "ERROR: npx not on PATH. Install Node.js + npm first."
  exit 1
fi

echo "[regen-screenshots] Running Playwright screenshots spec..."
cd "$REPO_ROOT/ui"
npm run screenshots

echo
echo "[regen-screenshots] Done. Review with:"
echo "  git diff --stat docs/img/"
echo
echo "  # If diffs look intentional:"
echo "  git add docs/img/ && git commit -m 'docs: regen screenshots'"
