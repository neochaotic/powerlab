#!/usr/bin/env bash
# PowerLab — Linux Production Packaging
# Cross-compiles all Go services + builds frontend, then bundles a deployable
# tarball with binaries, static UI, sample configs, systemd units, and an
# install script. Tested on macOS host targeting linux/amd64 and linux/arm64.
#
# Usage:
#   ./scripts/package-linux.sh                # builds amd64
#   ./scripts/package-linux.sh arm64          # builds arm64
#   ./scripts/package-linux.sh amd64 v0.1.0   # builds amd64 with version label
set -euo pipefail

ARCH="${1:-amd64}"
VERSION="${2:-0.1.0-dev}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/dist"
STAGE="$OUT/powerlab-$VERSION-linux-$ARCH"
TARBALL="$OUT/powerlab-$VERSION-linux-$ARCH.tar.gz"

if [[ "$ARCH" != "amd64" && "$ARCH" != "arm64" ]]; then
  echo "ERROR: unsupported arch '$ARCH'. Use amd64 or arm64." >&2
  exit 1
fi

log() { echo "[powerlab-pkg] $*"; }

# ─── 1. Clean & prepare ──────────────────────────────────────────────────
log "Packaging PowerLab v$VERSION for linux/$ARCH"
rm -rf "$STAGE" "$TARBALL"
mkdir -p "$STAGE/bin" "$STAGE/www" "$STAGE/conf" "$STAGE/systemd" "$STAGE/store"

# ─── 2. Cross-compile Go services ────────────────────────────────────────
log "Cross-compiling backend services for linux/$ARCH..."
SERVICES=(gateway app-management core user-service message-bus local-storage)

# user-service links against libpam (auth_os_pam_linux.go) when CGO is
# on. Without CGO the build picks up auth_os_pam_stub.go which routes
# users to the bcrypt SetupWizard instead — a working but less-elegant
# auth experience.
#
# CGO is enabled on amd64 because GitHub-hosted Ubuntu runners ship
# libpam0g-dev natively. arm64 cross-compile would need libpam0g-dev:arm64
# + a multi-arch apt setup that varies per Ubuntu version (different
# /etc/apt sources format on 22.04 vs 24.04+); rather than ship a
# fragile multi-arch apt dance, arm64 builds stay CGO_ENABLED=0 and
# fall back to the SetupWizard. arm64 native PAM is tracked as a
# follow-up release work item.
#
# local-storage uses bazil.org/fuse via syscall (no C linkage needed),
# so it builds with CGO off on every arch.
needs_cgo() {
  if [[ "$1" == "user-service" ]] && [[ "$ARCH" == "amd64" ]]; then
    return 0
  fi
  return 1
}

CC_FOR_CGO="${CC_FOR_CGO:-gcc}"

# Build-time version stamps injected via -ldflags. Issue #159: the old
# ldflag string was double-broken — `main.version` was the wrong target
# variable name (each main.go declares `commit` and `date`, not
# `version`) AND the `github.com/IceWhaleTech/CasaOS/common.POWERLAB_VERSION`
# path is dead after PR #151 renamed all modules to
# `github.com/neochaotic/powerlab/backend/*`. Result: every release
# binary shipped with `commit = "private build"` and the in-UI updater
# read `currentVersion = "dev"`, surfacing a permanent (and false)
# "Update available" prompt.
#
# Each ldflag below is documented inline so a future maintainer can
# confirm the target var still exists by grep'ing the codebase. If any
# `-X` target stops compiling into the binary, Go silently drops it —
# no build error, no runtime warning. The regression test at
# `scripts/check-package-linux-ldflags_test.sh` asserts the expected
# strings stay present so this class of bit-rot is caught at CI time.
GIT_COMMIT="${GIT_COMMIT:-$(cd "$ROOT" && git rev-parse --short HEAD 2>/dev/null || echo "unknown")}"
BUILD_DATE="${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

# `main.commit` / `main.date` — declared in every <svc>/main.go.
# `core/common.POWERLAB_VERSION` — read by /v1/powerlab/version handler.
# `core/route/v1.powerLabVersionAtCompileTime` — read by the in-UI
#   updater (currentPowerLabVersion()) to determine "current" in the
#   update-available comparison.
# Setting all four for every service is fine: Go silently ignores
# `-X` for vars that don't exist in a given binary, so the gateway/
# message-bus/etc. just get the `main.commit` and `main.date` ones.
LDFLAGS_VERSION_STAMP="-s -w \
  -X main.commit=$GIT_COMMIT \
  -X main.date=$BUILD_DATE \
  -X github.com/neochaotic/powerlab/backend/core/common.POWERLAB_VERSION=$VERSION \
  -X github.com/neochaotic/powerlab/backend/core/route/v1.powerLabVersionAtCompileTime=$VERSION"

for svc in "${SERVICES[@]}"; do
  log "  building $svc..."
  cd "$ROOT/backend/$svc"
  # codegen/ is .gitignored — regenerate from the OpenAPI specs before
  # compiling so the import paths in main.go resolve. gateway has no
  # generate directives.
  if [[ "$svc" != "gateway" ]]; then
    go generate ./... > /dev/null 2>&1 || true
  fi

  if needs_cgo "$svc"; then
    GOOS=linux GOARCH="$ARCH" CGO_ENABLED=1 CC="$CC_FOR_CGO" go build \
      -trimpath \
      -ldflags="$LDFLAGS_VERSION_STAMP" \
      -o "$STAGE/bin/powerlab-$svc" \
      .
  else
    GOOS=linux GOARCH="$ARCH" CGO_ENABLED=0 go build \
      -trimpath \
      -ldflags="$LDFLAGS_VERSION_STAMP" \
      -o "$STAGE/bin/powerlab-$svc" \
      .
  fi
