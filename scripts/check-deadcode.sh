#!/usr/bin/env bash
# CI gate: detect Go dead code (unreachable functions, methods, types)
# per service. Runs `golang.org/x/tools/cmd/deadcode` against each
# service's main package and fails if any dead symbol is found.
#
# Why this exists:
#   Sprint 1 (2026-05-08) ran deadcode once and flagged 5 packages as
#   "verify before deletion" — 3 turned out to be alive, 2 were
#   genuinely dead but the verify step was never carried out. They sat
#   untouched through 15 sprints. The 🟡 process had no CI counterpart,
#   so the next miss had no automatic catch. Sprint 19 PR 5 closes that.
#
# Usage:
#   ./scripts/check-deadcode.sh <service>            # per service
#   ./scripts/check-deadcode.sh                      # all services
#
# Modes:
#   POWERLAB_DEADCODE_STRICT=0  → warn-only (default) — print findings,
#                                  exit 0. The phase-in window before
#                                  v0.7.0 hard-fail.
#   POWERLAB_DEADCODE_STRICT=1  → hard-fail on any finding (exit 1).
#
# The warn-soft default lets developers see findings during the
# transition without blocking unrelated PRs. After Sprint 19 lands and
# the existing dead code is gone, flip the default to strict for v0.7.0.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SERVICES_DEFAULT=(app-management gateway core user-service message-bus local-storage sync-catalog)

STRICT="${POWERLAB_DEADCODE_STRICT:-0}"
DEADCODE_VERSION="${POWERLAB_DEADCODE_VERSION:-latest}"

# local-storage compiles only with Linux build tags (netlink, xattr,
# fuse). Skip it on non-Linux hosts. CI Linux runner is where the
# authoritative check happens for that service.
should_skip() {
  local svc="$1"
  if [[ "$svc" == "local-storage" && "$(uname -s)" != "Linux" ]]; then
    return 0
  fi
  return 1
}

run_service() {
  local svc="$1"
  local dir="$REPO_ROOT/backend/$svc"

  if [[ ! -d "$dir" ]]; then
    echo "  skip: $svc (no backend/$svc)" >&2
    return 0
  fi
  if should_skip "$svc"; then
    echo "  skip: $svc (not Linux — netlink/xattr/fuse syscalls)"
    return 0
  fi

  echo "── $svc ──"
  pushd "$dir" >/dev/null

  # `deadcode` finds functions reachable from `main`. Per-service runs
  # produce a list of unreachable symbols. The tool exits 0 even when
  # findings exist — we count and decide.
  local out
  out="$(go run "golang.org/x/tools/cmd/deadcode@$DEADCODE_VERSION" ./... 2>&1 || true)"
  local count
  count="$(printf '%s\n' "$out" | grep -cE '^[A-Za-z_/.-]+:[0-9]+:' || true)"

  if [[ "$count" -eq 0 ]]; then
    echo "  OK: 0 dead symbols"
  else
    echo "$out" | head -30
    echo ""
    echo "  Findings: $count dead symbol(s) in $svc"
    if [[ "$STRICT" -eq 1 ]]; then
      popd >/dev/null
      return 1
    fi
    echo "  (warn-only; set POWERLAB_DEADCODE_STRICT=1 to fail)"
  fi

  popd >/dev/null
  return 0
}

main() {
  local services=()
  if [[ $# -gt 0 ]]; then
    services=("$@")
  else
    services=("${SERVICES_DEFAULT[@]}")
  fi

  local any_fail=0
  for svc in "${services[@]}"; do
    if ! run_service "$svc"; then
      any_fail=1
    fi
  done

  if [[ "$any_fail" -ne 0 ]]; then
    echo ""
    echo "FAIL: dead code detected in strict mode" >&2
    exit 1
  fi
}

main "$@"
