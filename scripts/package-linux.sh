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
Port = 80
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

# Gateway needs to know where the static UI lives. Override with `-w` flag.
sed -i.bak 's|ExecStart=/usr/bin/powerlab-gateway -c /etc/powerlab/gateway.conf|ExecStart=/usr/bin/powerlab-gateway -c /etc/powerlab/gateway.conf -w /usr/share/powerlab/www|' \
  "$STAGE/systemd/powerlab-gateway.service"
rm -f "$STAGE/systemd/powerlab-gateway.service.bak"

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

echo "[powerlab-install] Installing systemd units..."
install -m 0644 "$HERE/systemd/"*.service /etc/systemd/system/
systemctl daemon-reload

echo "[powerlab-install] Enabling and starting services..."
SERVICES=(gateway message-bus user-service core app-management local-storage)
for svc in "${SERVICES[@]}"; do
  systemctl enable "powerlab-$svc.service" >/dev/null
  systemctl restart "powerlab-$svc.service"
done

# Wait briefly for gateway to come up + verify it's actually listening
sleep 2
GATEWAY_UP=no
for _ in 1 2 3 4 5; do
  if curl -fsS -m 1 http://127.0.0.1/ >/dev/null 2>&1; then
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

echo ""
echo "  Open PowerLab in your browser:"
echo ""
printf "    ${CYAN}→  http://localhost${RESET}                ${DIM}(on this machine)${RESET}\n"
if [[ -n "$LAN_IP" ]]; then
  printf "    ${CYAN}→  http://${LAN_IP}${RESET}      ${DIM}(any device on this LAN)${RESET}\n"
fi
printf "    ${CYAN}→  http://powerlab.local${RESET}           ${DIM}(via Bonjour/mDNS)${RESET}\n"
if [[ -n "$HOSTNAME_SHORT" && "$HOSTNAME_SHORT" != "powerlab" ]]; then
  printf "    ${CYAN}→  http://${HOSTNAME_SHORT}.local${RESET}      ${DIM}(this host's own mDNS name)${RESET}\n"
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