done

# ─── 3. Build frontend ───────────────────────────────────────────────────
# Honour an existing ui/build/ if the caller already built it (e.g. from
# the Mac host before invoking this script in a Linux container that
# does not have a recent enough Node). Skipping the rebuild also makes
# repeated package runs much faster — but ONLY when we can prove the
# existing bundle was built for THIS version, otherwise we'd ship a
# stale UI (the v0.2.5 first-attempt bug — bundle had 0.2.0 because
# CI cached an old build). Stamp the version into the build dir and
# refuse to reuse a mismatched one.
log "Building frontend (static SPA)..."
cd "$ROOT/ui"
export POWERLAB_VERSION="$VERSION"
BUILD_STAMP_FILE="build/.powerlab-version"
SKIP_OK=0
if [[ "${POWERLAB_SKIP_FRONTEND_BUILD:-}" == "1" ]] && [[ -f "build/index.html" ]] && [[ -f "$BUILD_STAMP_FILE" ]] && [[ "$(cat "$BUILD_STAMP_FILE")" == "$VERSION" ]]; then
  log "  POWERLAB_SKIP_FRONTEND_BUILD=1 — reusing build for v$VERSION"
  SKIP_OK=1
elif [[ "${POWERLAB_SKIP_FRONTEND_BUILD:-}" == "1" ]] && [[ -f "build/index.html" ]]; then
  STAMP="(none)"
  [[ -f "$BUILD_STAMP_FILE" ]] && STAMP="$(cat "$BUILD_STAMP_FILE")"
  log "  POWERLAB_SKIP_FRONTEND_BUILD=1 set but stamp is $STAMP, want $VERSION — rebuilding"
elif [[ -f "build/index.html" ]] && command -v node >/dev/null && [[ "$(node --version | sed 's/^v//' | cut -d. -f1)" -lt 18 ]]; then
  log "  node $(node --version) is too old for SvelteKit — reusing existing ui/build"
  SKIP_OK=1
fi
if (( SKIP_OK == 0 )); then
  npm run build > /dev/null
  echo "$VERSION" > "$BUILD_STAMP_FILE"
