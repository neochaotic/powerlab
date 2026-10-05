#!/usr/bin/env bash
# Offline test for scripts/check-dep-health.sh (#493). A stub `go` on PATH
# serves a fixed `go list -m -u -json all` stream and next-major probes,
# so the parsing and flag rules are checked without network access.
set -euo pipefail
cd "$(dirname "$0")/.."

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

mkdir -p "$WORK/bin" "$WORK/modok" "$WORK/modbroken"
echo "module example.com/modok" >"$WORK/modok/go.mod"
echo "module example.com/modbroken" >"$WORK/modbroken/go.mod"

cat >"$WORK/list.json" <<'EOF'
{"Path":"example.com/modok","Main":true,"GoVersion":"1.25"}
{"Path":"example.com/stale","Version":"v1.2.0","Time":"2022-01-01T00:00:00Z"}
{"Path":"example.com/minorbump","Version":"v1.2.0","Time":"2026-06-01T00:00:00.123Z","Update":{"Path":"example.com/minorbump","Version":"v1.4.1","Time":"2026-09-01T00:00:00Z"}}
{"Path":"example.com/patchonly","Version":"v1.2.0","Time":"2026-06-01T00:00:00Z","Update":{"Path":"example.com/patchonly","Version":"v1.2.9","Time":"2026-09-01T00:00:00Z"}}
{"Path":"example.com/zerominor","Version":"v0.3.0","Time":"2026-06-01T00:00:00Z","Update":{"Path":"example.com/zerominor","Version":"v0.4.0","Time":"2026-09-01T00:00:00Z"}}
{"Path":"example.com/hasmajor/v2","Version":"v2.1.0","Time":"2026-06-01T00:00:00Z"}
{"Path":"example.com/oldindirect","Version":"v0.1.0","Time":"2019-01-01T00:00:00Z","Indirect":true}
{"Path":"example.com/localrepl","Version":"v0.0.0-00010101000000-000000000000","Replace":{"Path":"../common","Dir":"/x/common"}}
{"Path":"gopkg.in/yaml.v3","Version":"v3.0.1","Time":"2026-06-01T00:00:00Z"}
EOF

cat >"$WORK/bin/go" <<EOF
#!/usr/bin/env bash
# stub: go list -m -u -json all | go list -m -json <path>@latest
case "\$(basename "\$PWD")" in modbroken) [[ "\$*" == *" all" ]] && { echo "boom" >&2; exit 1; } ;; esac
if [[ "\$*" == "list -m -u -json all" ]]; then cat "$WORK/list.json"; exit 0; fi
if [[ "\$*" == "list -m -json example.com/hasmajor/v3@latest" ]]; then
  echo '{"Path":"example.com/hasmajor/v3","Version":"v3.0.2"}'; exit 0
fi
if [[ "\$*" == *"gopkg.in"* ]]; then echo "unexpected gopkg.in probe" >"$WORK/gopkg-probed"; fi
exit 1
EOF
chmod +x "$WORK/bin/go"

# 2026-10-01T00:00:00Z
out="$(PATH="$WORK/bin:$PATH" DEP_HEALTH_NOW=1790812800 GITHUB_STEP_SUMMARY="$WORK/summary.md" \
  bash scripts/check-dep-health.sh "$WORK/modok" "$WORK/modbroken")" \
  || { echo "FAIL: script exited non-zero (must be warn-only)"; exit 1; }

failures=0
expect() {  # expect <description> <fixed string>
  if grep -qF -- "$2" <<<"$out"; then echo "  PASS: $1"; else echo "  FAIL: $1 (missing: $2)"; failures=$((failures + 1)); fi
}
reject() {  # reject <description> <fixed string>
  if grep -qF -- "$2" <<<"$out"; then echo "  FAIL: $1 (found: $2)"; failures=$((failures + 1)); else echo "  PASS: $1"; fi
}

expect "stale dep with no update: latest = current, flags exactly stale" \
  '| modok | `example.com/stale` | v1.2.0 | 2022-01-01 | 56 | v1.2.0 | stale |'
expect "minor bump flagged (fractional-second timestamp parsed)" \
  '| modok | `example.com/minorbump` | v1.2.0 | 2026-06-01 | 4 | v1.4.1 | minor |'
expect "v0 minor bump flagged" '`example.com/zerominor` | v0.3.0 | 2026-06-01 | 4 | v0.4.0 | minor |'
expect "next major detected" \
  '`example.com/hasmajor/v2` | v2.1.0 | 2026-06-01 | 4 | v2.1.0 (major: example.com/hasmajor/v3 v3.0.2) | major |'
reject "patch-only update not flagged" 'example.com/patchonly'
reject "indirect dep not flagged" 'example.com/oldindirect'
reject "locally replaced module skipped" 'example.com/localrepl'
reject "main module skipped" '`example.com/modok`'
reject "gopkg.in dep not flagged" 'gopkg.in/yaml.v3'
expect "broken module reported, scan continues" "modbroken: go list failed: boom"
expect "summary counts" "Scanned 1 module(s); 4 direct dependency finding(s)."
if [[ -e "$WORK/gopkg-probed" ]]; then echo "  FAIL: gopkg.in path was probed for a next major"; failures=$((failures + 1)); fi
if ! grep -qF 'example.com/stale' "$WORK/summary.md"; then
  echo "  FAIL: report not written to GITHUB_STEP_SUMMARY"; failures=$((failures + 1))
fi

if (( failures > 0 )); then
  echo "FAIL: $failures check(s) failed"; echo "$out"; exit 1
fi
echo "OK: check-dep-health.sh parsing and flag rules"
