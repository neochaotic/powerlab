#!/usr/bin/env bash
# CI gate: detect legacy docker-compose v1 hostname patterns in catalog
# entries. Pattern: `<project>_<service>_<idx>` in env-var values,
# typically used by upstream CasaOS/Umbrel entries to reference sister
# services (DB host, Redis host, etc.).
#
# Why this exists:
#   PowerLab ships docker-compose v2, which creates containers with
#   hyphens (`<project>-<service>-<idx>`) and exposes each service
#   as a network alias (`<service>`). The legacy underscore form
#   never resolves under v2 → app crashes in DNS-error loop. Sprint
#   20 PR 2 (#385 Activepieces) discovered 67 catalog apps with the
#   same latent bug.
#
# Usage:
#   ./scripts/check-catalog-hostnames.sh                 # warn-only (default)
#   POWERLAB_CATALOG_LINT_STRICT=1 ./scripts/check-catalog-hostnames.sh
#                                                        # hard-fail on any finding
#   ./scripts/check-catalog-hostnames.sh <path>          # scan a single file
#
# Exit codes:
#   0  — no findings, or warn-only mode
#   1  — strict mode + findings detected
#   2  — invalid invocation
#
# Output format (per finding):
#   <relative-path>:<line>: <project>_<service>_<idx> -> use <service>
#
# Affected apps are tracked in #402.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CATALOG_DIR="$REPO_ROOT/community-catalog/Apps"
STRICT="${POWERLAB_CATALOG_LINT_STRICT:-0}"

if [[ ! -d "$CATALOG_DIR" ]]; then
  echo "ERROR: $CATALOG_DIR not found" >&2
  exit 2
fi

scan_file() {
  local f="$1"
  local relpath="${f#$REPO_ROOT/}"
  # First line of the compose: `name: <project>`. Ignore comments.
  local project
  project="$(grep -m1 -E "^name:" "$f" | awk '{print $2}' | tr -d '"' || true)"
  if [[ -z "$project" ]]; then
    return 0
  fi

  # Find `<project>_<service>_<idx>` patterns inside env values. Match:
  #   - same project name (anchored, no false-positives across apps)
  #   - underscore + service token + underscore + 1+ digits
  # The regex extracts the matched substring so the report shows what
  # to fix on each line.
  local pattern="${project}_[a-z][a-z0-9]*_[0-9]+"
  local hits
  hits="$(grep -nE "$pattern" "$f" || true)"
  if [[ -z "$hits" ]]; then
    return 0
  fi

  while IFS= read -r line; do
    local lineno="${line%%:*}"
    local content="${line#*:}"
    local match
    match="$(printf '%s' "$content" | grep -oE "$pattern" | head -1)"
    local service
    service="$(printf '%s' "$match" | sed -E "s/^${project}_(.+)_[0-9]+$/\1/")"
    printf '%s:%s: %s -> use %s (service-name network alias)\n' \
      "$relpath" "$lineno" "$match" "$service"
  done <<< "$hits"
}

main() {
  local files=()
  if [[ $# -gt 0 ]]; then
    files=("$@")
  else
    while IFS= read -r f; do
      files+=("$f")
    done < <(find "$CATALOG_DIR" -name "docker-compose.yml")
  fi

  local findings=0
  local files_with_findings=0
  for f in "${files[@]}"; do
    local out
    out="$(scan_file "$f")"
    if [[ -n "$out" ]]; then
      echo "$out"
      # NB: avoid `((x++))` here. Post-increment returns the OLD
      # value, so when files_with_findings starts at 0 the
      # arithmetic exits with status 1 and `set -e` kills the
      # script before the rest of the loop runs. The assignment
      # form always returns 0.
      files_with_findings=$(( files_with_findings + 1 ))
      findings=$(( findings + $(echo "$out" | wc -l) ))
    fi
  done

  echo "" >&2
  if [[ "$findings" -eq 0 ]]; then
    echo "OK: 0 hostname findings across $(printf '%s\n' "${files[@]}" | wc -l) catalog file(s)" >&2
    exit 0
  fi

  echo "Found $findings hostname finding(s) across $files_with_findings catalog file(s)" >&2
  echo "Fix: replace \`<project>_<svc>_<idx>\` with the service-name alias (e.g. \`db\`, \`redis\`)" >&2
  echo "Tracking: https://github.com/neochaotic/powerlab/issues/402" >&2

  if [[ "$STRICT" -eq 1 ]]; then
    echo "FAIL: strict mode" >&2
    exit 1
  fi
  echo "(warn-only; set POWERLAB_CATALOG_LINT_STRICT=1 to fail)" >&2
  exit 0
}

main "$@"
