#!/usr/bin/env bash
# PowerLab Development Start Script
# Starts all backend services in the correct order, then the UI dev server.
# Run from the powerlab/ project root.
#
# Usage:
#   ./start.sh          — start all services
#   ./start.sh --build  — rebuild binaries before starting
#   ./start.sh --stop   — stop all running services

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND="$SCRIPT_DIR/backend"
RUNTIME="$BACKEND/runtime"
LOGS="$BACKEND/logs"
DATA="$BACKEND/data"
CONF="$BACKEND/conf"
PIDS_FILE="$RUNTIME/powerlab.pids"

# ── helpers ────────────────────────────────────────────────────────────────────

log()  { echo "[powerlab] $*"; }
die()  { echo "[powerlab] ERROR: $*" >&2; exit 1; }

# ── systemd detection ──────────────────────────────────────────────────────────

USE_SYSTEMD=false
if [[ "$(uname)" == "Linux" ]] && [[ -d "/etc/powerlab" ]] && command -v systemctl >/dev/null 2>&1; then
  if systemctl list-units "powerlab-*" >/dev/null 2>&1; then
    USE_SYSTEMD=true
  fi
fi

if [[ "$USE_SYSTEMD" == true ]]; then
  log "SystemD detected with /etc/powerlab config."
  log "Restarting production services..."
  sudo systemctl restart "powerlab-*"
  log "Done."
  exit 0
fi

wait_for_file() {
  local file="$1" timeout="${2:-15}" i=0
  while [[ ! -f "$file" ]] && (( i < timeout )); do
    sleep 1; (( i++ ))
  done
  [[ -f "$file" ]] || die "timed out waiting for $file"
}

stop_services() {
  log "Stopping services..."
  if [[ -f "$PIDS_FILE" ]]; then
    while read -r pid; do
      kill "$pid" 2>/dev/null || true
    done < "$PIDS_FILE"
    rm -f "$PIDS_FILE"
  fi
  # Kill any stray binaries in the runtime bin dir
  pkill -f "$RUNTIME/bin/" 2>/dev/null || true
  # Kill anything still on port 80 (gateway may have been started outside this script)
  local port80_pid
  port80_pid=$(lsof -ti :80 2>/dev/null || true)
  if [[ -n "$port80_pid" ]]; then
    log "  Killing leftover gateway on port 80 (PID $port80_pid)..."
    kill "$port80_pid" 2>/dev/null || true
  fi
  sleep 1
  log "Done."
  exit 0
}

start_service() {
  local name="$1"
  local bin="$RUNTIME/bin/$name"
  local svc_dir="$BACKEND/$name"
  local conf="$CONF/$name.conf"
  log "Starting $name..."
  mkdir -p "$RUNTIME/bin"
  # cd into the service source dir so os.Getwd() resolves go.mod and dev paths
  # correctly. Pass -c so services don't try to open the production default
  # /etc/casaos/<svc>.conf (which doesn't exist in dev).
  if [[ -f "$conf" ]]; then
    (cd "$svc_dir" && exec "$bin" -c "$conf") >> "$LOGS/$name.log" 2>&1 &
  else
    (cd "$svc_dir" && exec "$bin") >> "$LOGS/$name.log" 2>&1 &
  fi
  echo $! >> "$PIDS_FILE"
  log "  $name started (PID $!)"
}

# ── argument handling ──────────────────────────────────────────────────────────

BUILD=false
WATCH=false
for arg in "$@"; do
  case "$arg" in
    --build) BUILD=true ;;
    --stop)  stop_services ;;
    --watch) WATCH=true ;;
    *) die "Unknown argument: $arg" ;;
  esac
done

# ── directory setup ────────────────────────────────────────────────────────────

mkdir -p "$RUNTIME" "$LOGS" "$DATA/apps" "$DATA/appstore" "$DATA/files" "$CONF"
rm -f "$PIDS_FILE"

# Clear stale service URL files so wait_for_file actually waits for the new gateway
rm -f "$RUNTIME/management.url" "$RUNTIME/"*.url

# ── write dev conf files ───────────────────────────────────────────────────────

