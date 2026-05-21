#!/usr/bin/env bash
# Bring up the full PowerLab stack on a CI runner (background processes,
# no systemd) so the browser @smoke Playwright specs can run against a
# REAL backend — the gap that the Go backend-integration job doesn't
# cover at the UI level.
#
# Why a dedicated script (not start.sh): CI needs explicit control that
# start.sh doesn't expose — a fixed non-privileged port, the gateway
# serving the built UI (-w), and a provisioned admin user for login.
#
# Prints two lines on success, consumed by the workflow:
#   POWERLAB_E2E_BASE=http://127.0.0.1:<port>
#   (admin registered as $E2E_USER / $E2E_PASS)
#
# NOT for production. systemd-coupled specs (journald stream) are out of
# scope here — those need the packaged install + units; this stack runs
# the binaries directly.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND="$ROOT/backend"
RUNTIME="$BACKEND/runtime"
LOGS="$BACKEND/logs"
DATA="$BACKEND/data"
CONF="$BACKEND/conf"
BIN="$RUNTIME/bin"
UI_BUILD="$ROOT/ui/build"

GW_PORT="${POWERLAB_E2E_PORT:-8085}"
E2E_USER="${POWERLAB_E2E_USER:-ci-admin}"
E2E_PASS="${POWERLAB_E2E_PASSWORD:-ci-smoke-pass-12345}"

log() { echo "[ci-stack] $*"; }
die() { echo "[ci-stack] ERROR: $*" >&2; exit 1; }

mkdir -p "$RUNTIME" "$BIN" "$LOGS" "$DATA/apps" "$DATA/appstore" "$DATA/files" "$DATA/db" "$CONF"
rm -f "$RUNTIME/management.url" "$RUNTIME/"*.url "$RUNTIME/powerlab.pids"

# Linux runner: build all services (local-storage is Linux-only).
SERVICES=(gateway message-bus user-service local-storage core app-management)

# OpenAPI specs the gateway embeds for /docs (start.sh does this too).
for pair in \
  "backend/gateway/api/gateway/openapi.yaml:backend/gateway/api/docs/openapi_gateway.yaml" \
  "backend/app-management/api/app_management/openapi.yaml:backend/gateway/api/docs/openapi_app_management.yaml" \
  "backend/message-bus/api/message_bus/openapi.yaml:backend/gateway/api/docs/openapi_message_bus.yaml" \
  "backend/core/api/core/openapi.yaml:backend/gateway/api/docs/openapi_core.yaml" \
  "backend/local-storage/api/local_storage/openapi.yaml:backend/gateway/api/docs/openapi_local_storage.yaml" \
  "backend/user-service/api/user-service/openapi.yaml:backend/gateway/api/docs/openapi_user_service.yaml"; do
  mkdir -p backend/gateway/api/docs
  cp "$ROOT/${pair%%:*}" "$ROOT/${pair#*:}"
done

log "Building services..."
for svc in "${SERVICES[@]}"; do
  ( cd "$BACKEND/$svc" && go generate ./... >/dev/null 2>&1 || true; go build -o "$BIN/$svc" . ) || die "build $svc failed"
done

# Config: gateway.ini gives the gateway a fixed non-privileged port +
# runtime path. Other services read their .conf via -c.
cat > "$CONF/gateway.ini" <<EOF
[common]
RuntimePath=$RUNTIME

[gateway]
Port=$GW_PORT
EOF
for svc in message-bus user-service local-storage; do
  cat > "$CONF/$svc.conf" <<EOF
[common]
RuntimePath=$RUNTIME

[app]
LogPath=$LOGS
LogSaveName=$svc
LogFileExt=log
DBPath=$DATA/db
UserDataPath=$DATA
EOF
done
cat > "$CONF/core.conf" <<EOF
[app]
RuntimeRootPath=$RUNTIME/
LogPath=$LOGS/
LogSaveName=core
LogFileExt=log
DBPath=$DATA
UserDataPath=$DATA/conf

