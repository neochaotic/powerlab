#!/usr/bin/env bash
# Regression test for the v0.5.8 lock-out caught manually on prod
# (the fix lands in v0.5.9). Simulates the v0.5.4 mishap state — a
# stale `<DataPath>/db/user.db` left behind by a buggy hot-fix
# migration — and verifies that the user-service binary, when started
# with v0.5.9 main.go, now auto-moves the duplicate aside instead of
# refusing to boot.
#
# Per memory rule "bug fix = regression test, no exceptions" + "test
# upgrade pain — make it automated". This is the executable form of
# what we should have run BEFORE shipping v0.5.8.
#
# Usage:
#   ./scripts/test-upgrade-resolves-stale-legacy_test.sh
#
# Requires: a built powerlab-user-service binary (we use the locally
# cross-compiled one if present; otherwise skips with an informative
# message). On CI the package smoke step builds the binaries, so this
# can run as a follow-up step there.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

failures=0

skip_or_fail() {
  local desc="$1"
  echo "  SKIP: $desc" >&2
}

assert_file_missing() {
  local desc="$1"
  local path="$2"
  if [[ ! -e "$path" ]]; then
    echo "  PASS: $desc"
  else
    echo "  FAIL: $desc ($path still exists)" >&2
    failures=$((failures + 1))
  fi
}

assert_file_exists() {
  local desc="$1"
  local path="$2"
  if [[ -f "$path" ]]; then
    echo "  PASS: $desc"
  else
    echo "  FAIL: $desc ($path missing)" >&2
    failures=$((failures + 1))
  fi
}

assert_glob_matches() {
  local desc="$1"
  local pattern="$2"
  # shellcheck disable=SC2086
  if compgen -G "$pattern" > /dev/null; then
    echo "  PASS: $desc"
  else
    echo "  FAIL: $desc (no match for $pattern)" >&2
    failures=$((failures + 1))
  fi
}

# ── Build the user-service binary if not cached ───────────────────────
echo "Building powerlab-user-service for the host arch (this is a one-shot)..."
USER_SERVICE_BIN="$REPO_ROOT/dist/test-bin/powerlab-user-service"
mkdir -p "$REPO_ROOT/dist/test-bin"

if [[ ! -x "$USER_SERVICE_BIN" ]] || [[ "$REPO_ROOT/backend/user-service/main.go" -nt "$USER_SERVICE_BIN" ]]; then
  pushd "$REPO_ROOT/backend/user-service" > /dev/null
  if ! CGO_ENABLED=0 go build -o "$USER_SERVICE_BIN" . 2>/tmp/user-service-build.log; then
    skip_or_fail "could not build user-service for tests; see /tmp/user-service-build.log"
    popd > /dev/null
    echo
    echo "SKIP: build failure (likely missing toolchain). Run \`cd backend/user-service && go build .\` manually."
    exit 0
  fi
  popd > /dev/null
fi

# ── Test: v0.5.4-mishap-state hosts now boot cleanly ───────────────────
echo
echo "Test: v0.5.4 mishap state — both canonical and legacy user.db exist"
SANDBOX=$(mktemp -d)
trap 'rm -rf "$SANDBOX"' EXIT

# Set up the failure scenario:
#   /<sandbox>/user.db          — canonical, what user-service reads
#   /<sandbox>/db/user.db       — legacy, sobra do hot-fix v0.5.4
mkdir -p "$SANDBOX/db" "$SANDBOX/conf" "$SANDBOX/run"
echo "real authoritative data" > "$SANDBOX/user.db"
echo "stale junk from v0.5.4 hot-fix" > "$SANDBOX/db/user.db"

# Minimal config so user-service finds its dirs.
cat > "$SANDBOX/conf/user-service.conf" <<EOF
[common]
RuntimePath = $SANDBOX/run

[user]
DBPath = $SANDBOX
LogPath = $SANDBOX/log
LogSaveName = user-service
LogFileExt = log
EOF

# Run user-service init() phase — it doesn't need to fully come up
# (no message-bus etc.); init() runs before main() so the auto-move
# happens before any service-discovery would fail. Capture the
# stderr output for the "moved stale legacy DB aside" line.
mkdir -p "$SANDBOX/run"
touch "$SANDBOX/run/message-bus.url"  # so the systemd ExecStartPre would pass

# Use timeout: in dev/test we don't need user-service to actually
# come up healthy; we just need init() to run + auto-move to fire.
# init() runs synchronously before main() so a 3s timeout is plenty.
# On macOS, GNU `timeout` ships as `gtimeout` (via coreutils). On
# Linux/CI it's just `timeout`. If neither is found, fall back to a
# manual background+kill pattern.
TIMEOUT_BIN="$(command -v timeout || command -v gtimeout || true)"
set +e
if [[ -n "$TIMEOUT_BIN" ]]; then
  "$TIMEOUT_BIN" 3s "$USER_SERVICE_BIN" -c "$SANDBOX/conf/user-service.conf" -db "$SANDBOX" >"$SANDBOX/stdout" 2>"$SANDBOX/stderr"
  EXIT_CODE=$?
else
  # Manual timeout: launch in background, sleep 3s, kill.
  "$USER_SERVICE_BIN" -c "$SANDBOX/conf/user-service.conf" -db "$SANDBOX" >"$SANDBOX/stdout" 2>"$SANDBOX/stderr" &
  BIN_PID=$!
  sleep 3
  kill -TERM "$BIN_PID" 2>/dev/null || true
  wait "$BIN_PID" 2>/dev/null
  EXIT_CODE=$?
fi
set -e

# Either the binary exited 0/1 normally or timeout killed it (124).
# Both are fine — we only care that init() ran far enough to do the
# move BEFORE main() got far enough to need things we didn't set up
# (jwks, message-bus etc.).

# ── Assertions ─────────────────────────────────────────────────────────
assert_file_exists "canonical user.db preserved" "$SANDBOX/user.db"
assert_file_missing "legacy user.db moved aside (no longer at original path)" "$SANDBOX/db/user.db"
assert_glob_matches "legacy file renamed to .bak.<ts>" "$SANDBOX/db/user.db.bak.*"

# stderr should mention the move so operators have a clear trail.
if grep -q "moved stale legacy DB aside" "$SANDBOX/stderr"; then
  echo "  PASS: stderr mentions the move (operator trail)"
else
  echo "  FAIL: expected 'moved stale legacy DB aside' on stderr; got:" >&2
  head -10 "$SANDBOX/stderr" >&2 || true
  failures=$((failures + 1))
fi

# Canonical content untouched
canon_content=$(cat "$SANDBOX/user.db")
if [[ "$canon_content" == "real authoritative data" ]]; then
  echo "  PASS: canonical content untouched"
else
  echo "  FAIL: canonical content changed: '$canon_content'" >&2
  failures=$((failures + 1))
fi

# Backup content preserved (operator can recover if they need)
bak_file=$(echo "$SANDBOX/db/user.db.bak."*)
if [[ -f "$bak_file" ]]; then
  bak_content=$(cat "$bak_file")
  if [[ "$bak_content" == "stale junk from v0.5.4 hot-fix" ]]; then
    echo "  PASS: backup content preserved (non-destructive move)"
  else
    echo "  FAIL: backup content changed: '$bak_content'" >&2
    failures=$((failures + 1))
  fi
fi

# ── Summary ────────────────────────────────────────────────────────────
echo
if [[ "$failures" == "0" ]]; then
  echo "OK: all checks passed"
  exit 0
else
  echo "FAIL: $failures failure(s)" >&2
  exit 1
fi