fi
cp -R build/* "$STAGE/www/"

# ─── 4. Sample config files ──────────────────────────────────────────────
log "Generating sample configs..."

cat > "$STAGE/conf/gateway.ini.sample" <<EOF
[common]
RuntimePath = /var/run/powerlab

[gateway]
# Default port. The installer will probe at install time and rewrite this
# to a free port (preferring 80, then 8765, then a probe up to 8775).
# 8765 is unassigned by IANA and not used by any common self-hosted tool,
# making it a safe fallback when 80 is already taken.
Port = 8765
EOF

cat > "$STAGE/conf/app-management.conf.sample" <<EOF
[common]
RuntimePath = /var/run/powerlab

[app]
LogPath = /var/log/powerlab
LogSaveName = app-management
LogFileExt = log
AppStorePath = /var/lib/powerlab/appstore
AppsPath = /var/lib/powerlab/apps
StoragePath = /DATA

[server]
appstore = https://cdn.jsdelivr.net/gh/IceWhaleTech/CasaOS-AppStore@gh-pages/store/main.zip
EOF

cat > "$STAGE/conf/core.conf.sample" <<EOF
[common]
RuntimeRootPath = /var/run/powerlab/
LogPath = /var/log/powerlab/

[file]
DBPath = /var/lib/powerlab
ShellPath = /usr/share/powerlab/shell
UserDataPath = /var/lib/powerlab/conf
EOF

cat > "$STAGE/conf/user-service.conf.sample" <<EOF
[common]
RuntimePath = /var/run/powerlab

[user]
DBPath = /var/lib/powerlab
EOF

cat > "$STAGE/conf/message-bus.conf.sample" <<EOF
[common]
RuntimePath = /var/run/powerlab
EOF

cat > "$STAGE/conf/local-storage.conf.sample" <<EOF
[common]
RuntimePath = /var/run/powerlab
EOF

# ─── 5. systemd units ────────────────────────────────────────────────────
log "Generating systemd unit files..."

for svc in "${SERVICES[@]}"; do
  # Each service that takes `-c` reads its config that way. The gateway is
  # special: it loads gateway.ini from constants.DefaultConfigPath itself
  # (no `-c` flag exists on its binary) and only takes `-w` for the www
  # directory. We emit the gateway unit separately below.
  if [[ "$svc" == "gateway" ]]; then
    continue
  fi

  # Service ordering. The actual topology (verified by reading the
  # sources):
  #
  #   gateway      — STARTS FIRST. Writes /var/run/powerlab/management.url.
  #                  No PowerLab dependencies; only network + docker.
  #   message-bus  — Polls management.url for ~10s on startup; dies if
  #                  not found. Writes message-bus.url.
  #   user-service — Reads message-bus.url. Nil-deref-panics on startup
  #                  if absent (dials websocket before nil-check).
  #                  Writes user-service.url.
  #   app-management, local-storage — read message-bus.url.
  #   core         — reads message-bus.url + user-service.url.
  #
  # We use `Wants=` (soft) not `Requires=` (hard) because hard
  # `Requires=` makes one transient failure (e.g. message-bus
  # restarting) cascade to abort everyone. With `Wants=` + `After=` +
  # `ExecStartPre` waiting for the URL file, services can recover
  # individually. Type=simple gives no listen-readiness signal, so
  # ExecStartPre is the actual readiness gate.
  case "$svc" in
    message-bus)
      AFTER_LINE="After=network.target docker.service powerlab-gateway.service
Wants=powerlab-gateway.service"
      WAIT_LINES="ExecStartPre=/bin/bash -c 'for i in {1..30}; do [[ -f /var/run/powerlab/management.url ]] && exit 0; sleep 1; done; exit 1'"
      ;;
    user-service|local-storage|app-management)
      AFTER_LINE="After=network.target docker.service powerlab-gateway.service powerlab-message-bus.service
Wants=powerlab-gateway.service powerlab-message-bus.service"
      WAIT_LINES="ExecStartPre=/bin/bash -c 'for i in {1..30}; do [[ -f /var/run/powerlab/message-bus.url ]] && exit 0; sleep 1; done; exit 1'"
      ;;
    core)
      AFTER_LINE="After=network.target docker.service powerlab-gateway.service powerlab-message-bus.service powerlab-user-service.service
Wants=powerlab-gateway.service powerlab-message-bus.service powerlab-user-service.service"
      WAIT_LINES="ExecStartPre=/bin/bash -c 'for i in {1..30}; do [[ -f /var/run/powerlab/message-bus.url && -f /var/run/powerlab/user-service.url ]] && exit 0; sleep 1; done; exit 1'"
      ;;
    *)
      AFTER_LINE="After=network.target docker.service"
      WAIT_LINES=""
      ;;
  esac

  cat > "$STAGE/systemd/powerlab-$svc.service" <<EOF
[Unit]
Description=PowerLab $svc service
$AFTER_LINE
Wants=docker.service

[Service]
Type=simple
$WAIT_LINES
ExecStart=/usr/bin/powerlab-$svc -c /etc/powerlab/$svc.conf
Restart=always
RestartSec=5
PIDFile=/var/run/powerlab/$svc.pid
Environment=DOCKER_API_VERSION=1.44
Environment=HOME=/root

[Install]
WantedBy=multi-user.target
EOF
done

# Gateway: no -c flag (config is loaded from constants.DefaultConfigPath
# /gateway.ini at startup), only -w for the www directory. Gateway is
# the FIRST PowerLab service to come up — every backend reads
# management.url that gateway writes.
cat > "$STAGE/systemd/powerlab-gateway.service" <<EOF
[Unit]
Description=PowerLab gateway service
After=network.target docker.service
Wants=docker.service

[Service]
Type=simple
ExecStart=/usr/bin/powerlab-gateway -w /usr/share/powerlab/www
Restart=always
RestartSec=5
PIDFile=/var/run/powerlab/gateway.pid
Environment=DOCKER_API_VERSION=1.44
Environment=HOME=/root

[Install]
WantedBy=multi-user.target
EOF

# ─── 6. install.sh ───────────────────────────────────────────────────────
log "Generating install.sh..."
cat > "$STAGE/install.sh" <<'INSTALL_EOF'
#!/usr/bin/env bash
# PowerLab installer — run as root.
#
# Supports two modes:
#   default  — fresh install or in-place install. Idempotent.
#   --upgrade — same as default, plus:
#                · snapshot /etc/powerlab/, /var/lib/powerlab/db,
#                  /usr/bin/powerlab-*, and /etc/systemd/system/powerlab-*.service
#                  to /var/lib/powerlab/backups/pre-upgrade-<ts>/ BEFORE
#                  touching any of them
#                · run a 5-attempt health-check against the gateway after
#                  starting services; on failure, restore from the snapshot
#                  and start services again
#                · write /var/lib/powerlab/last-upgrade.json with
#                  {from, to, succeeded_at | failed_at, snapshot_path}
#                  so the UI can surface the result on next reload
#
# The `--upgrade` flag is set by `core` when it kicks off an in-UI
# update via POST /v1/powerlab-update/install. End users running
# install.sh manually never need to set it.
set -euo pipefail

UPGRADE_MODE=0
ALLOW_COEXIST=0
for arg in "$@"; do
  case "$arg" in
    --upgrade) UPGRADE_MODE=1 ;;
    --allow-coexist) ALLOW_COEXIST=1 ;;
    "") ;;
    *) echo "Unknown argument: $arg (supported: --upgrade, --allow-coexist)" >&2; exit 1 ;;
  esac
done

if [[ $EUID -ne 0 ]]; then
  echo "ERROR: install.sh must be run as root (sudo)." >&2
  exit 1
fi

# ── Distro detection ────────────────────────────────────────────────────
# PowerLab is mostly distro-agnostic (Go binaries, systemd units, FHS
# paths). The only distro-specific surfaces are: error messages that
# suggest install/uninstall commands, and the "remove CasaOS first"
# instruction below. We detect ID from /etc/os-release and map to a
# package-manager hint string.
detect_distro_family() {
  if [[ -r /etc/os-release ]]; then
    # shellcheck source=/dev/null
    . /etc/os-release
    case "${ID:-}${ID_LIKE:-}" in
      *debian*|*ubuntu*) echo "debian" ;;
      *fedora*|*rhel*|*centos*|*rocky*|*almalinux*|*amzn*) echo "rhel" ;;
      *suse*|*opensuse*) echo "suse" ;;
      *arch*|*manjaro*) echo "arch" ;;
      *) echo "unknown" ;;
    esac
  else
    echo "unknown"
  fi
}

DISTRO_FAMILY="$(detect_distro_family)"

docker_install_hint() {
  case "$DISTRO_FAMILY" in
    debian) echo "  apt-get update && apt-get install -y docker.io docker-compose-plugin" ;;
    rhel)   echo "  dnf install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin" ;;
    suse)   echo "  zypper install -y docker docker-compose" ;;
    arch)   echo "  pacman -Sy --noconfirm docker docker-compose" ;;
    *)      echo "  See https://docs.docker.com/engine/install/" ;;
  esac
}

casaos_uninstall_hint() {
  case "$DISTRO_FAMILY" in
    debian) echo "  apt remove --purge 'casaos*'" ;;
    rhel)   echo "  dnf remove 'casaos*'" ;;
    suse)   echo "  zypper remove 'casaos*'" ;;
    arch)   echo "  pacman -Rns \$(pacman -Qq | grep '^casaos')" ;;
    *)      echo "  (use your distro's package manager to remove casaos*)" ;;
  esac
}

if ! command -v docker &>/dev/null; then
  echo "ERROR: Docker is not installed. Install Docker Engine first." >&2
  echo "  Detected distro family: $DISTRO_FAMILY" >&2
  echo "$(docker_install_hint)" >&2
  echo "  Or see https://docs.docker.com/engine/install/" >&2
  exit 1
fi

# ── CasaOS coexistence check ────────────────────────────────────────────
# PowerLab is a fork of CasaOS. On hosts where the upstream CasaOS is
# already installed, the two can technically coexist (different ports,
# different data dirs, different docker labels) but the experience is
# confusing: users browse to port 80 (CasaOS), see a CasaOS UI, and
# think PowerLab is broken. We detect this and:
#   · default: warn loudly, list what we found, refuse to install
#   · --allow-coexist: print the warning but proceed anyway. PowerLab
#     comes up on its own port, with its own services. The user is
#     responsible for knowing they have two products on one host.
CASAOS_UNITS=$(systemctl list-unit-files --no-pager --no-legend 'casaos*.service' 2>/dev/null | awk '{print $1}' | grep -v '^$' || true)
# If PowerLab is already installed AND CasaOS exists, the user clearly
# already chose coexistence on a previous run — treat this run as
# implicitly --allow-coexist so the in-UI updater (which calls
# `install.sh --upgrade` without the flag) can still do its job.
if [[ -n "$CASAOS_UNITS" ]] && [[ -f /etc/systemd/system/powerlab-gateway.service ]]; then
  ALLOW_COEXIST=1
fi
if [[ -n "$CASAOS_UNITS" ]]; then
  echo ""
  echo "═══════════════════════════════════════════════════════════════════"
  echo "  ⚠  Existing CasaOS installation detected on this host."
  echo "═══════════════════════════════════════════════════════════════════"
  echo ""
  echo "  Active CasaOS units:"
  while IFS= read -r u; do echo "    · $u"; done <<< "$CASAOS_UNITS"
  echo ""
  echo "  Why this matters:"
  echo "    PowerLab is a fork of CasaOS. They use different ports,"
  echo "    different config dirs, and tag containers differently — so"
  echo "    technically they coexist. But:"
  echo "      · You'll have TWO web panels (CasaOS at :80, PowerLab at :8765)"
  echo "      · Apps installed in CasaOS won't appear in PowerLab and"
  echo "        vice-versa (different createdBy labels)"
  echo "      · Both fight for the avahi multicast socket"
  echo ""
  echo "  Options:"
  if (( ALLOW_COEXIST )); then
    echo "    [ ✔ ] You passed --allow-coexist; proceeding."
    echo "          PowerLab will install on a separate port. Browse to"
    echo "          http://<this-host>:8765 to use it. CasaOS continues"
    echo "          on http://<this-host>/ untouched."
    echo ""
  else
    echo "    A) Remove CasaOS first (recommended):"
    echo "         sudo systemctl disable --now casaos casaos-gateway \\"
    echo "             casaos-app-management casaos-message-bus \\"
    echo "             casaos-user-service casaos-local-storage"
    echo "         sudo $(casaos_uninstall_hint | sed 's/^  //')"
    echo "         (your /DATA volumes and Docker containers are preserved)"
    echo ""
    echo "    B) Keep both side by side: re-run with --allow-coexist:"
    echo "         curl -fsSL .../install.sh | sudo bash -s -- --allow-coexist"
    echo ""
    echo "  Refusing to install. (No changes made.)"
    echo "═══════════════════════════════════════════════════════════════════"
    exit 1
  fi
fi

HERE="$(cd "$(dirname "$0")" && pwd)"

# ── Upgrade snapshot ────────────────────────────────────────────────────
SNAPSHOT_DIR=""
PREVIOUS_VERSION=""
if (( UPGRADE_MODE )); then
  TS=$(date -u +%Y%m%dT%H%M%SZ)
  PREVIOUS_VERSION=$(awk -F'=' '/^[[:space:]]*VERSION[[:space:]]*=/ {gsub(/[[:space:]"]/,"",$2); print $2; exit}' /etc/powerlab/version 2>/dev/null || echo "unknown")
  SNAPSHOT_DIR="/var/lib/powerlab/backups/pre-upgrade-${TS}"
  echo "[powerlab-install] Upgrade mode — taking snapshot to $SNAPSHOT_DIR"
  mkdir -p "$SNAPSHOT_DIR"/{etc,bin,systemd,db,share}
  # Configs (small, always)
  if [[ -d /etc/powerlab ]]; then cp -a /etc/powerlab/. "$SNAPSHOT_DIR/etc/" 2>/dev/null || true; fi
  # Binaries (5-6 files, ~100MB total)
  cp /usr/bin/powerlab-* "$SNAPSHOT_DIR/bin/" 2>/dev/null || true
  # Systemd units
  cp /etc/systemd/system/powerlab-*.service "$SNAPSHOT_DIR/systemd/" 2>/dev/null || true
  # User DB (tiny — just o_users table + key/value)
  if [[ -f /var/lib/powerlab/db ]]; then cp -a /var/lib/powerlab/db "$SNAPSHOT_DIR/db/" 2>/dev/null || true; fi
  # Static UI (the only large item — symlinks instead of copying when
  # disk is tight; copies are safer though)
  if [[ -d /usr/share/powerlab/www ]]; then cp -a /usr/share/powerlab/www "$SNAPSHOT_DIR/share/" 2>/dev/null || true; fi
  echo "[powerlab-install]   snapshot: $(du -sh "$SNAPSHOT_DIR" | awk '{print $1}')"
fi

echo "[powerlab-install] Creating directories..."
install -d -m 0755 /etc/powerlab
# /var/lib/powerlab/db is critical: message-bus's persistent SQLite db
# lives at /var/lib/powerlab/db/message-bus.db. message-bus's repository
# code only mkdir's the *runtime* db path, not the persist db path, so
# without this dir it panics on first start with "out of memory (14)"
# (sqlite's confusing rendering of SQLITE_CANTOPEN).
install -d -m 0755 /var/lib/powerlab/{apps,appstore,conf,backups,db}
install -d -m 0755 /var/log/powerlab
install -d -m 0755 /var/run/powerlab
install -d -m 0755 /usr/share/powerlab
install -d -m 0755 /DATA/AppData

echo "[powerlab-install] Installing binaries to /usr/bin..."
install -m 0755 "$HERE/bin/powerlab-"* /usr/bin/

echo "[powerlab-install] Installing static UI to /usr/share/powerlab/www..."
rm -rf /usr/share/powerlab/www
cp -R "$HERE/www" /usr/share/powerlab/www

# Install the PAM service policy at /etc/pam.d/powerlab. PowerLab's
# Linux auth path (auth_os_pam_linux.go) prefers this service over the
# more-restrictive `login` policy, which on many distros pulls in
# pam_nologin / pam_securetty / MOTD modules that block valid web-panel
# logins. The policy below is intentionally minimal — pam_unix.so only,
# meaning auth flows straight to /etc/shadow with whatever hash
# algorithm the host uses (yescrypt, SHA-512, …) and account validity
# checks (locked, expired) are honoured. Admins who want to layer
# additional modules (pam_tally2, pam_faillock, pam_2fa, …) can edit
# this file in place — install.sh refuses to overwrite an existing one.
if [[ ! -f /etc/pam.d/powerlab ]]; then
  echo "[powerlab-install] Installing PAM policy at /etc/pam.d/powerlab..."
  cat > /etc/pam.d/powerlab <<'PAM_EOF'
#%PAM-1.0
# PowerLab web-panel authentication policy.
# Edit to add 2FA, faillock, etc.; install.sh leaves an existing file
# untouched on upgrades.
auth     required   pam_unix.so
account  required   pam_unix.so
PAM_EOF
  chmod 0644 /etc/pam.d/powerlab
fi

# Snapshot whether gateway.ini ALREADY existed before this install — used
# to decide whether the user has explicit config we must preserve, or if
# this is a fresh install where we may probe for a free port. Captured
# *before* the sample-copy step below.
GATEWAY_INI_PREEXISTED=no
[[ -f /etc/powerlab/gateway.ini ]] && GATEWAY_INI_PREEXISTED=yes

echo "[powerlab-install] Installing sample configs to /etc/powerlab/ (only if not present)..."
for sample in "$HERE/conf/"*.sample; do
  base="$(basename "${sample%.sample}")"
  if [[ ! -f "/etc/powerlab/$base" ]]; then
    install -m 0644 "$sample" "/etc/powerlab/$base"
  fi
done

# Stop any running PowerLab services BEFORE probing for free ports.
# Without this step the port probe sees our own previous gateway holding
# the configured port and incorrectly classifies it as "busy", causing
# upgrades to silently move the gateway to a different port. Stopping
# first guarantees the probe sees the real state of the host.
SERVICES=(gateway message-bus user-service core app-management local-storage)
for svc in "${SERVICES[@]}"; do
  systemctl stop "powerlab-$svc.service" 2>/dev/null || true
done

# ── Migrate /var/lib/casaos/* → /var/lib/powerlab/* (issue #158) ─────────
# v0.5.4 flipped service paths from /var/lib/casaos to /var/lib/powerlab
# (per PR #140), but install.sh did not migrate existing data. Result on
# a v0.5.x → v0.5.4 upgrade: user-service started fresh against an empty
# /var/lib/powerlab/db/, no users, login returns 400, UI completely
# unusable. Hot-fixed on the affected host by manually copying the DBs.
#
# Logic is in scripts/migrate-casaos-data.sh and tested by
# scripts/migrate-casaos-data_test.sh — kept in a standalone file so
# the regression test can exercise it directly with PREFIX override.
if [[ -f "$HERE/migrate-casaos-data.sh" ]]; then
  # shellcheck disable=SC1091
  source "$HERE/migrate-casaos-data.sh"
  migrate_casaos_data
fi

# ── Pick the gateway port and write it into gateway.ini ─────────────────
# Decision matrix:
#   · Pre-existing gateway.ini → trust it. The user (or a previous
#     install) already chose a port; never silently overwrite their
#     decision on upgrade. The "stop services first" step above means
#     we are not fooled by an outdated probe result.
#   · Fresh install → probe in this order: 8765 (IANA-unassigned, the
#     PowerLab default — and crucially a non-standard port, which
#     bypasses Chrome's HTTPS-First Mode that fires on :80), then
#     8766..8775, then 80 as last-resort fallback.
echo "[powerlab-install] Selecting gateway port..."
port_in_use() {
  ss -tlnH 2>/dev/null | awk '{print $4}' | sed -E 's|.*:([0-9]+)$|\1|' | grep -qx "$1"
}

CURRENT_PORT="$(awk -F'=' '/^[[:space:]]*Port[[:space:]]*=/ {gsub(/[[:space:]]/,"",$2); print $2; exit}' /etc/powerlab/gateway.ini 2>/dev/null || true)"
CHOSEN_PORT=""

if [[ "$GATEWAY_INI_PREEXISTED" == "yes" ]] && [[ "$CURRENT_PORT" =~ ^[0-9]+$ ]]; then
  # Existing install: respect the configured port unconditionally.
  CHOSEN_PORT="$CURRENT_PORT"
  echo "[powerlab-install]   → keeping configured port $CHOSEN_PORT"
else
  # Fresh install: probe.
  for p in 8765 8766 8767 8768 8769 8770 8771 8772 8773 8774 8775 80; do
    if ! port_in_use "$p"; then CHOSEN_PORT="$p"; break; fi
  done
  if [[ -z "$CHOSEN_PORT" ]]; then
    echo "ERROR: could not find a free port for the PowerLab gateway." >&2
    exit 1
  fi
  echo "[powerlab-install]   → fresh install, using port $CHOSEN_PORT"
  # Rewrite the Port = line in the freshly-copied gateway.ini.
  sed -i.bak -E "s/^[[:space:]]*Port[[:space:]]*=.*/Port = $CHOSEN_PORT/" /etc/powerlab/gateway.ini
  rm -f /etc/powerlab/gateway.ini.bak
