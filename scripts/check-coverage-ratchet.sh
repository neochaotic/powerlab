#!/usr/bin/env bash
# check-coverage-ratchet.sh — per-module Go coverage no-regression gate.
#
# Reads the FLOOR for <service> from scripts/coverage-baseline.tsv and
# fails if the module's coverage (from its coverage.out, produced by the
# backend matrix's `go test -coverprofile`) has dropped below it. This is
# a ratchet, not an absolute target: it locks the coverage we already
# have without demanding an unrealistic number, and the floors get
# bumped UP over time as pure-logic coverage climbs.
#
# Usage:  scripts/check-coverage-ratchet.sh <service>
#         (run from repo root; expects backend/<service>/coverage.out)
#
# Exit:   0 OK (≥ floor, or no floor defined → advisory skip)
#         1 regression (below floor)
#         2 usage / missing coverage profile
#
# Test hooks (used by check-coverage-ratchet_test.sh):
#   POWERLAB_COVERAGE_BASELINE_FILE  — override the baseline TSV path
#   POWERLAB_COVERAGE_PCT_OVERRIDE   — supply the measured % directly
#                                      (bypasses `go tool cover`)

set -euo pipefail

SVC="${1:-}"
if [[ -z "$SVC" ]]; then
  echo "usage: $0 <service>" >&2
  exit 2
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BASELINE="${POWERLAB_COVERAGE_BASELINE_FILE:-$ROOT/scripts/coverage-baseline.tsv}"

# Floor lookup (skip comment/blank lines).
floor="$(awk -F'\t' -v s="$SVC" '!/^#/ && $1==s {print $2}' "$BASELINE" 2>/dev/null | head -1)"
if [[ -z "$floor" ]]; then
  echo "[coverage-ratchet] $SVC: no floor defined — advisory only, skipping."
  exit 0
fi

# Measured coverage: override for tests, else parse the coverprofile.
if [[ -n "${POWERLAB_COVERAGE_PCT_OVERRIDE:-}" ]]; then
  pct="$POWERLAB_COVERAGE_PCT_OVERRIDE"
else
  mod="$ROOT/backend/$SVC"
  [[ -f "$mod/coverage.out" ]] || { echo "[coverage-ratchet] no coverage profile at $mod/coverage.out — run tests with -coverprofile first" >&2; exit 2; }
  # `go tool cover -func` resolves the profile's packages against the
  # module, so it MUST run from inside the module dir (running from the
  # go.work repo root fails with "no required module provides package").
  pct="$(cd "$mod" && go tool cover -func=coverage.out | awk '/^total:/{gsub("%","",$NF); print $NF}')"
  [[ -n "$pct" ]] || { echo "[coverage-ratchet] could not parse total from $mod/coverage.out" >&2; exit 2; }
fi

# Float compare: fail iff pct < floor.
if awk -v p="$pct" -v f="$floor" 'BEGIN{ exit !(p+0 < f+0) }'; then
  echo "[coverage-ratchet] REGRESSION: $SVC coverage ${pct}% is below floor ${floor}%." >&2
  echo "  Add tests to restore it, or — if the drop is intentional (e.g. a tested" >&2
  echo "  file was removed) — lower the floor in scripts/coverage-baseline.tsv with" >&2
  echo "  a justification in the PR." >&2
  exit 1
fi

echo "[coverage-ratchet] $SVC: ${pct}% ≥ floor ${floor}% — OK"
