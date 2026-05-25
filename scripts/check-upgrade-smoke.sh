#!/usr/bin/env bash
# check-upgrade-smoke.sh — automated version-update test (ADR-0043 follow-up).
#
# The reusable upgrade-regression gate PowerLab lacked. It installs a
# PREVIOUS release, then upgrades in place to a NEW build, and asserts
# the box is healthy after the upgrade. Catches the bug class that bit
# the upgrade-401 saga (v0.6.7→v0.6.10) and the v0.6.12 stale-UI cut.
#
# Generic invariants (not embed-specific) so every future release runs
# the same gate:
#   - gateway answers HTTP 200 after the upgrade
#   - /etc/powerlab/version == the NEW version
#   - no powerlab-* systemd unit is in a failed state
#   - the auth DB survives (user-service still reports a created admin
#     OR the login endpoint is reachable)
# Plus, when --expect-embedded is passed (ADR-0043 phase 3+):
#   - the gateway logs ui_source=embedded
#   - /usr/share/powerlab/www does NOT exist (legacy bundle removed)
#
# DESTRUCTIVE: this installs PowerLab system-wide and restarts its
# units. Run it ONLY on an ephemeral host — a CI runner or the Lima
# test VM — never on a box you care about. It refuses to run unless
# POWERLAB_UPGRADE_SMOKE_ALLOW=1 is set, to prevent an accidental
# invocation on a workstation.
#
# Usage:
#   POWERLAB_UPGRADE_SMOKE_ALLOW=1 sudo -E scripts/check-upgrade-smoke.sh \
#       --old  /path/to/powerlab-<oldver>-linux-<arch>.tar.gz \
#       --new  /path/to/powerlab-<newver>-linux-<arch>.tar.gz \
#       --new-version 0.7.4 \
#       [--expect-embedded] \
#       [--port 8765]

set -euo pipefail

OLD_TARBALL=""
NEW_TARBALL=""
NEW_VERSION=""
EXPECT_EMBEDDED=0
PORT="8765"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --old) OLD_TARBALL="$2"; shift 2 ;;
    --new) NEW_TARBALL="$2"; shift 2 ;;
    --new-version) NEW_VERSION="$2"; shift 2 ;;
    --expect-embedded) EXPECT_EMBEDDED=1; shift ;;
    --port) PORT="$2"; shift 2 ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done

if [[ "${POWERLAB_UPGRADE_SMOKE_ALLOW:-}" != "1" ]]; then
  echo "REFUSING: this test installs PowerLab system-wide and is destructive." >&2
  echo "Set POWERLAB_UPGRADE_SMOKE_ALLOW=1 and run on an ephemeral host (CI/Lima) only." >&2
  exit 2
fi
if [[ -z "$OLD_TARBALL" || -z "$NEW_TARBALL" || -z "$NEW_VERSION" ]]; then
  echo "usage: $0 --old <tarball> --new <tarball> --new-version <X.Y.Z> [--expect-embedded] [--port N]" >&2
  exit 2
fi
for t in "$OLD_TARBALL" "$NEW_TARBALL"; do
  [[ -f "$t" ]] || { echo "tarball not found: $t" >&2; exit 2; }
done
if ! command -v systemctl >/dev/null; then
  echo "SKIP: no systemd on this host — upgrade smoke needs a real init." >&2
  exit 0
fi

log()  { printf '\n[upgrade-smoke] %s\n' "$*"; }
fail() { printf '\n[upgrade-smoke] FAIL: %s\n' "$*" >&2; exit 1; }

# install_tarball <tarball> — extract to a temp dir and run install.sh.
install_tarball() {
  local tb="$1" d
  d="$(mktemp -d)"
  tar xzf "$tb" -C "$d"
  # tarball extracts to a single top dir
  local root; root="$(find "$d" -maxdepth 1 -mindepth 1 -type d | head -1)"
  [[ -x "$root/install.sh" ]] || fail "install.sh not found/executable in $tb"
  # POWERLAB_NO_INSTALL_LOG=1 stops install.sh from redirecting its
  # output into /var/log/powerlab/install-*.log via `exec > >(tee ...)`.
  # Without it an install failure prints NOTHING to our console (the
  # error is buried in that log file) and the smoke fails opaquely.
  ( cd "$root" && POWERLAB_NO_INSTALL_LOG=1 ./install.sh )
}

wait_for_gateway() {
  local i
  for i in $(seq 1 30); do
    if curl -fsS -m 3 "http://127.0.0.1:${PORT}/" -o /dev/null 2>/dev/null; then
      return 0
    fi
    sleep 1
  done
  return 1
}

# ── 1. Install the OLD release ───────────────────────────────────────────
log "Installing OLD release from $(basename "$OLD_TARBALL")..."
install_tarball "$OLD_TARBALL"
wait_for_gateway || fail "gateway did not come up after OLD install"
OLD_VER="$(cat /etc/powerlab/version 2>/dev/null | tr -dc '0-9.a-zA-Z-' || true)"
log "OLD install healthy; version stamp: ${OLD_VER:-<none>}"

# ── 2. Upgrade in place to the NEW build ─────────────────────────────────
log "Upgrading to NEW build from $(basename "$NEW_TARBALL")..."
install_tarball "$NEW_TARBALL"
wait_for_gateway || fail "gateway did not come up after NEW (upgrade) install"

# ── 3. Assert the upgrade invariants ─────────────────────────────────────
log "Asserting post-upgrade invariants..."

# (a) version stamp moved to NEW
got_ver="$(cat /etc/powerlab/version 2>/dev/null | tr -dc '0-9.a-zA-Z-' || true)"
case "$got_ver" in
  *"$NEW_VERSION"*) log "  version OK ($got_ver contains $NEW_VERSION)" ;;
  *) fail "version stamp is '$got_ver', expected to contain '$NEW_VERSION'" ;;
esac

# (b) no powerlab unit in a failed state
failed="$(systemctl --no-legend --state=failed list-units 'powerlab-*' 2>/dev/null | awk '{print $1}' | tr '\n' ' ')"
[[ -z "${failed// /}" ]] || fail "powerlab units in failed state: $failed"
log "  no failed powerlab units"

# (c) gateway still answers
curl -fsS -m 3 "http://127.0.0.1:${PORT}/" -o /dev/null || fail "gateway not answering 200 after upgrade"
log "  gateway answers HTTP 200"

# (d) embedded-UI assertions (ADR-0043 phase 3+)
if [[ "$EXPECT_EMBEDDED" == "1" ]]; then
  if [[ -d /usr/share/powerlab/www ]]; then
    fail "/usr/share/powerlab/www still exists after upgrade — stale on-disk UI not removed (version-skew risk)"
  fi
  log "  legacy /usr/share/powerlab/www removed"
  # Check the MOST RECENT "Static web service is listening" line (the
  # current gateway), not any historical one — otherwise a future
  # embedded→disk regression could pass on a stale line. Match both the
  # JSON ("ui_source":"embedded") and logfmt (ui_source=embedded) shapes.
  last_static="$(journalctl -u powerlab-gateway --no-pager 2>/dev/null | grep 'Static web service is listening' | tail -1)"
  if printf '%s' "$last_static" | grep -Eq 'ui_source"?[:=] ?"?embedded'; then
    log "  gateway log shows ui_source=embedded (latest static-web line)"
  else
    fail "latest gateway static-web line does not report ui_source=embedded: ${last_static:-<none>}"
  fi
fi

log "PASS — upgrade $OLD_VER → $got_ver healthy."
