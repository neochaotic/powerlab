#!/usr/bin/env bash
# PowerLab — Linux end-to-end smoke test
#
# Runs the full install + login + feature exercise inside a privileged
# Ubuntu 22.04 Docker container with avahi + dockerd. Bails on first
# failure. Wired into validate.sh --full so a release cannot be tagged
# without this script passing.
#
# Three scenarios exercise the full real-world topology:
#
#   A) Clean host: install.sh runs cleanly, all 6 services come up
#      cold with zero restarts, gateway HTTP 200, then exercise:
#         · PAM login (POST /v1/users/login → JWT)
#         · editor write (PUT /v1/file with file_path/file_content)
#         · apps list (GET /v2/app_management/compose)
#         · terminal websocket (ws://…/v1/sys/wsshell?token=<jwt>)
#         · file upload (POST /v1/file/upload, multipart)
#
#   B) Host with CasaOS already installed, no flag — install.sh must
#      detect the conflict and exit 1 with a clear refusal message.
#
#   C) Host with CasaOS already installed + --allow-coexist — install.sh
#      proceeds, banner mentions ports, all 6 services come up.
#
# Usage:
#   ./scripts/test-linux-e2e.sh                  # builds a fresh tarball
#   ./scripts/test-linux-e2e.sh /path/to.tar.gz  # reuses a tarball
#
# Exit code: 0 on full pass, 1 on first failure.
set -euo pipefail

REPO="$(cd "$(dirname "$0")/.." && pwd)"
TARBALL="${1:-}"

red()   { printf '\033[0;31m%s\033[0m\n' "$*"; }
green() { printf '\033[0;32m%s\033[0m\n' "$*"; }
cyan()  { printf '\033[0;36m%s\033[0m\n' "$*"; }
fail()  { red "FAIL: $*"; exit 1; }

cyan "[e2e] PowerLab Linux end-to-end smoke test"

if ! command -v docker >/dev/null; then
  fail "docker is not installed on this host"
fi

# ─── 1. Build a tarball if the caller did not pass one ───────────────────
if [[ -z "$TARBALL" ]]; then
  cyan "[e2e] no tarball provided — building a fresh one for amd64..."
  ( cd "$REPO" && POWERLAB_SKIP_FRONTEND_BUILD="${POWERLAB_SKIP_FRONTEND_BUILD:-1}" \
    ./scripts/package-linux.sh amd64 0.0.0-e2e >/tmp/e2e-pkg.log 2>&1 ) || {
    tail -30 /tmp/e2e-pkg.log
    fail "package-linux.sh failed"
  }
  TARBALL="$REPO/dist/powerlab-0.0.0-e2e-linux-amd64.tar.gz"
fi
[[ -f "$TARBALL" ]] || fail "tarball not found: $TARBALL"
green "[e2e] using $TARBALL ($(du -h "$TARBALL" | awk '{print $1}'))"

# ─── 2. Helpers ───────────────────────────────────────────────────────────
NAME="pwl-e2e-$$"

cleanup() { docker rm -f "$NAME" >/dev/null 2>&1 || true; }
trap cleanup EXIT

run_in_container() {
  docker exec "$NAME" bash -c "$1"
}

start_container() {
  cleanup
  docker run -d --name "$NAME" --privileged --platform linux/amd64 \
    --tmpfs /tmp --tmpfs /run --tmpfs /run/lock \
    -v /sys/fs/cgroup:/sys/fs/cgroup:rw \
    jrei/systemd-ubuntu:22.04 >/dev/null
  sleep 3
  run_in_container '
    apt-get update -qq >/dev/null
    DEBIAN_FRONTEND=noninteractive apt-get install -yqq \
      curl ca-certificates avahi-daemon avahi-utils libnss-mdns docker.io python3-pip >/dev/null 2>&1
    systemctl enable --now avahi-daemon docker >/dev/null 2>&1
    pip3 install -q websocket-client 2>/dev/null
    sleep 2
  '
  docker cp "$TARBALL" "$NAME:/root/p.tar.gz"
  run_in_container 'mkdir -p /tmp/x && tar xzf /root/p.tar.gz -C /tmp/x --strip-components=1'
}