write_conf_if_absent() {
  local path="$1" content="$2"
  if [[ ! -f "$path" ]]; then
    printf "%s\n" "$content" > "$path"
    log "Created $path"
  fi
}

write_conf_if_absent "$CONF/message-bus.conf" "[common]
RuntimePath=$RUNTIME

[app]
LogPath=$LOGS
LogSaveName=message-bus
LogFileExt=log
DBPath=$DATA"

write_conf_if_absent "$CONF/user-service.conf" "[common]
RuntimePath=$RUNTIME

[app]
LogPath=$LOGS
LogSaveName=user-service
LogFileExt=log
DBPath=$DATA/db
UserDataPath=$DATA"

write_conf_if_absent "$CONF/local-storage.conf" "[common]
RuntimePath=$RUNTIME

[app]
LogPath=$LOGS
LogSaveName=local-storage
LogFileExt=log
DBPath=$DATA/db
ShellPath=$BACKEND/share/shell

[server]
USBAutoMount=
EnableMergerFS=false"

write_conf_if_absent "$CONF/casaos.conf" "[app]
PAGE_SIZE=10
RuntimeRootPath=$RUNTIME/
LogPath=$LOGS/
LogSaveName=core
LogFileExt=log
DBPath=$DATA
ShellPath=$BACKEND/share/shell
UserDataPath=$DATA/conf

[server]
RunMode=release"

# Always overwrite app-management.conf with current absolute paths
cat > "$CONF/app-management.conf" << EOF
[common]
RuntimePath=$RUNTIME

[app]
LogPath=$LOGS
LogSaveName=app-management
LogFileExt=log
AppStorePath=$DATA/appstore
AppsPath=$DATA/apps

[server]
appstore=$SCRIPT_DIR/store
appstore=https://cdn.jsdelivr.net/gh/IceWhaleTech/CasaOS-AppStore@gh-pages/store/main.zip
appstore=https://github.com/bigbeartechworld/big-bear-casaos/archive/refs/heads/master.zip
EOF

# ── service list (OS-aware) ────────────────────────────────────────────────────

# local-storage uses bazil.org/fuse + Linux extended attribute syscalls.
# It only builds on Linux. On macOS, the Files page is unavailable in dev mode.
if [[ "$(uname)" == "Linux" ]]; then
  SERVICES=(gateway message-bus user-service local-storage core app-management)
else
  SERVICES=(gateway message-bus user-service core app-management)
  log "Note: running on macOS — local-storage skipped (Linux-only, fuse/mergerfs not available)."
  log "      The Files page will not work in this dev environment."
fi

# ── build ──────────────────────────────────────────────────────────────────────

function sync_specs() {
  # Mirror each canonical per-service openapi.yaml into the gateway's
  # embedded docs dir so the API docs portal at /docs always serves
  # what the backend was built against (ADR 0008). Strict: if a source
  # spec is missing we fail the build instead of silently shipping a
  # stale embedded copy.
  log "Syncing OpenAPI specs..."
  mkdir -p backend/gateway/api/docs
  local sync_pairs=(
    "backend/gateway/api/gateway/openapi.yaml:backend/gateway/api/docs/openapi_gateway.yaml"
    "backend/app-management/api/app_management/openapi.yaml:backend/gateway/api/docs/openapi_app_management.yaml"
    "backend/message-bus/api/message_bus/openapi.yaml:backend/gateway/api/docs/openapi_message_bus.yaml"
    "backend/core/api/casaos/openapi.yaml:backend/gateway/api/docs/openapi_core.yaml"
    "backend/local-storage/api/local_storage/openapi.yaml:backend/gateway/api/docs/openapi_local_storage.yaml"
    "backend/user-service/api/user-service/openapi.yaml:backend/gateway/api/docs/openapi_user_service.yaml"
  )
  for pair in "${sync_pairs[@]}"; do
    src="${pair%%:*}"
    dst="${pair#*:}"
    [[ -f "$src" ]] || die "OpenAPI source missing: $src — embedded portal would be stale"
    cp "$src" "$dst"
  done
}

