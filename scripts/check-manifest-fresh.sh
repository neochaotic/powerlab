#!/usr/bin/env bash
# Pre-tag check: refuse to proceed when release-manifest.yaml's `summary`
# field is identical to the most recently published GitHub release.
#
# Why this exists:
#   v0.5.4 was tagged with the v0.5.0 summary text still in the YAML
#   ("Settings → change gateway port from the UI. mDNS coexistence...").
#   The manifest.json shipped wrong; the in-UI updater showed stale text.
#   Hot-fixed via `gh release upload v0.5.4 manifest.json --clobber` post-hoc.
#   See issue #156.
#
# Usage:
#   ./scripts/check-manifest-fresh.sh                 # against the actual latest release on GitHub
#   ./scripts/check-manifest-fresh.sh <fixture.json>  # against a local fixture (used by tests)
#
# Exit codes:
#   0  — summary differs from latest released version → safe to tag
#   1  — summary is identical → STOP, update release-manifest.yaml first
#   2  — could not determine state (network down, missing yq, etc.)

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
YAML_PATH="$REPO_ROOT/release-manifest.yaml"

if [[ ! -f "$YAML_PATH" ]]; then
  echo "ERROR: $YAML_PATH not found" >&2
  exit 2
fi

# Pull the local YAML summary. We don't depend on yq — sed against the
# `summary: |` block is good enough for the one-field check we do here.
# If `summary:` is on a single line it works; if it's a `|` literal
# block it works (we strip leading whitespace from continuation lines).
local_summary=$(awk '
  /^summary:[[:space:]]*\|/ { capturing = 1; next }
  /^summary:[[:space:]]*"/ { sub(/^summary:[[:space:]]*"/, ""); sub(/"[[:space:]]*$/, ""); print; exit }
  /^summary:[[:space:]]*[^|"]/ { sub(/^summary:[[:space:]]*/, ""); print; exit }
  capturing && /^[[:space:]]/ { sub(/^[[:space:]]+/, ""); printf "%s ", $0; next }
  capturing && /^[^[:space:]]/ { exit }
' "$YAML_PATH")

if [[ -z "$local_summary" ]]; then
  echo "ERROR: could not parse summary from $YAML_PATH" >&2
  exit 2
fi

# Optional fixture argument lets tests run against a known JSON file
# instead of hitting GitHub. The fixture must be the manifest.json of
# the most recent release we want to compare against.
if [[ "${1:-}" != "" ]]; then
  fixture="$1"
  if [[ ! -f "$fixture" ]]; then
    echo "ERROR: fixture $fixture not found" >&2
    exit 2
  fi
  remote_summary=$(jq -r '.summary // ""' "$fixture" | tr -d '\n')
else
  # Production path: hit GitHub releases/latest.
  if ! command -v gh >/dev/null 2>&1; then
    echo "ERROR: gh CLI is required (install via 'brew install gh')" >&2
    exit 2
  fi
  if ! command -v jq >/dev/null 2>&1; then
    echo "ERROR: jq is required (install via 'brew install jq')" >&2
    exit 2
  fi
  latest_tag=$(gh release view --json tagName -q .tagName 2>/dev/null || true)
  if [[ -z "$latest_tag" ]]; then
    echo "WARNING: no published releases found — first release, allowing." >&2
    exit 0
  fi
  manifest_url="https://github.com/neochaotic/powerlab/releases/download/$latest_tag/manifest.json"
  remote_summary=$(curl -fsSL "$manifest_url" 2>/dev/null | jq -r '.summary // ""' | tr -d '\n' || true)
  if [[ -z "$remote_summary" ]]; then
    echo "WARNING: could not fetch latest manifest from $manifest_url — skipping check." >&2
    exit 0
  fi
fi

local_normalized=$(echo "$local_summary" | tr -s '[:space:]' ' ' | sed -e 's/^ //' -e 's/ $//')
remote_normalized=$(echo "$remote_summary" | tr -s '[:space:]' ' ' | sed -e 's/^ //' -e 's/ $//')

if [[ "$local_normalized" == "$remote_normalized" ]]; then
  cat <<EOF >&2
ERROR: release-manifest.yaml summary is identical to the previously
published release. Update the summary in release-manifest.yaml before
tagging — otherwise the in-UI updater will show stale text to users.

Latest published summary:
  $remote_summary

This caused the v0.5.4 mishap (issue #156). See docs/UPDATE_MANIFEST.md
for the maintainer instructions.
EOF
  exit 1
fi

echo "OK: release-manifest.yaml summary differs from latest published release."
exit 0
