#!/usr/bin/env bash
# Regression tests for powerlab-mcp packaging (issue #599).
#
# powerlab-mcp (ADR-0034) is the standalone observability + MCP service.
# It's built and tested in CI from day one but was deliberately NOT wired
# into the release tarball while the surface was empty — shipping an
# empty :9090 listener to users would have been worse than not shipping
# at all. With the read-only surface (system:// + journal:// + audit://)
# complete and smoke-validated on real hardware, this test locks in the
# contracts the packaging change must honour:
#
#   1. cross-compile: powerlab-mcp must be in the SERVICES list (or
#      built alongside, like logs-cli), with CGO_ENABLED=0
#   2. version stamp: -X .../powerlab-mcp/main.version=$VERSION must be
#      in the shared LDFLAGS — without it /version reports "private
#      build" on installed boxes (Go's -X is fail-soft; bit-rot is
#      invisible at build time, same class of bug as #159)
#   3. sample conf: /etc/powerlab/mcp.conf.sample must be emitted with
#      ListenAddr / AuditDir / RuntimePath at the defaults the binary
#      already falls back to (a missing/empty conf still boots — this is
#      documentation more than required wiring)
#   4. systemd unit: powerlab-mcp.service must be emitted with:
#        - ExecStart using `-conf` (the flag name in main.go), NOT `-c`
#        - User=root (audit.jsonl is root:root 0600 per ADR-0033;
#          the .142 smoke confirmed root is the only working choice
#          for audit:// reads under the gateway-written file)
#   5. install path: install.sh's SERVICES list (used for stop/enable/
#      restart loops) must include powerlab-mcp; same for uninstall.sh
#
# Usage:
#   ./scripts/check-package-linux-powerlab-mcp_test.sh
#
# Exit 0 = all assertions pass; exit 1 = at least one failed.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TARGET="$REPO_ROOT/scripts/package-linux.sh"

failures=0

assert_grep() {
  local description="$1"
  local pattern="$2"
  if grep -q -F -- "$pattern" "$TARGET"; then
    echo "  PASS: $description"
  else
    echo "  FAIL: $description (pattern '$pattern' not found)" >&2
    failures=$((failures + 1))
  fi
}

assert_grep_extended() {
  local description="$1"
  local pattern="$2"
  if grep -q -E -- "$pattern" "$TARGET"; then
    echo "  PASS: $description"
  else
    echo "  FAIL: $description (pattern '$pattern' not found)" >&2
    failures=$((failures + 1))
  fi
}

echo "Test: powerlab-mcp is cross-compiled by package-linux.sh"
# The build loop either iterates over a SERVICES array that contains it,
# or builds it explicitly the way logs-cli is built. Either is fine — we
# just need a concrete `go build` step that targets backend/powerlab-mcp.
assert_grep "backend/powerlab-mcp path referenced for cross-compile" \
  "backend/powerlab-mcp"
assert_grep "powerlab-mcp binary output named powerlab-mcp" \
  "powerlab-mcp"

echo "Test: powerlab-mcp version is ldflag-stamped (no 'private build' in releases)"
# Go's -X is fail-soft (#159). MCP's main.go declares `version`,
# `commit`, `date`. `main.commit` and `main.date` are already covered
# by the shared stamp.
#
# For `main.version`: the linker's import path for package main is the
# literal string `main` — fully-qualified forms (`github.com/.../
# powerlab-mcp/main.version`) silently no-op (verified with `go tool
# nm`). The unqualified `-X main.version=$VERSION` is the only form
# the linker resolves. Safe for the other services: they don't declare
# `version` in package main, so Go fail-softs the flag on them.
assert_grep "main.version ldflag set (unqualified — package main's only-working form)" \
  "-X main.version=\$VERSION"

echo "Test: mcp.conf.sample is emitted with the default keys"
# The sample is doc-grade — Config.Load() falls back to defaults if any
# key is missing, but a sample shipped in /etc/powerlab is what operators
# read to learn what's tunable.
assert_grep "mcp.conf.sample is generated" \
  "mcp.conf.sample"
assert_grep_extended "mcp.conf.sample contains ListenAddr" \
  "ListenAddr"
assert_grep_extended "mcp.conf.sample contains AuditDir" \
  "AuditDir"
assert_grep_extended "mcp.conf.sample contains RuntimePath" \
  "RuntimePath"

echo "Test: powerlab-mcp.service systemd unit is emitted with the right ExecStart"
# The MCP binary uses `-conf`, not `-c` like the other services. Wiring
# `-c` here would silently start the binary without the conf flag — Go's
# `flag` package would treat `-c` as an unknown flag and Exit(2) the
# process. systemd would loop-restart it forever.
assert_grep "powerlab-mcp.service unit generated" \
  "powerlab-mcp.service"
assert_grep "ExecStart uses -conf (matching main.go's flag name)" \
  "ExecStart=/usr/bin/powerlab-mcp -conf /etc/powerlab/mcp.conf"
# audit.jsonl is root:root 0600 (the gateway is the only writer per
# ADR-0033). The smoke on .142 confirmed running powerlab-mcp under any
# non-root user fails audit:// reads with permission-denied; root is the
# only choice that exposes the full observability surface.
assert_grep "User=root is set on powerlab-mcp.service (audit.jsonl is root:root 0600)" \
  "User=root"

echo "Test: install.sh stops/enables/restarts powerlab-mcp"
# install.sh has multiple SERVICES arrays (stop-before-probe, enable+restart,
# upgrade-rollback retry). They use SHORT names and synthesize
# powerlab-$svc.service in the loop, so the right addition is `mcp`. We
# assert the lengthened list shows up so all three sites stay in sync.
assert_grep "install.sh SERVICES list includes mcp (resolves to powerlab-mcp.service)" \
  "SERVICES=(gateway message-bus user-service core app-management local-storage mcp)"

echo
if [[ "$failures" == "0" ]]; then
  echo "OK: all checks passed"
  exit 0
else
  echo "FAIL: $failures failure(s)" >&2
  exit 1
fi
