#!/usr/bin/env bash
# Regression tests for scripts/package-linux.sh wiring of Sprint 14
# Phase 1 (#150 / ADR-0027):
#   1. Cross-compile backend/logs-cli/ → $STAGE/bin/powerlab-logs
#   2. install.sh tees stdout to /var/log/powerlab/install-<ts>.log
#      with 10-file retention rotation
#   3. install.sh patches /etc/docker/daemon.json with json-file
#      rotation (max-size=10m, max-file=3), idempotent + jq-guarded

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

echo "Test: logs-cli cross-compile step present"
assert_grep "go build for logs-cli"     "cd \"\$ROOT/backend/logs-cli\""
assert_grep "logs-cli binary output"    "-o \"\$STAGE/bin/powerlab-logs\""

echo "Test: install.sh tees stdout to /var/log/powerlab/install-<ts>.log"
assert_grep "tee install log"           "tee \"\$__POWERLAB_INSTALL_LOG\""
assert_grep "rotate 10 files"           "tail -n +11 | xargs -r rm -f"
assert_grep "env escape hatch"          "POWERLAB_NO_INSTALL_LOG"

echo "Test: Docker daemon.json rotation patch wired"
assert_grep "daemon.json path"          "DAEMON_JSON=/etc/docker/daemon.json"
assert_grep "max-size 10m"              "max-size"
assert_grep "max-file 3"                "max-file"
assert_grep "SIGHUP reload"             "systemctl reload docker"

echo "Test: backend/logs-cli package compiles"
if (cd "$REPO_ROOT/backend/logs-cli" && go build -o /dev/null . 2>/dev/null); then
  echo "  PASS: backend/logs-cli compiles"
else
  echo "  FAIL: backend/logs-cli failed to compile" >&2
  failures=$((failures + 1))
fi

echo
if [[ "$failures" == "0" ]]; then
  echo "OK: all checks passed"
  exit 0
else
  echo "FAIL: $failures failure(s)" >&2
  exit 1
fi
