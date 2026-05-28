#!/usr/bin/env bash
# Regression test: the CI install-smoke step MUST set
# POWERLAB_ALLOW_DEV_BUILD=1 so the dev-versioned tarball (`0.0.0-ci`)
# is allowed past install.sh's dev-bundle refusal gate.
#
# Background
# ----------
# scripts/package-linux.sh stamps every CI build with version `0.0.0-ci`
# (see the "Compute version" step in .github/workflows/ci.yml — the
# fallback branch when GITHUB_REF is not a tag). The install.sh template
# emitted by package-linux.sh refuses to install anything starting with
# `0.0.0-` unless POWERLAB_ALLOW_DEV_BUILD=1 is set (the dev-bundle
# guard locked by scripts/check-install-refuses-dev-bundle_test.sh,
# which exists to prevent dev artifacts ever installing onto a real
# host by accident).
#
# Without the override, ALL install-smoke matrix entries fail at the
# install step — the gate refuses to even start. This left the
# install-smoke gate chronically red on `main` for 13/20 of the recent
# pushes (verified 2026-05-28) without anyone noticing, because
# install-smoke is `if: github.event_name != 'pull_request'` so PRs
# never run it. The gate appeared green on PR ("skipping") then went
# red post-merge, where nobody was watching.
#
# Net effect: the release-blocking gate that's supposed to catch
# install-time regressions (#509 was the original motivation —
# powerlab-gateway / powerlab-mcp / any other service whose unit file
# breaks at install time) never actually ran. powerlab-mcp.service
# specifically merged via PR #601 with `Install smoke` listed as
# `pass` on the PR (skipped), and the post-merge install-smoke also
# never validated it (refused at install.sh dev-block).
#
# This test locks the env var into the workflow so the gate can never
# silently regress to "skipped at install" again.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORKFLOW="$REPO_ROOT/.github/workflows/ci.yml"

failures=0

if [[ ! -f "$WORKFLOW" ]]; then
  echo "ERROR: $WORKFLOW not found" >&2
  exit 1
fi

# Extract just the install-smoke job body so we can grep within scope
# (rather than matching POWERLAB_ALLOW_DEV_BUILD anywhere in the file
# and getting a false positive from a comment elsewhere). Anchor: the
# job name `install-smoke:` at top-level indent, slurping until the
# next top-level job key.
install_smoke_body=$(awk '
  /^  install-smoke:/ { in_job=1; print; next }
  in_job && /^  [a-zA-Z][a-zA-Z0-9_-]*:/ { in_job=0 }
  in_job { print }
' "$WORKFLOW")

if [[ -z "$install_smoke_body" ]]; then
  echo "ERROR: could not extract install-smoke job from $WORKFLOW" >&2
  echo "  (expected a top-level job key '  install-smoke:')" >&2
  exit 1
fi

assert_in_body() {
  local description="$1"
  local pattern="$2"
  if grep -q -F -- "$pattern" <<<"$install_smoke_body"; then
    echo "  PASS: $description"
  else
    echo "  FAIL: $description (pattern '$pattern' not found in install-smoke job)" >&2
    failures=$((failures + 1))
  fi
}

echo "Test: install-smoke job sets POWERLAB_ALLOW_DEV_BUILD=1"
# The CI tarball is always 0.0.0-ci on non-tag pushes; install.sh
# refuses 0.0.0-* without this override. Without it, the gate cannot
# get past the very first install step.
assert_in_body "POWERLAB_ALLOW_DEV_BUILD=1 is present" \
  "POWERLAB_ALLOW_DEV_BUILD=1"

# Additionally guard the safety contract: the dev-build allowance is
# CI-only. If anyone ever moves it OUT of install-smoke and into a
# tag-only release path, this test will still pass but the intent
# would be wrong — so also assert the install-smoke job actually
# invokes the smoke script (the env var alone is meaningless without
# a real install).
echo "Test: install-smoke job actually invokes check-upgrade-smoke.sh"
assert_in_body "check-upgrade-smoke.sh is invoked" \
  "scripts/check-upgrade-smoke.sh"

echo
if [[ "$failures" == "0" ]]; then
  echo "OK: install-smoke wires the dev-build override and runs the smoke script"
  exit 0
else
  echo "FAIL: $failures failure(s)" >&2
  exit 1
fi