fi

echo "[powerlab-install] Installing systemd units..."
install -m 0644 "$HERE/systemd/"*.service /etc/systemd/system/

# Self-heal: older PowerLab releases (≤ v0.1.1) shipped a gateway unit
# with a bogus `-c /etc/powerlab/gateway.conf` flag the binary doesn't
# accept, leaving the service stuck in restart-loop. Strip it on every
# install so existing hosts auto-recover when they upgrade.
sed -i 's| -c /etc/powerlab/gateway.conf||g' /etc/systemd/system/powerlab-gateway.service

systemctl daemon-reload

echo "[powerlab-install] Enabling and starting services..."
SERVICES=(gateway message-bus user-service core app-management local-storage)
for svc in "${SERVICES[@]}"; do
  systemctl enable "powerlab-$svc.service" >/dev/null
  systemctl reset-failed "powerlab-$svc.service" 2>/dev/null || true
  systemctl restart "powerlab-$svc.service"
done

# Wait briefly for gateway to come up + verify it's actually listening on
# the port we just chose.
sleep 2
GATEWAY_UP=no
for _ in 1 2 3 4 5; do
  if curl -fsS -m 1 "http://127.0.0.1:${CHOSEN_PORT}/" >/dev/null 2>&1; then
    GATEWAY_UP=yes
    break
  fi
  sleep 1