assert_all_active_no_restart() {
  local out
  out=$(run_in_container '
    sleep 8
    fail=0
    for s in gateway message-bus user-service local-storage app-management core; do
      state=$(systemctl is-active powerlab-$s)
      restarts=$(systemctl show powerlab-$s -p NRestarts --value)
      if [[ "$state" != "active" ]] || (( restarts > 0 )); then
        echo "  · powerlab-$s: state=$state NRestarts=$restarts"
        fail=1
      fi
    done
    exit $fail
  ') && {
    green "  → all 6 services active, 0 restarts"
  } || {
    red   "  → service health failed:"
    echo "$out"
    return 1
  }
}

# ─── Scenario A: clean install ───────────────────────────────────────────
cyan "[e2e] Scenario A: clean install"
start_container
run_in_container 'bash /tmp/x/install.sh > /tmp/install.log 2>&1' || {
  run_in_container 'tail -30 /tmp/install.log'
  fail "install.sh exited non-zero on a clean host"
}
assert_all_active_no_restart || fail "scenario A: services unhealthy after install"

# Stamped UI version must match the backend version that install.sh
# wrote to /etc/powerlab/version. Catches the v0.2.5-first-attempt
# bug where CI cached a stale ui/build with 0.2.0 baked into the JS
# bundle even though the backend was 0.2.5.
EXPECTED_UI_VERSION=$(run_in_container "awk -F'\"' '/VERSION/ {print \$2}' /etc/powerlab/version 2>/dev/null")
[[ -n "$EXPECTED_UI_VERSION" ]] || fail "scenario A: /etc/powerlab/version empty/missing"
SEEN_VERSIONS=$(run_in_container "grep -roh '0\\.[0-9]\\+\\.[0-9]\\+' /usr/share/powerlab/www/_app/ 2>/dev/null | sort -u" || true)
echo "$SEEN_VERSIONS" | grep -qx "$EXPECTED_UI_VERSION" \
  || fail "scenario A: UI bundle version mismatch — backend says $EXPECTED_UI_VERSION, bundle has $(echo $SEEN_VERSIONS | tr -s ' ')"
green "  → UI bundle stamped $EXPECTED_UI_VERSION (matches backend)"

# Version handshake endpoint must report the same version the backend
# binary was linked with — otherwise the UI's banner-on-mismatch logic
# would never know the right answer to compare against. Catches a
# regression where the -ldflags="-X common.POWERLAB_VERSION=..." path
# silently breaks (wrong import path, etc).
HANDSHAKE_VERSION=$(run_in_container "curl -fsS http://localhost:8765/v1/powerlab/version | python3 -c 'import sys,json; print(json.load(sys.stdin)[\"version\"])'")
[[ "$HANDSHAKE_VERSION" == "$EXPECTED_UI_VERSION" ]] \
  || fail "scenario A: /v1/powerlab/version returned $HANDSHAKE_VERSION, expected $EXPECTED_UI_VERSION"
green "  → /v1/powerlab/version handshake returns $HANDSHAKE_VERSION"

# Smoke: login → editor → apps → terminal → upload
run_in_container '
  set -e
  useradd -m -s /bin/bash testuser 2>/dev/null
  echo "testuser:testpass" | chpasswd
' >/dev/null

