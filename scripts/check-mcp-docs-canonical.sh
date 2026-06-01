#!/usr/bin/env bash
# check-mcp-docs-canonical.sh — ADR-0050 enforcement gate.
#
# ADR-0050 declares MCP-served documentation as the canonical source
# of truth for agent-visible behavior; code is implementation. This
# script catches DRIFT between the two by comparing what the
# app-management parser actually accepts against what the canonical
# MCP-served doc (docs/concepts/compose-conventions.md) documents.
#
# Scope today: the compose-extension key family ONLY (x-powerlab +
# legacy aliases). Follow-up PRs can extend the same shape to:
#   - OpenAPI paths vs route registrations
#   - x-powerlab subfield names (title, icon, main, etc.)
#   - catalog metadata fields vs powerlab-store/validate.py
#
# Behaviour:
#   - Extract the extension keys the parser literally reads (grepping
#     for the documented family from extension.go).
#   - For each, assert the canonical doc mentions it. A "mention" is
#     a literal substring match — backward-compat / deprecated keys
#     just need to be named somewhere in the doc so a contributor
#     reading the doc isn't surprised.
#   - Fail with a clear message that names the missing keys + points
#     at both files. Pass silently when in sync.
#
# Exit codes:
#   0 — in sync
#   1 — drift detected (parser accepts a key the doc doesn't mention)
#   2 — script setup error (one of the source files missing)

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PARSER="$REPO_ROOT/backend/app-management/common/constants.go"
DOC="$REPO_ROOT/docs/concepts/compose-conventions.md"

for f in "$PARSER" "$DOC"; do
  if [ ! -f "$f" ]; then
    echo "FAIL: source file missing: $f" >&2
    echo "      (script needs both the parser and the canonical doc)" >&2
    exit 2
  fi
done

# Extract every distinct `"x-..."` literal string that appears in the
# parser. This is a heuristic — the parser uses these literals as map
# keys when reading the compose document's extensions block, so any
# new alias added to the parser surfaces here automatically.
PARSER_KEYS=$(grep -oE '"x-[a-z0-9_-]+"' "$PARSER" | sort -u | tr -d '"')

if [ -z "$PARSER_KEYS" ]; then
  # No extension keys found — either the parser was refactored to read
  # extensions differently, or the heuristic broke. Either way the
  # operator needs to know.
  echo "FAIL: no x-* extension keys found in $PARSER" >&2
  echo "      (parser refactored? update this script's extraction heuristic)" >&2
  exit 2
fi

MISSING=()
for key in $PARSER_KEYS; do
  if ! grep -q -F "$key" "$DOC"; then
    MISSING+=("$key")
  fi
done

if [ ${#MISSING[@]} -gt 0 ]; then
  cat <<EOF >&2
FAIL: ADR-0050 drift detected — the following compose-extension keys
      are accepted by the parser but NOT mentioned in the canonical doc:

EOF
  for key in "${MISSING[@]}"; do
    echo "        - $key" >&2
  done
  cat <<EOF >&2

      Either:
        a) Add the key (with a one-line "what it is" entry) to
           $DOC
           so a contributor reading the doc knows it exists, OR
        b) Remove the key from the parser if it's no longer needed.

      Per ADR-0050: code may support legacy aliases for backward
      compat, but MCP-served docs MUST mention every key the parser
      accepts. A contributor learning PowerLab through the docs must
      not be ambushed by an undocumented key that "happens to work."
EOF
  exit 1
fi

echo "OK: all $(echo "$PARSER_KEYS" | wc -l | tr -d ' ') extension key(s) accepted by the parser are mentioned in $(basename "$DOC")"
