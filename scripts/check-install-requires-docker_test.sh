#!/usr/bin/env bash
# Test gate: the bootstrap install.sh (curl … | sudo bash) refuses to
# continue on a host without Docker, matching the bundled installer in
# scripts/package-linux.sh, instead of printing a dim "Continuing" line
# and installing a panel whose App Store cannot work (#63).
#
# The root check cannot be satisfied in CI (EUID is read-only), so the
# test runs a copy of install.sh with that one line neutralised, under a
# PATH that only holds shims for the tools the pre-flight needs.

set -euo pipefail
cd "$(dirname "$0")/.."

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

grep -q '\[\[ "\$EUID" -eq 0 \]\]' install.sh \
  || { echo "FAIL: install.sh root check moved; update this test"; exit 1; }
sed 's/^\[\[ "\$EUID" -eq 0 \]\] || die .*$/: # root check stripped by test/' install.sh > "$WORK/install.sh"

BIN="$WORK/bin"
mkdir -p "$BIN"
for tool in uname tar mktemp rm stat; do
  ln -s "$(command -v "$tool")" "$BIN/$tool"
done
# curl shim: always fails, so a run that gets past the Docker gate stops
# at "download failed" without touching the network.
printf '#!/bin/sh\nexit 22\n' > "$BIN/curl"
chmod +x "$BIN/curl"

run_installer() {
  set +e
  out=$(PATH="$BIN" "$BASH" "$WORK/install.sh" 2>&1)
  code=$?
  set -e
}

# Case 1 — no docker on PATH: refuse with the distro hint.
run_installer
if [[ "$code" == "0" ]]; then
  echo "FAIL: install.sh exited 0 without Docker"; exit 1
fi
grep -q "Docker is not installed" <<<"$out" \
  || { echo "FAIL: missing 'Docker is not installed' message: $out"; exit 1; }
grep -q "Detected distro family:" <<<"$out" \
  || { echo "FAIL: missing distro family hint: $out"; exit 1; }
grep -q "docs.docker.com/engine/install" <<<"$out" \
  || { echo "FAIL: missing Docker install docs link: $out"; exit 1; }
if grep -q "Continuing" <<<"$out"; then
  echo "FAIL: install.sh still says it is continuing without Docker: $out"; exit 1
fi
if grep -q "Downloading PowerLab" <<<"$out"; then
  echo "FAIL: install.sh started the download without Docker: $out"; exit 1
fi

# Case 2 — docker present: the gate passes and the run reaches the download.
printf '#!/bin/sh\nexit 0\n' > "$BIN/docker"
chmod +x "$BIN/docker"
run_installer
if grep -q "Docker is not installed" <<<"$out"; then
  echo "FAIL: Docker gate fired with docker on PATH: $out"; exit 1
fi
grep -q "download failed" <<<"$out" \
  || { echo "FAIL: expected the run to reach the (stubbed) download: $out"; exit 1; }

echo "PASS: bootstrap install.sh refuses without Docker and proceeds with it"
