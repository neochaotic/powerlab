#!/usr/bin/env bash
# CI gate: detect Go dead code (unreachable functions, methods, types)
# per service. Runs `golang.org/x/tools/cmd/deadcode` against each
# service's main package and compares the count to a per-service
# baseline checked into git.
#
# Why this exists:
#   Sprint 1 (2026-05-08) ran deadcode once and flagged 5 packages as
#   "verify before deletion" — 3 turned out to be alive, 2 were
#   genuinely dead but the verify step was never carried out. They sat
#   untouched through 15 sprints. Sprint 19 PR 5 added a warn-only CI
#   gate; Sprint 21 PR 7 extracted ADR-0037 which replaces the zero-
#   strict goal with delta-strict: the baseline is a ceiling, not a
#   target. New dead code → CI fails; reductions → developer updates
#   the baseline file in the same PR.
#
# Usage:
#   ./scripts/check-deadcode.sh <service>            # per service
#   ./scripts/check-deadcode.sh                      # all services
#
# Modes (POWERLAB_DEADCODE_MODE, default: delta):
#   delta   → compare count to scripts/deadcode-baseline/<svc>.txt
#             ↑ exceeds baseline  → FAIL
#             = matches baseline  → OK
#             ↓ below baseline    → OK + instruction to update baseline
#   warn    → print findings, always exit 0 (legacy / debugging)
#   strict  → fail on ANY finding (the original "zero deadcode" mode;
#             not used by CI per ADR-0037)
#
# Legacy alias: POWERLAB_DEADCODE_STRICT=0 → MODE=warn, =1 → MODE=strict.
# Honored only when POWERLAB_DEADCODE_MODE is unset.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SERVICES_DEFAULT=(app-management gateway core user-service message-bus local-storage)
BASELINE_DIR="$REPO_ROOT/scripts/deadcode-baseline"

# Mode resolution: explicit MODE wins; otherwise fall back to legacy
# STRICT={0,1} mapping for backwards compatibility.
if [[ -n "${POWERLAB_DEADCODE_MODE:-}" ]]; then
  MODE="$POWERLAB_DEADCODE_MODE"
elif [[ -n "${POWERLAB_DEADCODE_STRICT:-}" ]]; then
  if [[ "$POWERLAB_DEADCODE_STRICT" -eq 1 ]]; then
    MODE="strict"
  else
    MODE="warn"
  fi
else
  MODE="delta"
fi
DEADCODE_VERSION="${POWERLAB_DEADCODE_VERSION:-latest}"

case "$MODE" in
  delta|warn|strict) ;;
  *)
    echo "ERROR: POWERLAB_DEADCODE_MODE must be one of: delta, warn, strict (got: $MODE)" >&2
    exit 2
    ;;
esac

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

# read_baseline returns the integer ceiling for `svc`, or "" when no
# baseline file exists. The caller decides what to do with "":
#   delta mode treats missing baseline as 0 (any finding is a new
#   regression — fail loudly, baseline must be created in same PR).
read_baseline() {
  local svc="$1"
  local f="$BASELINE_DIR/$svc.txt"
  if [[ ! -f "$f" ]]; then
    echo ""
    return 0
  fi
  # Strip any whitespace; tolerate trailing newline. A baseline file
  # is a single integer line. Anything else is a developer bug —
  # error rather than silently re-baseline.
  local content
  content="$(tr -d '[:space:]' <"$f")"
  if ! [[ "$content" =~ ^[0-9]+$ ]]; then
    echo "ERROR: baseline file $f has non-integer content: '$content'" >&2
    return 1
  fi
  echo "$content"
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

  popd >/dev/null

  case "$MODE" in
    strict)
      if [[ "$count" -eq 0 ]]; then
        echo "  OK: 0 dead symbols"
      else
        echo "$out" | head -30
        echo ""
        echo "  Findings: $count dead symbol(s) in $svc"
        echo "  FAIL: strict mode — POWERLAB_DEADCODE_MODE=strict requires zero" >&2
        return 1
      fi
      ;;
    warn)
      if [[ "$count" -eq 0 ]]; then
        echo "  OK: 0 dead symbols"
      else
        echo "$out" | head -30
        echo ""
        echo "  Findings: $count dead symbol(s) in $svc"
        echo "  (warn-only mode; no failure emitted)"
      fi
      ;;
    delta)
      local baseline
      if ! baseline="$(read_baseline "$svc")"; then
        return 1
      fi
      if [[ -z "$baseline" ]]; then
        # No baseline file exists. Treat as 0 — any finding fails so
        # the operator is forced to add the baseline file in the same
        # commit (and review the contents).
        if [[ "$count" -eq 0 ]]; then
          echo "  OK: 0 dead symbols (no baseline file yet; consider creating $BASELINE_DIR/$svc.txt with '0')"
        else
          echo "$out" | head -30
          echo ""
          echo "  Findings: $count dead symbol(s) in $svc"
          echo "  FAIL: no baseline file at $BASELINE_DIR/$svc.txt — create it with '$count' (and review the symbols) before this PR can land." >&2
          return 1
        fi
      elif [[ "$count" -gt "$baseline" ]]; then
        echo "$out" | head -30
        echo ""
        echo "  Findings: $count dead symbol(s) in $svc (baseline: $baseline)"
        echo "  FAIL: count exceeds baseline by $((count - baseline)). Either:" >&2
        echo "        - Wire up the newly-dead function so it's reachable, OR" >&2
        echo "        - Delete it, OR" >&2
        echo "        - (Rarely) bump the baseline in $BASELINE_DIR/$svc.txt with reviewer sign-off" >&2
        return 1
      elif [[ "$count" -lt "$baseline" ]]; then
        echo "  OK: $count dead symbols — DOWN from baseline $baseline"
        echo "  Reduce baseline: \`echo $count > $BASELINE_DIR/$svc.txt\` (commit the change in this PR)"
      else
        echo "  OK: $count dead symbols (matches baseline)"
      fi
      ;;
  esac

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
    case "$MODE" in
      strict) echo "FAIL: dead code detected in strict mode" >&2 ;;
      delta)  echo "FAIL: dead code count exceeded baseline (delta mode)" >&2 ;;
    esac
    exit 1
  fi
}

main "$@"
