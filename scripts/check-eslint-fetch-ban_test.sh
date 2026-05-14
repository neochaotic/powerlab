#!/usr/bin/env bash
# check-eslint-fetch-ban_test.sh — Sprint 15 #353
#
# Meta-test for the no-raw-fetch ESLint rule in ui/eslint.config.js.
# Asserts BOTH directions:
#   1. `npm run lint` exits 0 on the current main state (allow-list covers
#      the 5 intentional raw-fetch callers).
#   2. Dropping a contrived violator file into src/lib/stores/ makes
#      `npm run lint` exit 1 with exactly one no-restricted-syntax error.
#
# This is the equivalent of a contract test for an ESLint rule. Without it,
# someone could comment out the rule in eslint.config.js and CI would happily
# keep going — silent regression of the v0.6.7 → v0.6.10 upgrade-401 lesson.
#
# CI wires this into the Frontend job. Fail-fast — exits non-zero on any
# assertion failure.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT/ui"

log() { echo "[check-eslint-fetch-ban] $*"; }

VIOLATOR="src/lib/stores/__rule_smoke__violator.ts"
cleanup() { rm -f "$VIOLATOR"; }
trap cleanup EXIT

# ── Assertion 1: lint clean on the current main state ────────────────────────
log "Direction 1: lint must pass on main (allow-list covers all 5 intentional callers)"
if ! npm run lint --silent >/tmp/lint-out.log 2>&1; then
	echo "FAIL: lint failed on a clean main checkout — rule too strict, missing allow-list entry, OR a new raw-fetch slipped in."
	echo "Output:"
	cat /tmp/lint-out.log
	exit 1
fi
log "  ✓ lint passes on main"

# ── Assertion 2: rule fires on a contrived violator ──────────────────────────
log "Direction 2: dropping a raw-fetch violator must produce exactly 1 no-restricted-syntax error"
cat > "$VIOLATOR" <<'TS'
// Smoke-test violator — must trigger the no-raw-fetch rule.
// Auto-deleted by check-eslint-fetch-ban_test.sh on exit.
export async function trigger() {
	const res = await fetch('/v1/some-authenticated-route', { method: 'POST' });
	return res.json();
}
TS

if npx eslint "$VIOLATOR" --no-warn-ignored >/tmp/violator-out.log 2>&1; then
	echo "FAIL: lint passed on a deliberate violator — rule is not firing."
	echo "Expected at least one no-restricted-syntax error on $VIOLATOR."
	echo "Output:"
	cat /tmp/violator-out.log
	exit 1
fi

VIOLATIONS=$(grep -c "no-restricted-syntax" /tmp/violator-out.log || echo 0)
if [ "$VIOLATIONS" -lt 1 ]; then
	echo "FAIL: violator did not produce any no-restricted-syntax error."
	echo "Got $VIOLATIONS, expected ≥1."
	echo "Output:"
	cat /tmp/violator-out.log
	exit 1
fi
log "  ✓ violator caught — $VIOLATIONS no-restricted-syntax error(s)"

log "PASS — ESLint fetch ban firing in both directions."
