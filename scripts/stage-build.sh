#!/usr/bin/env bash
# Build PowerLab backend binaries for a Linux/amd64 hot-swap deploy
# from a non-Linux dev box (#414).
#
# CGO services (user-service uses libpam, local-storage uses fuse +
# netlink syscalls) cannot be cross-compiled from macOS because the
# macOS toolchain doesn't ship the required Linux headers. Adding an
# x86_64-linux-gnu toolchain to every dev box is heavy — instead, this
# script:
#
#   1. Cross-compiles pure-Go services from the local source
#      (gateway, app-management, core, message-bus, sync-catalog —
#      `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build`).
#   2. Downloads the latest GitHub release tarball and extracts the
#      CGO services (user-service, local-storage) from it. These were
#      built by CI on Linux, so they work natively on the deploy host.
#   3. Writes everything to ./stage-build/bin/ in a layout matching
#      /usr/bin/ on the target box.
#
# Deploy is intentionally manual: this script ONLY builds. To hot-swap
# the binaries onto a target host, run:
#
#   scp stage-build/bin/* root@<host>:/usr/bin/
#   ssh root@<host> 'systemctl restart powerlab-*'
#
# Background: memory `feedback_staging_build_linux_only` + #414.
# Symptom this prevents: user-service built on macOS with CGO_ENABLED=0
# silently returns "invalid credentials" on /v1/users/login because the
# PAM call site no-ops. Caught in Sprint 20 staging deploy (2026-05-15).

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STAGE_DIR="$REPO_ROOT/stage-build"
STAGE_BIN="$STAGE_DIR/bin"

# Override these via env if you want a non-default release source.
GH_OWNER="${POWERLAB_GH_OWNER:-neochaotic}"
GH_REPO="${POWERLAB_GH_REPO:-powerlab}"
RELEASE_TAG="${POWERLAB_RELEASE_TAG:-latest}"  # "latest" or "vX.Y.Z"

log() { echo "[stage-build] $*"; }

# ─── Local cross-compile of pure-Go services ─────────────────────────
PURE_GO_SERVICES=(
  "backend/gateway:powerlab-gateway"
  "backend/app-management:powerlab-app-management"
  "backend/core:powerlab-core"
  "backend/message-bus:powerlab-message-bus"
  "backend/sync-catalog:powerlab-sync-catalog"
)

build_pure_go() {
  log "Cross-compiling pure-Go services (CGO_ENABLED=0, linux/amd64)..."
  mkdir -p "$STAGE_BIN"
  for entry in "${PURE_GO_SERVICES[@]}"; do
    local src="${entry%%:*}"
    local out="${entry##*:}"
    log "  building $out from $src"
    (
      cd "$REPO_ROOT/$src"
      CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
        go build -trimpath -o "$STAGE_BIN/$out" .
    )
  done
}

# ─── Pull CGO services from latest release tarball ───────────────────
CGO_SERVICES=(
  "powerlab-user-service"
  "powerlab-local-storage"
)

fetch_cgo_from_release() {
  log "Fetching CGO services from GitHub release ($GH_OWNER/$GH_REPO, tag=$RELEASE_TAG)..."

  local api_url
  if [[ "$RELEASE_TAG" == "latest" ]]; then
    api_url="https://api.github.com/repos/$GH_OWNER/$GH_REPO/releases/latest"
  else
    api_url="https://api.github.com/repos/$GH_OWNER/$GH_REPO/releases/tags/$RELEASE_TAG"
  fi

  local tarball_url
  tarball_url=$(curl -fsSL "$api_url" \
    | grep '"browser_download_url"' \
    | grep 'linux-amd64.tar.gz"' \
    | grep -v 'manifest' \
    | head -1 \
    | sed -E 's/.*"browser_download_url": *"([^"]+)".*/\1/')

  if [[ -z "$tarball_url" ]]; then
    log "ERROR: could not resolve linux-amd64.tar.gz asset URL from $api_url" >&2
    exit 1
  fi
  log "  tarball: $tarball_url"

  local tmp
  tmp=$(mktemp -d "${TMPDIR:-/tmp}/powerlab-cgo-fetch-XXXXXX")
  trap 'rm -rf "$tmp"' EXIT

  log "  downloading + extracting..."
  curl -fsSL "$tarball_url" -o "$tmp/release.tar.gz"
  tar -xzf "$tmp/release.tar.gz" -C "$tmp"

  local extracted_bin
  extracted_bin=$(find "$tmp" -type d -name bin | head -1)
  if [[ -z "$extracted_bin" ]]; then
    log "ERROR: tarball layout unexpected — no bin/ dir found inside $tmp" >&2
    exit 1
  fi

  for svc in "${CGO_SERVICES[@]}"; do
    if [[ ! -f "$extracted_bin/$svc" ]]; then
      log "ERROR: $svc not in release tarball (looked in $extracted_bin)" >&2
      exit 1
    fi
    cp "$extracted_bin/$svc" "$STAGE_BIN/$svc"
    log "  staged $svc (from release)"
  done
}

# ─── Main ────────────────────────────────────────────────────────────
log "Cleaning $STAGE_BIN..."
rm -rf "$STAGE_DIR"

build_pure_go
fetch_cgo_from_release

log "Done. Staged binaries:"
ls -lh "$STAGE_BIN" 2>&1 | sed 's/^/  /'

cat <<EOF

Next steps (manual hot-swap to a Linux target):

  scp $STAGE_BIN/* root@<HOST>:/usr/bin/
  ssh root@<HOST> 'systemctl restart powerlab-*'

Or for a single service:

  scp $STAGE_BIN/powerlab-app-management root@<HOST>:/usr/bin/
  ssh root@<HOST> 'systemctl restart powerlab-app-management'
EOF
