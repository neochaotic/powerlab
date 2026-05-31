#!/usr/bin/env bash
# CI gate: release-manifest.yaml `summary` length must be ≤ 1000 chars.
#
# Spec (docs/UPDATE_MANIFEST.md):
#   "summary (string, required, ≤ 1000 chars) — Plain-text summary
#    for the 'Update available' surface in AboutPane. UI shows ~240
#    chars inline and offers a 'Show more' toggle for the rest."
#
# Historically not enforced anywhere. v0.6.6 published an 11k-char
# summary that rendered as a wall of text in AboutPane (the trigger
# for this gate). The summary kept growing because the cut process
# appended "Sprint N framing (preserved for context):" blocks every
# release without trimming. v0.7.5 raised the cap from 250 → 1000 so
# feature-bundle cuts can include enough context to motivate the
# upgrade; the UI's defensive truncation at ~240 chars + Show more
# (ui/src/lib/utils/release-notes-summary.ts) keeps the toast itself
# readable while the expanded view shows the full text.
#
# This gate fails CI loud if a manifest blows past 1000 chars (the
# wall-of-text floor from the v0.6.6 incident).

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MANIFEST="$REPO_ROOT/release-manifest.yaml"
MAX_CHARS=1000

if [[ ! -f "$MANIFEST" ]]; then
  echo "FAIL: $MANIFEST not found"
  exit 1
fi

# Extract the YAML block scalar that follows `summary: |`. AWK reads
# from the marker line; once we hit the next top-level key (line
# matching ^[a-zA-Z_]+:), we stop. Stripping the 2-space block-scalar
# indent gives us the literal text the manifest renderer ships.
SUMMARY=$(awk '
  /^summary: \|/ { capturing=1; next }
  # A YAML block scalar ends when we hit a line that is NOT blank
  # and does NOT start with the 2-space indent the scalar uses.
  # That covers: a sibling top-level key, a comment at column 0,
  # or end-of-file.
  capturing && /^[^ ]/ { capturing=0 }
  capturing { sub(/^  /, ""); print }
' "$MANIFEST")

# Strip trailing blank lines.
SUMMARY_TRIMMED=$(printf '%s' "$SUMMARY" | awk 'BEGIN{n=0} /^$/{blank[++n]=1; next} {for(i=1;i<=n;i++) print ""; n=0; print}')
LEN=${#SUMMARY_TRIMMED}

if [[ "$LEN" -le "$MAX_CHARS" ]]; then
  echo "OK: release-manifest.yaml summary is $LEN chars (≤ $MAX_CHARS)."
  exit 0
fi

echo "FAIL: release-manifest.yaml summary is $LEN chars — spec is ≤ $MAX_CHARS." >&2
echo "      docs/UPDATE_MANIFEST.md describes the summary field as:" >&2
echo "        \"One-paragraph plain-text summary for the 'Update available' toast.\"" >&2
echo "      Trim the summary BEFORE the tag. The full release notes live in CHANGELOG.md;" >&2
echo "      the manifest summary is the one-line user-facing blurb only." >&2
echo "" >&2
echo "      Current summary:" >&2
echo "      ─────────────────────────────────────────────────────────────────" >&2
echo "$SUMMARY_TRIMMED" | head -20 >&2
exit 1