TOKEN=$(run_in_container '
  curl -sS -X POST http://localhost:8765/v1/users/login \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"testuser\",\"password\":\"testpass\"}" \
    | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get(\"data\",{}).get(\"token\",{}).get(\"access_token\",\"\"))"
')
[[ -n "$TOKEN" ]] || fail "scenario A: PAM login returned no token"
green "  → login OK (token ${#TOKEN} chars)"

# Files page default path. The user logged in via PAM as `testuser`
# (real Linux account with /home/testuser) — Files should land
# there under PowerLab/ rather than at /DATA or filesystem root.
HOME_JSON=$(run_in_container "curl -fsS http://localhost:8765/v1/file/home -H 'Authorization: $TOKEN'")
HOME_PATH=$(echo "$HOME_JSON" | python3 -c 'import sys,json; print(json.load(sys.stdin)["data"]["path"])')
HOME_SOURCE=$(echo "$HOME_JSON" | python3 -c 'import sys,json; print(json.load(sys.stdin)["data"]["source"])')
[[ "$HOME_PATH" == "/home/testuser/PowerLab" ]] && [[ "$HOME_SOURCE" == "os-home" ]] \
  || fail "scenario A: /v1/file/home expected /home/testuser/PowerLab (os-home), got $HOME_PATH ($HOME_SOURCE)"
green "  → /v1/file/home → $HOME_PATH (os-home, mkdir-p'd)"

run_in_container "
  echo original > /tmp/smoke-edit.txt
  RESP=\$(curl -sS -X PUT http://localhost:8765/v1/file -H 'Authorization: $TOKEN' \
    -H 'Content-Type: application/json' \
    -d '{\"file_path\":\"/tmp/smoke-edit.txt\",\"file_content\":\"updated\"}')
  case \"\$RESP\" in *success*200*) echo OK ;; *) echo BAD: \$RESP; exit 1 ;; esac
  [[ \"\$(cat /tmp/smoke-edit.txt)\" == updated ]] || { echo file-not-updated; exit 1; }
" >/dev/null || fail "scenario A: editor PUT /v1/file did not update existing file"
green "  → editor PUT (existing file) → 200 OK"

# Filebrowser-style POST=create / PUT=update split:
#   POST /v1/file (file does not exist) → 200 created
#   POST /v1/file (file exists)         → 409 Conflict
#   POST /v1/file?override=true         → 200 overwrites
#   PUT  /v1/file (file does not exist) → 404 (must use POST)
CODE=$(run_in_container "
  rm -f /tmp/smoke-new.txt
  curl -sS -o /dev/null -w '%{http_code}' -X POST http://localhost:8765/v1/file \
    -H 'Authorization: $TOKEN' -H 'Content-Type: application/json' \
    -d '{\"file_path\":\"/tmp/smoke-new.txt\",\"file_content\":\"fresh\"}'
")
[[ "$CODE" == "200" ]] || fail "scenario A: POST /v1/file (new) must return 200, got $CODE"
green "  → editor POST (new file) → 200 created"

CODE=$(run_in_container "
  curl -sS -o /dev/null -w '%{http_code}' -X POST http://localhost:8765/v1/file \
    -H 'Authorization: $TOKEN' -H 'Content-Type: application/json' \
    -d '{\"file_path\":\"/tmp/smoke-new.txt\",\"file_content\":\"again\"}'
")
[[ "$CODE" == "409" ]] || fail "scenario A: POST /v1/file (already exists) must return 409, got $CODE"
green "  → editor POST (existing file, no override) → 409 Conflict"

CODE=$(run_in_container "
  curl -sS -o /dev/null -w '%{http_code}' -X POST 'http://localhost:8765/v1/file?override=true' \
    -H 'Authorization: $TOKEN' -H 'Content-Type: application/json' \
    -d '{\"file_path\":\"/tmp/smoke-new.txt\",\"file_content\":\"forced\"}'
")
[[ "$CODE" == "200" ]] || fail "scenario A: POST ?override=true must return 200, got $CODE"
green "  → editor POST (override=true) → 200 OK"

CODE=$(run_in_container "
  rm -f /tmp/smoke-put-missing.txt
  curl -sS -o /dev/null -w '%{http_code}' -X PUT http://localhost:8765/v1/file \
    -H 'Authorization: $TOKEN' -H 'Content-Type: application/json' \
    -d '{\"file_path\":\"/tmp/smoke-put-missing.txt\",\"file_content\":\"x\"}'
")
[[ "$CODE" == "404" ]] || fail "scenario A: PUT /v1/file (missing) must return 404 — caller should POST instead, got $CODE"
green "  → editor PUT (missing file) → 404 (forces POST for new files)"

# Legacy "+ New File" button still works (sends bare {path})
CODE=$(run_in_container "
  rm -f /tmp/smoke-empty.txt
  curl -sS -o /dev/null -w '%{http_code}' -X POST http://localhost:8765/v1/file \
    -H 'Authorization: $TOKEN' -H 'Content-Type: application/json' \
    -d '{\"path\":\"/tmp/smoke-empty.txt\"}'
")
[[ "$CODE" == "200" ]] && [[ "$(run_in_container 'wc -c < /tmp/smoke-empty.txt')" -eq 0 ]] \
  || fail "scenario A: legacy POST {path} did not create empty file (got $CODE)"
green "  → legacy + New File button (POST {path}) → 200 (empty file)"

run_in_container "
  RESP=\$(curl -sS http://localhost:8765/v2/app_management/compose -H 'Authorization: $TOKEN')
  case \"\$RESP\" in '{'*) ;; *) echo \"BAD: \$RESP\"; exit 1 ;; esac
" >/dev/null || fail "scenario A: GET /v2/app_management/compose did not return JSON"
green "  → apps list OK"

run_in_container "
  python3 - <<PY
import websocket, time, sys
ws = websocket.create_connection('ws://localhost:8765/v1/sys/wsshell?cols=80&rows=24&token=$TOKEN', timeout=5)
time.sleep(0.5)
ws.send('echo PWLAB-SMOKE\n')
out = b''
deadline = time.time() + 3
while time.time() < deadline:
  try:
    ws.settimeout(1.0)
    chunk = ws.recv()
    out += chunk if isinstance(chunk, bytes) else chunk.encode()
    if b'PWLAB-SMOKE' in out:
      sys.exit(0)
  except Exception:
    break
sys.exit(1)
PY
" >/dev/null 2>&1 || fail "scenario A: terminal websocket did not echo back"
green "  → terminal websocket OK"

run_in_container "
  mkdir -p /tmp/smoke-upload
  echo upload-content > /tmp/source.txt
  RESP=\$(curl -sS -X POST http://localhost:8765/v1/file/upload \
    -H 'Authorization: $TOKEN' \
    -F file=@/tmp/source.txt \
    -F filename=u.txt \
    -F relativePath=u.txt \
    -F totalChunks=1 \
    -F chunkNumber=0 \
    -F path=/tmp/smoke-upload)
  case \"\$RESP\" in *success*200*) ;; *) echo \"BAD: \$RESP\"; exit 1 ;; esac
  grep -q upload-content /tmp/smoke-upload/u.txt || { echo content-mismatch; exit 1; }
" >/dev/null || fail "scenario A: file upload did not land"
green "  → upload OK"

CODE=$(run_in_container "
  curl -sS -o /dev/null -w '%{http_code}' -X POST http://localhost:8765/v1/file/upload \
    -H 'Authorization: $TOKEN' -F filename=g.txt -F relativePath=g.txt \
    -F totalChunks=1 -F chunkNumber=0 -F path=/tmp/smoke-upload
")
[[ "$CODE" == "400" ]] || fail "scenario A: upload missing-file should return 400, got $CODE"
green "  → upload missing-file rejected with 400"

# Upload to a destination directory that doesn't exist yet — the Files
# page can navigate to a folder that lives in the production layout
# (/DATA) but is absent on the dev machine. Old behaviour: 500 "no
# such file or directory". Expected behaviour: auto-create the parent
# (mkdir -p) and accept the upload, like every Unix file manager.
run_in_container "
  set -e
  rm -rf /tmp/freshly-made-dir
  echo upload-into-fresh-dir > /tmp/source-fresh.txt
  RESP=\$(curl -sS -X POST http://localhost:8765/v1/file/upload \
    -H 'Authorization: $TOKEN' \
    -F file=@/tmp/source-fresh.txt -F filename=u.txt -F relativePath=u.txt \
    -F totalChunks=1 -F chunkNumber=0 -F path=/tmp/freshly-made-dir)
  case \"\$RESP\" in *success*200*) ;; *) echo BAD: \$RESP; exit 1 ;; esac
  grep -q upload-into-fresh-dir /tmp/freshly-made-dir/u.txt || { echo content-mismatch; exit 1; }
" >/dev/null || fail "scenario A: upload to non-existent parent should auto-mkdir, not 500"
green "  → upload auto-creates parent dir (no more 500 on /DATA)"

# Read a file that does not exist — used to be 500 (looked like a
# backend crash), now 404 (proper "not found" semantics so the UI
# can offer to create the file instead of just failing silently).
CODE=$(run_in_container "
  curl -sS -o /dev/null -w '%{http_code}' \
    -H 'Authorization: $TOKEN' \
    'http://localhost:8765/v1/file/content?path=/no/such/file.txt'
")
[[ "$CODE" == "404" ]] || fail "scenario A: read of missing file must be 404, got $CODE"
green "  → read missing file returns 404 (not 500)"

# Download flow used by both <a href> and <video src> / <audio src>
# in the Files preview pane. Three things must hold or the panel
# breaks under any non-localhost client:
#   · GET /v1/file?path= returns the file body (200 + correct bytes)
#   · ?token=… authenticates (so EventSource-equivalent <video src> works)
#   · Range: bytes=N-M returns 206 Partial Content (so video seeking works)
run_in_container "
  set -e
  echo download-content > /tmp/smoke-dl.txt
  body=\$(curl -fsS 'http://localhost:8765/v1/file?path=%2Ftmp%2Fsmoke-dl.txt&token=$TOKEN')
  [[ \"\$body\" == 'download-content' ]] || { echo BAD-BODY: \$body; exit 1; }
  range_code=\$(curl -sS -o /dev/null -w '%{http_code}' -H 'Range: bytes=0-3' 'http://localhost:8765/v1/file?path=%2Ftmp%2Fsmoke-dl.txt&token=$TOKEN')
  [[ \"\$range_code\" == '206' ]] || { echo BAD-RANGE: \$range_code; exit 1; }
" >/dev/null || fail "scenario A: download (200) + Range (206) flow broken"
green "  → download OK + Range request returns 206 (video seeking works)"

# Catch-all for the smaller endpoints whose silent failure (404 / 400)
# pollutes every page render in the browser console without anyone
# noticing because the UI swallows the rejection. If any of these
# stops responding 200, fail the release.
for path_label in \
    "/v1/sys/hardware:hardware-info" \
    "/v2/app_management/config:app-management-config" \
    "/v2/app_management/compose/test-helloworld/disk-usage:disk-usage(404 acceptable)" \
    "/v1/powerlab/version:version-handshake" ; do
  PATH_ONLY="${path_label%%:*}"
  LABEL="${path_label##*:}"
  CODE=$(run_in_container "curl -sS -o /dev/null -w '%{http_code}' -H 'Authorization: $TOKEN' http://localhost:8765$PATH_ONLY")
  case "$LABEL:$CODE" in
    "disk-usage(404 acceptable):200"|"disk-usage(404 acceptable):404")
      green "  → $LABEL OK ($CODE)"
      ;;
    *":200")
      green "  → $LABEL OK"
      ;;
    *)
      fail "scenario A: $LABEL ($PATH_ONLY) returned $CODE — must be 200"
      ;;
  esac
done

# ─── Scenario B: CasaOS present, no flag → must refuse ───────────────────
cyan "[e2e] Scenario B: CasaOS present (no --allow-coexist)"
start_container
run_in_container '
  cat > /etc/systemd/system/casaos.service <<EOF
[Unit]
Description=CasaOS Main Service
[Service]
ExecStart=/bin/sleep infinity
[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable casaos >/dev/null 2>&1
'
if run_in_container 'bash /tmp/x/install.sh > /tmp/install.log 2>&1'; then
  fail "scenario B: install.sh should have refused, got exit 0"
fi
run_in_container 'grep -q "Refusing to install" /tmp/install.log' \
  || fail "scenario B: install.sh did not print refusal message"
green "  → install.sh correctly refused with diagnostic"

# ─── Scenario C: CasaOS + --allow-coexist → succeeds with banner ─────────
cyan "[e2e] Scenario C: CasaOS present + --allow-coexist"
run_in_container 'bash /tmp/x/install.sh --allow-coexist > /tmp/install.log 2>&1' \
  || fail "scenario C: install.sh --allow-coexist should have succeeded"
run_in_container 'grep -q "You passed --allow-coexist" /tmp/install.log' \
  || fail "scenario C: banner did not confirm --allow-coexist"
assert_all_active_no_restart || fail "scenario C: services unhealthy after --allow-coexist install"

green ""
green "╔═══════════════════════════════════════════════════╗"
green "║  Linux E2E PASSED — release gate cleared.         ║"
green "║  Scenarios A (clean), B (refuse), C (coexist) OK. ║"
green "╚═══════════════════════════════════════════════════╝"