[server]
RunMode=release
EOF
cat > "$CONF/app-management.conf" <<EOF
[common]
RuntimePath=$RUNTIME

[app]
LogPath=$LOGS
LogSaveName=app-management
LogFileExt=log
AppStorePath=$DATA/appstore
AppsPath=$DATA/apps
EOF

# Gateway first — it serves the built UI (-w) and writes management.url
# the other services discover. CASAOS_CONFIG_PATH points at our conf dir.
log "Starting gateway on :$GW_PORT (serving $UI_BUILD)..."
[ -f "$UI_BUILD/index.html" ] || die "UI build missing — run 'npm run build' first"
( cd "$BACKEND/gateway" && CASAOS_CONFIG_PATH="$CONF" exec "$BIN/gateway" -w "$UI_BUILD" ) >> "$LOGS/gateway.log" 2>&1 &
echo $! >> "$RUNTIME/powerlab.pids"

for i in $(seq 1 30); do
  [ -f "$RUNTIME/management.url" ] && break
  sleep 1
done
[ -f "$RUNTIME/management.url" ] || { cat "$LOGS/gateway.log"; die "gateway did not come up"; }

for svc in "${SERVICES[@]}"; do
  [ "$svc" = gateway ] && continue
  log "Starting $svc..."
  ( cd "$BACKEND/$svc" && exec "$BIN/$svc" -c "$CONF/$svc.conf" ) >> "$LOGS/$svc.log" 2>&1 &
  echo $! >> "$RUNTIME/powerlab.pids"
  sleep 1
done

BASE="http://127.0.0.1:$GW_PORT"
log "Waiting for gateway to answer on $BASE ..."
for i in $(seq 1 30); do
  curl -fsS -o /dev/null "$BASE/" && break
  sleep 1
done
curl -fsS -o /dev/null "$BASE/" || { cat "$LOGS/gateway.log"; die "gateway not answering on $BASE"; }

# First-run provisioning. The register endpoint requires a one-time `key`
# issued by GET /v1/users/status when the instance is uninitialized
# (no users yet). So: fetch the key, then register {username,password,key}.
log "Fetching first-run registration key from /v1/users/status..."
key=""
for i in $(seq 1 20); do
  resp=$(curl -s "$BASE/v1/users/status" || true)
  key=$(echo "$resp" | jq -r '.data.key // empty' 2>/dev/null)
  init=$(echo "$resp" | jq -r '.data.initialized // empty' 2>/dev/null)
  log "  status attempt $i: initialized=$init key=${key:0:8}"
  [ -n "$key" ] && break
  [ "$init" = "true" ] && { log "  already initialized (ok)."; break; }
  sleep 2
done

if [ -n "$key" ]; then
  log "Registering admin '$E2E_USER' with key..."
  rc=$(curl -s -o /tmp/reg.out -w "%{http_code}" -X POST "$BASE/v1/users/register" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"$E2E_USER\",\"password\":\"$E2E_PASS\",\"key\":\"$key\"}" || echo "000")
  log "  register: HTTP $rc $(head -c 160 /tmp/reg.out)"
fi

# Authoritative gate: a real login must work, or the stack is unusable
# for the @smoke specs. Fail loudly with the body rather than hand
# Playwright a broken backend (which only shows up as cascading 400/429).
log "Sanity login as '$E2E_USER'..."
lcode=$(curl -s -o /tmp/login.out -w "%{http_code}" -X POST "$BASE/v1/users/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$E2E_USER\",\"password\":\"$E2E_PASS\"}" || echo "000")
log "  login: HTTP $lcode $(head -c 200 /tmp/login.out 2>/dev/null)"
[[ "$lcode" == 2* ]] || { echo "--- user-service log ---"; tail -n 60 "$LOGS/user-service.log" 2>/dev/null; die "sanity login failed (HTTP $lcode) — admin not provisioned"; }

log "Stack up. Base: $BASE"
echo "POWERLAB_E2E_BASE=$BASE"