if [[ "$BUILD" == true ]]; then
  sync_specs
  log "Building all services..."
  for svc in "${SERVICES[@]}"; do
    if [[ "$svc" == "local-storage" ]]; then
      (cd "$BACKEND/$svc" && go generate ./... > /dev/null 2>&1; go build -o "$RUNTIME/bin/$svc" .) || die "Failed to build $svc"
    else
      (cd "$BACKEND/$svc" && go build -o "$RUNTIME/bin/$svc" .) || die "Failed to build $svc"
    fi
    log "  $svc built."
  done
  log "All services built."
else
  sync_specs
  # Build only if binary is missing
  for svc in "${SERVICES[@]}"; do
    bin="$RUNTIME/bin/$svc"
    if [[ ! -f "$bin" ]]; then
      log "Binary missing for $svc — building..."
      if [[ "$svc" == "local-storage" ]]; then
        (cd "$BACKEND/$svc" && go generate ./... > /dev/null 2>&1; go build -o "$bin" .) || die "Failed to build $svc"
      else
        (cd "$BACKEND/$svc" && go build -o "$bin" .) || die "Failed to build $svc"
      fi
    fi
  done
fi

# ── start services ─────────────────────────────────────────────────────────────

# 1. Gateway must start first — it writes management.url that other services need
log "Starting gateway..."
(cd "$BACKEND/gateway" && exec "$RUNTIME/bin/gateway") >> "$LOGS/gateway.log" 2>&1 &
echo $! >> "$PIDS_FILE"
log "  gateway started (PID $!)"

log "Waiting for gateway management URL..."
wait_for_file "$RUNTIME/management.url" 15
log "  gateway ready: $(cat "$RUNTIME/management.url")"

# 2. Remaining services (all except gateway, which is already running)
for svc in "${SERVICES[@]}"; do
  [[ "$svc" == "gateway" ]] && continue
  start_service "$svc" "$BACKEND/$svc"
  sleep 0.5
done

log ""
log "All backend services are running. PIDs saved to $PIDS_FILE"
log "Logs:    $LOGS/"
log "Runtime: $RUNTIME/"
log ""
log "PowerLab is reachable at:"
log "  http://localhost           (this machine)"
log "  http://powerlab.local      (any device on this LAN — via Bonjour/mDNS)"
log ""
log "To stop: ./start.sh --stop"
log "To rebuild and restart: ./start.sh --stop && ./start.sh --build"
log "To watch and auto-restart on crash: ./start.sh --watch"

# ── watchdog (only if --watch passed) ──────────────────────────────────────────
# Loops every 5s checking each PID. If a process is gone, rebuilds nothing —
# just relaunches the binary so the service comes back. This mirrors what
# systemd does in production with Restart=always.
if [[ "${WATCH:-false}" == true ]]; then
  log ""
  log "Watchdog active. Press Ctrl+C to stop the watcher (services keep running)."
  trap 'log ""; log "Watchdog stopped (services still running)."; exit 0' INT
  while true; do
    sleep 5
    # Gateway is special — its PID is the first line of the file
    GW_PID=$(head -1 "$PIDS_FILE" 2>/dev/null)
    if [[ -n "$GW_PID" ]] && ! kill -0 "$GW_PID" 2>/dev/null; then
      log "Gateway (PID $GW_PID) died — restarting..."
      (cd "$BACKEND/gateway" && exec "$RUNTIME/bin/gateway") >> "$LOGS/gateway.log" 2>&1 &
      sed -i.bak "1s/.*/$!/" "$PIDS_FILE" && rm -f "$PIDS_FILE.bak"
      log "  gateway restarted (PID $!)"
      continue
    fi
    # Other services
    for svc in "${SERVICES[@]}"; do
      [[ "$svc" == "gateway" ]] && continue
      bin="$RUNTIME/bin/$svc"
      # Find the PID for this service among the saved PIDs
      pid=$(pgrep -fx "$bin" 2>/dev/null | head -1)
      if [[ -z "$pid" ]]; then
        log "$svc died — restarting..."
        start_service "$svc" "$BACKEND/$svc"
      fi
    done
  done
fi