done

# Persist the version this install put down so future runs know what
# to roll back FROM. Done unconditionally (fresh install + upgrade)
# so an upgrade can always read PREVIOUS_VERSION cleanly. The
# VERSION_NEXT bash variable was sed-injected by package-linux.sh
# right after the heredoc.
if [[ "$GATEWAY_UP" == "yes" ]]; then
  echo "VERSION = \"${VERSION_NEXT:-unknown}\"" > /etc/powerlab/version
fi

# ── Upgrade rollback (only when --upgrade was passed) ──────────────────
if (( UPGRADE_MODE )); then
  if [[ "$GATEWAY_UP" == "yes" ]]; then
    NEW_VERSION="${VERSION_NEXT:-unknown}"
    echo "[powerlab-install]   upgrade succeeded — gateway responding on $CHOSEN_PORT"
    cat > /var/lib/powerlab/last-upgrade.json <<JSON
{
  "from": "${PREVIOUS_VERSION}",
  "to": "${NEW_VERSION}",
  "result": "success",
  "succeeded_at": "$(date -u +%FT%TZ)",
  "snapshot_path": "${SNAPSHOT_DIR}"
}
JSON
  else
    echo "[powerlab-install]   upgrade FAILED — gateway not responding. Rolling back from snapshot."
    # Stop everything before swapping back so the kernel does not
    # hold open file handles to the broken binaries.
    for svc in gateway message-bus user-service core app-management local-storage; do
      systemctl stop "powerlab-$svc.service" 2>/dev/null || true
    done
    # Restore from snapshot. We deliberately do NOT touch
    # /var/lib/powerlab/{apps,appstore} — those are user data and
    # were never modified by the upgrade either.
    cp -a "$SNAPSHOT_DIR"/etc/. /etc/powerlab/ 2>/dev/null || true
    cp -a "$SNAPSHOT_DIR"/bin/. /usr/bin/ 2>/dev/null || true
    cp -a "$SNAPSHOT_DIR"/systemd/. /etc/systemd/system/ 2>/dev/null || true
    if [[ -f "$SNAPSHOT_DIR/db/db" ]]; then cp -a "$SNAPSHOT_DIR/db/db" /var/lib/powerlab/db; fi
    if [[ -d "$SNAPSHOT_DIR/share/www" ]]; then rm -rf /usr/share/powerlab/www && cp -a "$SNAPSHOT_DIR/share/www" /usr/share/powerlab/www; fi
    systemctl daemon-reload
    for svc in gateway message-bus user-service core app-management local-storage; do
      systemctl reset-failed "powerlab-$svc.service" 2>/dev/null || true
      systemctl start "powerlab-$svc.service"
    done
    sleep 4
    cat > /var/lib/powerlab/last-upgrade.json <<JSON
{
  "from": "${PREVIOUS_VERSION}",
  "to": "unknown",
  "result": "rolled_back",
  "failed_at": "$(date -u +%FT%TZ)",
  "snapshot_path": "${SNAPSHOT_DIR}",
  "diagnostic": "Gateway did not respond on http://127.0.0.1:${CHOSEN_PORT}/ within 7 seconds after the binary swap. Snapshot restored. Check journalctl -u powerlab-gateway for the underlying failure."
}
JSON
    echo "[powerlab-install]   rolled back to previous version. last-upgrade.json written."
    exit 1
  fi
