#!/usr/bin/env bash
# Tests for scripts/check-catalog-app-safety.sh — the ADR-0039 compose-
# level security gate. Each fixture is a minimal docker-compose.yml
# exercising one rejection class. The test asserts the script:
#   - flags the bad fixture (exit 1) with the expected reason substring
#   - passes the clean fixture (exit 0)
#
# Run: ./scripts/check-catalog-app-safety_test.sh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO_ROOT/scripts/check-catalog-app-safety.sh"
FIXTURES_DIR="$(mktemp -d)"
trap 'rm -rf "$FIXTURES_DIR"' EXIT

fail_count=0
pass_count=0

# Write a fixture compose file and assert the script's behavior on it.
#   $1 = test name (human-readable)
#   $2 = expected exit code (0=pass, 1=fail)
#   $3 = expected substring in output when failing (empty for clean fixtures)
#   stdin = compose YAML body
assert_safety() {
  local name="$1" want_exit="$2" want_substr="$3"
  local fixture_dir
  fixture_dir="$(mktemp -d -p "$FIXTURES_DIR")"
  cat > "$fixture_dir/docker-compose.yml"

  local out exit_code=0
  # Tests assume strict mode — they're testing the rejection rules,
  # not the warn-only behavior. The CI integration uses warn-only
  # during Sprint 22 ship phase (see script header).
  out="$(POWERLAB_CATALOG_SAFETY_STRICT=1 "$SCRIPT" "$fixture_dir/docker-compose.yml" 2>&1)" || exit_code=$?

  if [[ "$exit_code" -ne "$want_exit" ]]; then
    echo "FAIL: $name — exit $exit_code, want $want_exit"
    echo "  output: $out"
    fail_count=$((fail_count + 1))
    return
  fi
  if [[ -n "$want_substr" ]] && ! echo "$out" | grep -q "$want_substr"; then
    echo "FAIL: $name — output missing expected substring '$want_substr'"
    echo "  output: $out"
    fail_count=$((fail_count + 1))
    return
  fi
  echo "OK: $name"
  pass_count=$((pass_count + 1))
}

# ──────────────────────────────────────────────────────────────────
# Clean fixture — minimum viable compose that should PASS.
# ──────────────────────────────────────────────────────────────────
assert_safety "clean nginx" 0 "" <<'EOF'
name: nginx
services:
  web:
    image: nginx:alpine
    ports:
      - 8080:80
EOF

# ──────────────────────────────────────────────────────────────────
# Rejection classes — each should FAIL with a recognisable reason.
# ──────────────────────────────────────────────────────────────────

assert_safety "privileged: true rejected" 1 "privileged" <<'EOF'
name: bad
services:
  web:
    image: nginx
    privileged: true
EOF

assert_safety "docker.sock bind rejected (short form)" 1 "docker.sock" <<'EOF'
name: bad
services:
  web:
    image: nginx
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
EOF

assert_safety "docker.sock bind rejected (read-only suffix)" 1 "docker.sock" <<'EOF'
name: bad
services:
  web:
    image: nginx
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
EOF

assert_safety "network_mode: host rejected" 1 "network_mode" <<'EOF'
name: bad
services:
  web:
    image: nginx
    network_mode: host
EOF

assert_safety "pid: host rejected" 1 "pid" <<'EOF'
name: bad
services:
  web:
    image: nginx
    pid: host
EOF

assert_safety "ipc: host rejected" 1 "ipc" <<'EOF'
name: bad
services:
  web:
    image: nginx
    ipc: host
EOF

assert_safety "cap_add: SYS_ADMIN rejected" 1 "SYS_ADMIN" <<'EOF'
name: bad
services:
  web:
    image: nginx
    cap_add:
      - SYS_ADMIN
EOF

assert_safety "cap_add: ALL rejected" 1 "ALL" <<'EOF'
name: bad
services:
  web:
    image: nginx
    cap_add:
      - ALL
EOF

# ──────────────────────────────────────────────────────────────────
# Bind-mount safety — paths inside the app's data dir are fine;
# binds to system paths are not.
# ──────────────────────────────────────────────────────────────────

assert_safety "/etc bind-mount rejected" 1 "/etc" <<'EOF'
name: bad
services:
  web:
    image: nginx
    volumes:
      - /etc/nginx/conf.d:/etc/nginx/conf.d
EOF

# ──────────────────────────────────────────────────────────────────
# Allow cases that LOOK suspicious but are fine.
# ──────────────────────────────────────────────────────────────────

assert_safety "non-host network is fine" 0 "" <<'EOF'
name: ok
services:
  web:
    image: nginx
    networks:
      default:
        ipv4_address: 172.16.0.2
EOF

assert_safety "cap_drop: ALL is fine (defensive, not privileged)" 0 "" <<'EOF'
name: ok
services:
  web:
    image: nginx
    cap_drop:
      - ALL
EOF

assert_safety "/DATA bind-mount is fine (app data dir)" 0 "" <<'EOF'
name: ok
services:
  web:
    image: nginx
    volumes:
      - /DATA/PowerLabAppData/myapp/data:/usr/share/nginx/html
EOF

# ──────────────────────────────────────────────────────────────────
# Summary
# ──────────────────────────────────────────────────────────────────

echo ""
echo "================================="
echo "PASSED: $pass_count"
echo "FAILED: $fail_count"
echo "================================="

if [[ "$fail_count" -ne 0 ]]; then
  exit 1
fi

echo "ALL TESTS PASSED"
