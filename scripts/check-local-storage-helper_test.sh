#!/usr/bin/env bash
# Smoke tests for scripts/shell/local-storage-helper.sh (#464).
#
# Validates the helper:
#   1. Parses cleanly (bash -n)
#   2. All 4 expected functions are defined (USB_Start_Auto,
#      USB_Stop_Auto, do_mount, UDEVILUmount) — these are the names
#      the Go call sites in backend/local-storage/service/{usb,disk}.go
#      hardcode after the `source $ShellPath/local-storage-helper.sh ;`
#      prefix.
#   3. Functions are idempotent against missing args (don't crash
#      with set -u when called incorrectly).
#   4. UDEVILUmount refuses to rmdir anything outside POWERLAB_MOUNT_ROOT
#      (security regression catcher — would-be path-traversal bypass).
#
# Hardware-dependent tests (real udevadm trigger, real mount of a loop
# device) are NOT run here — those live in the Phase E manual hardware
# checklist (#465). This file covers the static contract + safety
# guards that CAN be tested without root + a real USB stick.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HELPER="$REPO_ROOT/scripts/shell/local-storage-helper.sh"

if [[ ! -f "$HELPER" ]]; then
  echo "FAIL: $HELPER missing" >&2
  exit 1
fi

failures=0

assert() {
  local description="$1"
  local condition="$2"
  if eval "$condition"; then
    echo "  PASS: $description"
  else
    echo "  FAIL: $description (condition: $condition)" >&2
    failures=$((failures + 1))
  fi
}

echo "Test: helper parses cleanly (bash -n)"
if bash -n "$HELPER"; then
  echo "  PASS: bash -n clean"
else
  echo "  FAIL: bash -n failed" >&2
  failures=$((failures + 1))
fi

echo "Test: all 4 expected functions are defined"
for fn in USB_Start_Auto USB_Stop_Auto do_mount UDEVILUmount; do
  assert "function $fn defined" "grep -q '^${fn}()' '$HELPER'"
done

echo "Test: functions handle missing args without set -u explosion"
# Source the helper in a subshell with set -u (matches the actual
# `source $ShellPath/local-storage-helper.sh ; FUNC args` Go call site).
output=$(bash -c "
  set -u
  source '$HELPER'
  do_mount 2>&1 || true
  UDEVILUmount 2>&1 || true
" 2>&1)
if echo "$output" | grep -q 'unbound variable'; then
  echo "  FAIL: helper triggered 'unbound variable' under set -u" >&2
  echo "$output" >&2
  failures=$((failures + 1))
else
  echo "  PASS: no unbound-variable errors with missing args"
fi

echo "Test: UDEVILUmount refuses rmdir outside POWERLAB_MOUNT_ROOT (path-traversal safety)"
# Create a temp dir outside any mount root + ask UDEVILUmount to handle
# it. The function should not rmdir it (no mountpoint there, no
# matching POWERLAB_MOUNT_ROOT prefix).
tmp_outside=$(mktemp -d "${TMPDIR:-/tmp}/powerlab-helper-test-XXXXXX")
echo "marker" > "$tmp_outside/marker.txt"
bash -c "
  set -u
  source '$HELPER'
  POWERLAB_MOUNT_ROOT=/mnt/powerlab UDEVILUmount '$tmp_outside'
" >/dev/null 2>&1 || true
assert "outside-prefix dir preserved" "[[ -d '$tmp_outside' ]]"
assert "outside-prefix marker file preserved" "[[ -f '$tmp_outside/marker.txt' ]]"
rm -rf "$tmp_outside"

echo "Test: helper documents the udev rule path it installs"
assert "POWERLAB_UDEV_RULE constant present" \
  "grep -q 'POWERLAB_UDEV_RULE.*99-powerlab-automount.rules' '$HELPER'"

echo "Test: USB_Start_Auto installs rule that covers SD cards (ID_DRIVE_FLASH_SD)"
assert "SD card branch present in udev rule emit" \
  "grep -q 'ID_DRIVE_FLASH_SD' '$HELPER'"

echo "Test: USB_Start_Auto installs rule that covers USB block devices (ID_BUS=usb)"
assert "USB branch present in udev rule emit" \
  "grep -q 'ID_BUS.*usb' '$HELPER'"

echo "Test: package-linux.sh ships the helper under /usr/share/powerlab/shell/"
PKG_SCRIPT="$REPO_ROOT/scripts/package-linux.sh"
assert "package-linux.sh creates /usr/share/powerlab/shell" \
  "grep -q 'install -d.*usr/share/powerlab/shell' '$PKG_SCRIPT'"
assert "package-linux.sh copies shell helpers into target" \
  "grep -q 'shell/.*\\.sh.*usr/share/powerlab/shell' '$PKG_SCRIPT'"
assert "package-linux.sh stages shell/ dir during build" \
  "grep -q 'scripts/shell' '$PKG_SCRIPT'"

echo
if [[ "$failures" == "0" ]]; then
  echo "OK: local-storage-helper.sh contract + safety guards verified"
  exit 0
else
  echo "FAIL: $failures assertion(s) failed" >&2
  exit 1
fi