fi

# Detect the primary LAN IP (first non-loopback IPv4 address). `hostname -I`
# is Linux-specific and returns space-separated IPs.
LAN_IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
HOSTNAME_SHORT="$(hostname -s 2>/dev/null || hostname)"

GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[0;33m'
DIM='\033[2m'
RESET='\033[0m'

echo ""
if [[ "$GATEWAY_UP" == "yes" ]]; then
  printf "${GREEN}┌─────────────────────────────────────────────────────────────┐${RESET}\n"
  printf "${GREEN}│${RESET}  ${GREEN}✓ PowerLab is installed and running${RESET}                       ${GREEN}│${RESET}\n"
  printf "${GREEN}└─────────────────────────────────────────────────────────────┘${RESET}\n"
else
  printf "  PowerLab is installed but the gateway is not responding yet.\n"
  printf "  Check the logs: journalctl -u powerlab-gateway -f\n"
fi

# Browser URLs include the chosen port unless it's 80 (where the browser
# implicitly assumes :80 and we'd just be adding noise).
PORT_SUFFIX=""
if [[ "$CHOSEN_PORT" != "80" ]]; then
  PORT_SUFFIX=":$CHOSEN_PORT"
fi

echo ""
if [[ -n "$CASAOS_UNITS" ]]; then
  printf "  ${YELLOW}⚠  CasaOS is also installed on this host.${RESET}\n"
  printf "     PowerLab is at port ${PORT_SUFFIX#:} (URLs below).\n"
  printf "     Your existing CasaOS continues at http://<this-host>/ (port 80).\n"
  printf "     ${DIM}Apps you install in PowerLab will NOT appear in CasaOS, and vice versa.${RESET}\n"
  echo ""
fi
echo "  Open PowerLab in your browser:"
echo ""
printf "    ${CYAN}→  http://localhost${PORT_SUFFIX}${RESET}                ${DIM}(on this machine)${RESET}\n"
if [[ -n "$LAN_IP" ]]; then
  printf "    ${CYAN}→  http://${LAN_IP}${PORT_SUFFIX}${RESET}      ${DIM}(any device on this LAN)${RESET}\n"
fi
# mDNS hostname: avahi only publishes the system's own hostname.
# `powerlab.local` only resolves if the host's static hostname is
# literally `powerlab`. We print the host's actual hostname here, not
# the misleading `powerlab.local` (which used to suggest mDNS aliasing
# we don't actually have — see issue #33).
if [[ -n "$HOSTNAME_SHORT" ]]; then
  printf "    ${CYAN}→  http://${HOSTNAME_SHORT}.local${PORT_SUFFIX}${RESET}      ${DIM}(via Bonjour/mDNS — requires nss-mdns on Linux clients)${RESET}\n"
  if [[ "$HOSTNAME_SHORT" != "powerlab" ]]; then
    printf "    ${DIM}                                            (To use http://powerlab.local, run: sudo hostnamectl set-hostname powerlab && sudo systemctl restart avahi-daemon powerlab-gateway)${RESET}\n"
  fi
fi
echo ""
echo "  First sign-in: use your operating-system username and password."
echo ""
printf "  ${DIM}Service status: systemctl status powerlab-gateway${RESET}\n"
printf "  ${DIM}Logs:           journalctl -u powerlab-gateway -f${RESET}\n"
printf "  ${DIM}Uninstall:      sudo /usr/share/powerlab/uninstall.sh${RESET}\n"
echo ""
INSTALL_EOF
# Inject the version this release advertises so the rollback path can
# write `VERSION = "X.Y.Z"` to /etc/powerlab/version on success and
# stamp the last-upgrade.json record. The placeholder is at the top
# of the file (right after `set -euo pipefail`) so the awk in the
# rollback block finds it without recursing through the body.
sed -i.bak "1a\\
VERSION_NEXT=\"$VERSION\"
" "$STAGE/install.sh"
rm -f "$STAGE/install.sh.bak"
chmod +x "$STAGE/install.sh"

# Ship the standalone CasaOS-data migration script alongside install.sh
# (issue #158). install.sh sources it via `$HERE/migrate-casaos-data.sh`.
# The script is also unit-tested directly via
# scripts/migrate-casaos-data_test.sh — keeping it standalone keeps the
# test surface small.
cp "$ROOT/scripts/migrate-casaos-data.sh" "$STAGE/migrate-casaos-data.sh"
chmod +x "$STAGE/migrate-casaos-data.sh"

# ─── 7. uninstall.sh ─────────────────────────────────────────────────────
cat > "$STAGE/uninstall.sh" <<'UNINSTALL_EOF'
#!/usr/bin/env bash
# PowerLab uninstaller — run as root. Does NOT delete /DATA/AppData by default.
set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "ERROR: uninstall.sh must be run as root." >&2
  exit 1
fi

SERVICES=(gateway message-bus user-service core app-management local-storage)

echo "[powerlab-uninstall] Stopping services..."
for svc in "${SERVICES[@]}"; do
  systemctl disable --now "powerlab-$svc.service" 2>/dev/null || true
  rm -f "/etc/systemd/system/powerlab-$svc.service"
done
systemctl daemon-reload

echo "[powerlab-uninstall] Removing binaries and static UI..."
rm -f /usr/bin/powerlab-*
rm -rf /usr/share/powerlab

echo "[powerlab-uninstall] Done. Configs in /etc/powerlab and data in /DATA/AppData kept."
echo "  Remove manually if desired:  rm -rf /etc/powerlab /var/lib/powerlab /var/log/powerlab /var/run/powerlab"
UNINSTALL_EOF
chmod +x "$STAGE/uninstall.sh"

# ─── 8. README ───────────────────────────────────────────────────────────
cat > "$STAGE/README.md" <<EOF
# PowerLab v$VERSION (linux-$ARCH)

## Install

\`\`\`bash
sudo ./install.sh
\`\`\`

Requires Docker Engine ≥ 20.10. The installer creates \`/etc/powerlab\`,
\`/var/lib/powerlab\`, \`/var/log/powerlab\`, \`/var/run/powerlab\`, and
\`/DATA/AppData\` (for app data volumes).

After install: open **http://powerlab.local** from any device on your LAN
(via Bonjour/mDNS). Or **http://localhost** on the host itself.

## Uninstall

\`\`\`bash
sudo ./uninstall.sh
\`\`\`

App data in \`/DATA/AppData\` is preserved by default.

## Layout

| Path | Contents |
|---|---|
| /usr/bin/powerlab-* | Service binaries |
| /usr/share/powerlab/www | Static SPA |
| /etc/powerlab/*.conf | Configuration |
| /var/lib/powerlab | App store cache, app YAMLs |
| /var/log/powerlab | Service logs |
| /var/run/powerlab | PID files, runtime URLs |
| /DATA/AppData | Bind-mount root for installed apps |
EOF

# ─── 9. Tarball ──────────────────────────────────────────────────────────
log "Creating tarball..."
cd "$OUT"
tar -czf "$TARBALL" "$(basename "$STAGE")"

# Also publish under a stable, version-less filename so the
# `releases/latest/download/powerlab-linux-<arch>.tar.gz` URL in the README
# keeps working across releases. Both tarballs are identical bytes — the
# stable name is just a copy.
STABLE_TARBALL="$OUT/powerlab-linux-$ARCH.tar.gz"
cp "$TARBALL" "$STABLE_TARBALL"

# ─── 10. Release manifest ───────────────────────────────────────────────
# Emit `manifest.json` describing this release. The host-side updater
# (issue #21, see docs/UPDATE_MANIFEST.md) fetches this file to decide
# whether to offer the upgrade. We write two copies:
#   · $OUT/manifest.json — uploaded as a release asset so the updater
#     can fetch ~2 KB of metadata before pulling the 60 MB tarball
#   · $STAGE/manifest.json — re-tarred into the bundle so install.sh
#     inside the tarball has the same view of what it shipped
#
# The build-manifest tool needs both arch tarballs to compute the
# per-arch SHA-256 + size. We invoke it with whichever tarball this
# script just produced; if the OTHER arch's tarball is sitting in
# $OUT from a previous run, include it too.
log "Generating manifest.json..."
MANIFEST_ARGS=(-version "$VERSION" -repo neochaotic/powerlab)
if [[ -f "$OUT/powerlab-${VERSION}-linux-amd64.tar.gz" ]]; then
  MANIFEST_ARGS+=(-amd64-tarball "$OUT/powerlab-${VERSION}-linux-amd64.tar.gz")
fi
if [[ -f "$OUT/powerlab-${VERSION}-linux-arm64.tar.gz" ]]; then
  MANIFEST_ARGS+=(-arm64-tarball "$OUT/powerlab-${VERSION}-linux-arm64.tar.gz")
fi
(cd "$ROOT" && go run ./scripts/build-manifest "${MANIFEST_ARGS[@]}") > "$OUT/manifest.json"
cp "$OUT/manifest.json" "$STAGE/manifest.json"

# Re-tar the bundle so the new manifest.json is included. We could
# instead tar.append, but rebuilding from scratch is simpler and the
# extra second of tar work is invisible compared to the npm build.
tar -czf "$TARBALL" "$(basename "$STAGE")"
cp "$TARBALL" "$STABLE_TARBALL"

SIZE=$(du -sh "$TARBALL" | awk '{print $1}')
log "Done."
log ""
log "  $TARBALL ($SIZE)"
log "  $STABLE_TARBALL (stable URL)"
log "  $OUT/manifest.json (release manifest — uploaded as separate asset)"
log ""
log "Deploy: copy to a Linux host, extract, and run sudo ./install.sh"
