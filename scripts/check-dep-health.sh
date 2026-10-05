#!/usr/bin/env bash
#
# check-dep-health.sh — weekly, WARN-ONLY dependency health scan
# (ADR-0042 §5, #493).
#
# For every backend Go module (or the module dirs given as arguments) it
# runs `go list -m -u -json all` and flags DIRECT dependencies that are:
#   stale  — the release in use is older than DEP_HEALTH_STALE_MONTHS
#            (default 24, the ADR-0042 activity threshold)
#   minor  — a newer minor (or a newer v0.x line) exists on the same path
#   major  — a newer major version exists (<path>/vN+1 resolves)
# Patch-only updates are not flagged (Dependabot handles those).
#
# Output is a Markdown table on stdout; when GITHUB_STEP_SUMMARY is set it
# is appended there too. Always exits 0: this is a report, not a gate.
# Locally-replaced modules (replace => ../x) are skipped.
#
# Note: "stale" uses the publish time of the version in use, which is a
# cheap stand-in for ADR-0042's "last commit on the default branch".
# A stale dep with no newer release is the likely-abandoned case; review
# those by hand and record an ACCEPTED-RISK comment or open an issue.
#
# Usage:
#   scripts/check-dep-health.sh                      # all backend modules
#   scripts/check-dep-health.sh backend/core ...     # specific module dirs
#
# Env:
#   DEP_HEALTH_STALE_MONTHS  threshold in months (default 24)
#   DEP_HEALTH_NOW           "now" as epoch seconds (tests; default: date +%s)
#   DEP_HEALTH_SKIP_MAJOR=1  skip the per-dep next-major probe (faster)
#
# Requires: go, jq.
set -uo pipefail

cd "$(dirname "$0")/.." || exit 0

STALE_MONTHS="${DEP_HEALTH_STALE_MONTHS:-24}"
NOW="${DEP_HEALTH_NOW:-$(date +%s)}"
SKIP_MAJOR="${DEP_HEALTH_SKIP_MAJOR:-0}"

command -v go >/dev/null || { echo "WARN: go not found; dep-health scan skipped." >&2; exit 0; }
command -v jq >/dev/null || { echo "WARN: jq not found; dep-health scan skipped." >&2; exit 0; }

if (( $# > 0 )); then
  modules=("$@")
else
  modules=()
  for gomod in backend/*/go.mod; do
    modules+=("$(dirname "$gomod")")
  done
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
major_cache="$work/major-cache"
: >"$major_cache"
rows="$work/rows"
: >"$rows"
errors=()
scanned=0

# next_major <module path> <current version> -> prints the latest version
# of the next major path (e.g. github.com/x/y/v3 v3.1.0), or nothing.
next_major() {
  local path="$1" version="$2" base cur next probe hit
  case "$path" in gopkg.in/*) return 0 ;; esac          # major lives in .vN
  case "$version" in *+incompatible) return 0 ;; esac    # pre-modules major
  if [[ "$path" =~ ^(.*)/v([0-9]+)$ ]]; then
    base="${BASH_REMATCH[1]}"; cur="${BASH_REMATCH[2]}"
  else
    base="$path"; cur=1
  fi
  next=$((cur + 1))
  probe="$base/v$next"
  hit="$(awk -v p="$probe" '$1 == p { $1 = ""; sub(/^ /, ""); print; found=1 } END { exit !found }' "$major_cache")" \
    && { [[ -n "$hit" ]] && echo "$hit"; return 0; }
  hit="$(go list -m -json "$probe@latest" 2>/dev/null | jq -r 'select(.Version) | "\(.Path) \(.Version)"' 2>/dev/null || true)"
  echo "$probe $hit" >>"$major_cache"
  [[ -n "$hit" ]] && echo "$hit"
  return 0
}

for mod in "${modules[@]}"; do
  if [[ ! -f "$mod/go.mod" ]]; then
    errors+=("$mod: no go.mod")
    continue
  fi
  json="$work/list.json"
  if ! (cd "$mod" && go list -m -u -json all) >"$json" 2>"$work/err"; then
    errors+=("$mod: go list failed: $(head -c 300 "$work/err" | tr '\n' ' ')")
    continue
  fi
  scanned=$((scanned + 1))
  name="$(basename "$mod")"

  # One line per direct, non-locally-replaced dep, fields separated by
  # \x1f (not tab: read collapses runs of whitespace IFS, which would
  # shift empty fields):
  # path, version, release date, age in months, update version, minor flag.
  jq -r --argjson now "$NOW" --argjson stale "$STALE_MONTHS" '
    def epoch: sub("\\.[0-9]+"; "") | fromdateiso8601;
    def mm: ltrimstr("v") | split(".") | .[0:2] | join(".");
    select((.Main // false) | not)
    | select((.Indirect // false) | not)
    | select((.Replace != null and (.Replace.Version // "") == "") | not)
    | (if .Time then ((($now - (.Time | epoch)) / 2629746) | floor) else -1 end) as $age
    | (.Update.Version // "") as $upd
    | [ .Path,
        .Version,
        (if .Time then .Time[0:10] else "?" end),
        ($age | tostring),
        $upd,
        (if $upd != "" and ($upd | mm) != (.Version | mm) then "minor" else "" end),
        (if $age >= $stale then "stale" else "" end)
      ] | join("\u001f")
  ' "$json" >"$work/deps.txt" 2>"$work/err" || {
    errors+=("$mod: could not parse go list output: $(head -c 300 "$work/err" | tr '\n' ' ')")
    continue
  }

  while IFS=$'\x1f' read -r path version released age upd minor stale; do
    flags=()
    latest="${upd:-$version}"
    [[ -n "$stale" ]] && flags+=("stale")
    [[ -n "$minor" ]] && flags+=("minor")
    if [[ "$SKIP_MAJOR" != "1" ]]; then
      major="$(next_major "$path" "$version")"
      if [[ -n "$major" ]]; then
        flags+=("major")
        latest="$latest (major: ${major})"
      fi
    fi
    (( ${#flags[@]} == 0 )) && continue
    printf '| %s | `%s` | %s | %s | %s | %s | %s |\n' \
      "$name" "$path" "$version" "$released" "$age" "$latest" "$(IFS=,; echo "${flags[*]}")" >>"$rows"
  done <"$work/deps.txt"
done

flagged=$(wc -l <"$rows" | tr -d ' ')
report="$work/report.md"
{
  echo "## Dependency health (ADR-0042, warn-only)"
  echo
  echo "Scanned ${scanned} module(s); ${flagged} direct dependency finding(s)."
  echo "Flags: **stale** = release in use older than ${STALE_MONTHS} months;"
  echo "**minor** = newer minor available; **major** = newer major module path exists."
  echo
  if (( flagged > 0 )); then
    echo "| Module | Dependency | Current | Released | Age (months) | Latest | Flags |"
    echo "|---|---|---|---|---|---|---|"
    cat "$rows"
  else
    echo "No direct dependency is stale or behind a minor/major release."
  fi
  if (( ${#errors[@]} > 0 )); then
    echo
    echo "### Modules that could not be scanned"
    for e in "${errors[@]}"; do echo "- $e"; done
  fi
} >"$report"

cat "$report"
if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  cat "$report" >>"$GITHUB_STEP_SUMMARY"
fi
exit 0
