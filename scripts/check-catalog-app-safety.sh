#!/usr/bin/env bash
# CI gate (ADR-0039): compose-level security lint for every PowerLab-
# curated catalog app. Rejects composes that grant the container
# capabilities PowerLab is not willing to ship without per-app sign-off.
#
# Rejection classes (each a hard fail; no overrides via flag):
#   - privileged: true                    — full host access
#   - /var/run/docker.sock mount          — container can control Docker
#   - network_mode: host                  — bypass network namespace
#   - pid: host / ipc: host               — bypass process/IPC isolation
#   - cap_add: ALL / SYS_ADMIN            — kernel-equivalent privileges
#   - bind-mount of system paths (/etc, /var/lib, /usr, /root)
#     — escape from the app's data dir
#
# Allowed (LOOKS suspicious but is fine):
#   - cap_drop: ALL                       — defensive, the opposite
#   - bind-mount of /DATA/PowerLabAppData/<app>/...
#                                         — the canonical app data path
#   - network with custom subnet / static IP
#
# Usage:
#   ./scripts/check-catalog-app-safety.sh                     # scan every catalog file
#   ./scripts/check-catalog-app-safety.sh <path/to/file.yml>  # scan a single file
#
# Modes (POWERLAB_CATALOG_SAFETY_STRICT, default 0=warn-only):
#   0 — print findings, exit 0 (Sprint 22 ship phase — legacy Umbrel
#       apps in community-catalog/ still have many findings; the
#       initial curated catalog PR will wipe them and re-seed clean)
#   1 — print findings, exit 1 on any finding (strict mode; flip
#       after the initial curated catalog lands)
#
# Exit codes:
#   0 — pass (no findings, OR warn-only)
#   1 — strict mode + findings detected
#   2 — invalid invocation

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CATALOG_DIR="$REPO_ROOT/community-catalog/Apps"
STRICT="${POWERLAB_CATALOG_SAFETY_STRICT:-0}"

# scan_file inspects one compose file and writes "<file>:<reason>"
# lines for every rejection trigger. Caller decides how to aggregate.
scan_file() {
  local f="$1"
  local relpath="${f#$REPO_ROOT/}"

  # privileged: true
  if grep -qE '^\s*privileged:\s*true' "$f"; then
    echo "$relpath: privileged: true is not allowed (full host access)"
  fi

  # /var/run/docker.sock anywhere (volume mount, short or long form)
  if grep -q '/var/run/docker.sock' "$f"; then
    echo "$relpath: /var/run/docker.sock mount is not allowed (container could control Docker)"
  fi

  # network_mode: host (with optional surrounding quotes)
  if grep -qE "^[[:space:]]*network_mode:[[:space:]]*['\"]?host['\"]?[[:space:]]*\$" "$f"; then
    echo "$relpath: network_mode: host is not allowed (bypasses network namespace)"
  fi

  # pid: host
  if grep -qE "^[[:space:]]*pid:[[:space:]]*['\"]?host['\"]?[[:space:]]*\$" "$f"; then
    echo "$relpath: pid: host is not allowed (bypasses process namespace)"
  fi

  # ipc: host
  if grep -qE "^[[:space:]]*ipc:[[:space:]]*['\"]?host['\"]?[[:space:]]*\$" "$f"; then
    echo "$relpath: ipc: host is not allowed (bypasses IPC namespace)"
  fi

  # cap_add: ALL or SYS_ADMIN. We need to distinguish cap_add from
  # cap_drop: extract just the cap_add block. Approach: pull the
  # next ~8 lines after each cap_add: header and grep for the bad
  # tokens.
  if awk '
    /^[[:space:]]*cap_add:/ {in_add=1; n=0; next}
    in_add && /^[[:space:]]*cap_drop:/ {in_add=0; next}
    in_add && /^[[:space:]]*[a-zA-Z_]+:/ && !/^[[:space:]]*-/ {in_add=0}
    in_add { n++; print }
    n > 12 {in_add=0}
  ' "$f" | grep -qE '^\s*-\s*"?(ALL|SYS_ADMIN)"?'; then
    if grep -q "ALL" "$f"; then
      echo "$relpath: cap_add: ALL is not allowed (kernel-equivalent privileges)"
    fi
    if grep -q "SYS_ADMIN" "$f"; then
      echo "$relpath: cap_add: SYS_ADMIN is not allowed (kernel-equivalent privileges)"
    fi
  fi

  # System-path bind mounts. Match `volumes:` entries that map FROM
  # a forbidden system path. Allowed root: /DATA/PowerLabAppData/*,
  # /tmp/powerlab-data/* (macOS dev), and named volumes (no leading /).
  #
  # Forbidden roots: /etc, /var/lib, /usr, /root, /home, /boot, /sys,
  # /proc (these are escape points or contain host secrets).
  local forbidden_roots='^[[:space:]]*-[[:space:]]*("?)(/etc|/var/lib|/usr|/root|/home|/boot|/sys|/proc)(/[^:]*)?:'
  if grep -E "$forbidden_roots" "$f" | grep -vE '^[[:space:]]*-[[:space:]]*"?(/DATA/PowerLabAppData|/tmp/powerlab-data)'; then
    grep -nE "$forbidden_roots" "$f" | grep -vE '^[0-9]+:[[:space:]]*-[[:space:]]*"?(/DATA/PowerLabAppData|/tmp/powerlab-data)' | while IFS= read -r line; do
      local lineno src
      lineno="${line%%:*}"
      src="$(echo "$line" | grep -oE '/(etc|var/lib|usr|root|home|boot|sys|proc)[^:]*' | head -1)"
      echo "$relpath:$lineno: bind-mount of system path '$src' is not allowed (escape from app data dir)"
    done
  fi
}

main() {
  local files=()
  if [[ $# -gt 0 ]]; then
    files=("$@")
  else
    if [[ ! -d "$CATALOG_DIR" ]]; then
      echo "INFO: no $CATALOG_DIR — nothing to scan" >&2
      exit 0
    fi
    while IFS= read -r f; do
      files+=("$f")
    done < <(find "$CATALOG_DIR" -name "docker-compose.yml")
  fi

  if [[ "${#files[@]}" -eq 0 ]]; then
    echo "INFO: no docker-compose.yml files found to scan" >&2
    exit 0
  fi

  local total_findings=0
  local files_with_findings=0
  for f in "${files[@]}"; do
    local out
    out="$(scan_file "$f")"
    if [[ -n "$out" ]]; then
      echo "$out"
      files_with_findings=$(( files_with_findings + 1 ))
      total_findings=$(( total_findings + $(echo "$out" | wc -l) ))
    fi
  done

  if [[ "$total_findings" -eq 0 ]]; then
    echo "OK: 0 safety findings across ${#files[@]} catalog file(s)" >&2
    exit 0
  fi

  echo "" >&2
  echo "Found $total_findings safety finding(s) across $files_with_findings catalog file(s)" >&2
  echo "Each finding requires per-app sign-off (ADR-0039) or removal." >&2

  if [[ "$STRICT" -eq 1 ]]; then
    echo "FAIL: strict mode" >&2
    exit 1
  fi
  echo "(warn-only; set POWERLAB_CATALOG_SAFETY_STRICT=1 to fail)" >&2
  exit 0
}

main "$@"
