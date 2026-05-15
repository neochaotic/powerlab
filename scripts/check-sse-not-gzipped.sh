#!/usr/bin/env bash
# Pre-tag check: SSE endpoints must NOT advertise Content-Encoding: gzip
# when the client sends Accept-Encoding: gzip — gzip buffers chunks
# before emitting, killing the browser EventSource's per-line streaming.
#
# Why this exists:
#   v0.6.13-stg shipped two stacked SSE-buffering bugs (audit middleware
#   missing Flush() forwarding + Echo Gzip applied to text/event-stream).
#   Both passed every existing gate: 524 vitest green, 19/19 Playwright
#   (mocked SSE via page.route()), `task_e2e_test.go` green (bare Echo,
#   no gzip middleware), curl-without-Accept-Encoding flushed live.
#   Only a real browser hit both → install modal "fica travado na tela
#   de progresso". The decisive test was curl with the headers a real
#   browser sends.
#
# Usage:
#   ./scripts/check-sse-not-gzipped.sh <host> <username> <password> <task_id>
#   ./scripts/check-sse-not-gzipped.sh 192.168.18.86 neochaotic <pwd> probe-id
#
# Exit codes:
#   0  — SSE response has no Content-Encoding: gzip → safe
#   1  — SSE response is gzipped → STOP, add Skipper to gzip middleware
#   2  — could not authenticate / contact server

set -euo pipefail

HOST="${1:-}"
USERNAME="${2:-}"
PASSWORD="${3:-}"
TASK_ID="${4:-sse-gate-probe}"

if [[ -z "$HOST" || -z "$USERNAME" || -z "$PASSWORD" ]]; then
  echo "usage: $0 <host> <username> <password> [task_id]" >&2
  exit 2
fi

BASE_URL="http://${HOST}:8765"

TOKEN=$(curl -fsS --data "{\"username\":\"${USERNAME}\",\"password\":\"${PASSWORD}\"}" \
  -H 'Content-Type: application/json' \
  "${BASE_URL}/v1/users/login" \
  | python3 -c 'import sys,json;d=json.loads(sys.stdin.read(),strict=False);print((d.get("data") or {}).get("token",{}).get("access_token") or "")' 2>/dev/null || true)

if [[ -z "$TOKEN" ]]; then
  echo "ERROR: login to ${HOST} failed; cannot verify SSE gate" >&2
  exit 2
fi

TMPHDR=$(mktemp)
TMPBODY=$(mktemp)
trap 'rm -f "$TMPHDR" "$TMPBODY"' EXIT

# Send the headers a real browser EventSource sends. Disable curl's
# automatic decompression so we observe the raw Content-Encoding header
# on the wire — the symptom we're guarding against.
curl --no-buffer -fsS \
  -D "$TMPHDR" -o "$TMPBODY" -m 3 \
  -H "Accept-Encoding: gzip, deflate" \
  "${BASE_URL}/v2/app_management/compose/task/${TASK_ID}/logs?token=${TOKEN}" \
  || true  # SSE keeps the conn open; curl -m exits non-zero on the timeout — that's fine

if grep -qiE '^Content-Encoding:.*gzip' "$TMPHDR"; then
  echo "FAIL: SSE response advertises Content-Encoding: gzip" >&2
  echo "  Browser EventSource will buffer events indefinitely." >&2
  echo "  Add a GzipWithConfig{Skipper} that bypasses text/event-stream paths." >&2
  echo "  Headers received:" >&2
  sed 's/^/    /' "$TMPHDR" >&2
  exit 1
fi

# Body sanity: must not start with gzip magic (0x1f 0x8b) even if header
# was absent (would mean misaligned framing).
MAGIC=$(head -c 2 "$TMPBODY" | xxd -p 2>/dev/null || true)
if [[ "$MAGIC" == "1f8b" ]]; then
  echo "FAIL: SSE body starts with gzip magic 0x1f8b — broken framing" >&2
  exit 1
fi

echo "OK: SSE endpoint not gzipped (Content-Encoding absent, body is plain)"
exit 0
