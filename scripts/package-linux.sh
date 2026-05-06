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
for svc in "${SERVICES[@]}"; do
  log "  building $svc..."
  cd "$ROOT/backend/$svc"
  # codegen/ is .gitignored — regenerate from the OpenAPI specs before
  # compiling so the import paths in main.go resolve. gateway has no
  # generate directives.
  if [[ "$svc" != "gateway" ]]; then
    go generate ./... > /dev/null 2>&1 || true
  fi
  # -ldflags strip debug + reduce binary size; trimpath removes local paths
  GOOS=linux GOARCH="$ARCH" CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags="-s -w -X main.version=$VERSION" \
    -o "$STAGE/bin/powerlab-$svc" \
    .
done

# ─── 3. Build frontend ───────────────────────────────────────────────────
log "Building frontend (static SPA)..."
cd "$ROOT/ui"
npm run build > /dev/null
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

cat > "$STAGE/conf/casaos.conf.sample" <<EOF
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
  cat > "$STAGE/systemd/powerlab-$svc.service" <<EOF
[Unit]
Description=PowerLab $svc service
After=network.target docker.service
Wants=docker.service

[Service]
Type=simple
ExecStart=/usr/bin/powerlab-$svc -c /etc/powerlab/$svc.conf
Restart=always
RestartSec=5
PIDFile=/var/run/powerlab/$svc.pid
Environment=DOCKER_API_VERSION=1.44

[Install]
WantedBy=multi-user.target
EOF
done

# Gateway: no -c flag (config is loaded from constants.DefaultConfigPath
# /gateway.ini at startup), only -w for the www directory.
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

[Install]
WantedBy=multi-user.target
EOF

# ─── 6. install.sh ───────────────────────────────────────────────────────
log "Generating install.sh..."
cat > "$STAGE/install.sh" <<'INSTALL_EOF'
#!/usr/bin/env bash
# PowerLab installer — run as root.
set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "ERROR: install.sh must be run as root (sudo)." >&2
  exit 1
fi

if ! command -v docker &>/dev/null; then
  echo "ERROR: Docker is not installed. Install Docker Engine first." >&2
  echo "  See https://docs.docker.com/engine/install/" >&2
  exit 1
fi

HERE="$(cd "$(dirname "$0")" && pwd)"

echo "[powerlab-install] Creating directories..."
install -d -m 0755 /etc/powerlab
install -d -m 0755 /var/lib/powerlab/{apps,appstore,conf}
install -d -m 0755 /var/log/powerlab
install -d -m 0755 /var/run/powerlab
install -d -m 0755 /usr/share/powerlab
install -d -m 0755 /DATA/AppData

echo "[powerlab-install] Installing binaries to /usr/bin..."
install -m 0755 "$HERE/bin/powerlab-"* /usr/bin/

echo "[powerlab-install] Installing static UI to /usr/share/powerlab/www..."
rm -rf /usr/share/powerlab/www
cp -R "$HERE/www" /usr/share/powerlab/www

echo "[powerlab-install] Installing sample configs to /etc/powerlab/ (only if not present)..."
for sample in "$HERE/conf/"*.sample; do
  base="$(basename "${sample%.sample}")"
  if [[ ! -f "/etc/powerlab/$base" ]]; then
    install -m 0644 "$sample" "/etc/powerlab/$base"
  fi
done

# ── Pick a free port for the gateway and write it into gateway.ini ──────
# Preferences in order: keep whatever the user already configured (if it's
# free), then 80 (the conventional choice), then 8765 (IANA unassigned,
# not used by any common self-hosted tool), then 8766..8775 as a fallback.
echo "[powerlab-install] Probing for an available HTTP port..."
port_in_use() {
  ss -tlnH 2>/dev/null | awk '{print $4}' | sed -E 's|.*:([0-9]+)$|\1|' | grep -qx "$1"
}

CURRENT_PORT="$(awk -F'=' '/^[[:space:]]*Port[[:space:]]*=/ {gsub(/[[:space:]]/,"",$2); print $2; exit}' /etc/powerlab/gateway.ini 2>/dev/null || true)"
CHOSEN_PORT=""

# 1. honour an existing config if its port is free
if [[ -n "$CURRENT_PORT" ]] && ! port_in_use "$CURRENT_PORT"; then
  CHOSEN_PORT="$CURRENT_PORT"
fi

# 2. prefer 80, then 8765, then linear scan
if [[ -z "$CHOSEN_PORT" ]]; then
  for p in 80 8765 8766 8767 8768 8769 8770 8771 8772 8773 8774 8775; do
    if ! port_in_use "$p"; then CHOSEN_PORT="$p"; break; fi
  done
fi

if [[ -z "$CHOSEN_PORT" ]]; then
  echo "ERROR: could not find a free port for the PowerLab gateway." >&2
  exit 1
fi

# Rewrite the Port = line in gateway.ini so the chosen port is what the
# gateway actually binds to.
sed -i.bak -E "s/^[[:space:]]*Port[[:space:]]*=.*/Port = $CHOSEN_PORT/" /etc/powerlab/gateway.ini
rm -f /etc/powerlab/gateway.ini.bak
echo "[powerlab-install]   → using port $CHOSEN_PORT"

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

# Detect the primary LAN IP (first non-loopback IPv4 address). `hostname -I`
# is Linux-specific and returns space-separated IPs.
LAN_IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
HOSTNAME_SHORT="$(hostname -s 2>/dev/null || hostname)"

GREEN='\033[0;32m'
CYAN='\033[0;36m'
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
echo "  Open PowerLab in your browser:"
echo ""
printf "    ${CYAN}→  http://localhost${PORT_SUFFIX}${RESET}                ${DIM}(on this machine)${RESET}\n"
if [[ -n "$LAN_IP" ]]; then
  printf "    ${CYAN}→  http://${LAN_IP}${PORT_SUFFIX}${RESET}      ${DIM}(any device on this LAN)${RESET}\n"
fi
printf "    ${CYAN}→  http://powerlab.local${PORT_SUFFIX}${RESET}           ${DIM}(via Bonjour/mDNS)${RESET}\n"
if [[ -n "$HOSTNAME_SHORT" && "$HOSTNAME_SHORT" != "powerlab" ]]; then
  printf "    ${CYAN}→  http://${HOSTNAME_SHORT}.local${PORT_SUFFIX}${RESET}      ${DIM}(this host's own mDNS name)${RESET}\n"
fi
echo ""
echo "  First sign-in: use your operating-system username and password."
echo ""
printf "  ${DIM}Service status: systemctl status powerlab-gateway${RESET}\n"
printf "  ${DIM}Logs:           journalctl -u powerlab-gateway -f${RESET}\n"
printf "  ${DIM}Uninstall:      sudo /usr/share/powerlab/uninstall.sh${RESET}\n"
echo ""
INSTALL_EOF
chmod +x "$STAGE/install.sh"

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

SIZE=$(du -sh "$TARBALL" | awk '{print $1}')
log "Done."
log ""
log "  $TARBALL ($SIZE)"
log "  $STABLE_TARBALL (stable URL)"
log ""
log "Deploy: copy to a Linux host, extract, and run sudo ./install.sh"
