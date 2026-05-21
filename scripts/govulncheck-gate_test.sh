#!/usr/bin/env bash
# Unit test for scripts/govulncheck-gate.sh.
#
# Drives the gate with a FAKE govulncheck (canned NDJSON) and an overridable
# allowlist (GOVULNCHECK_ALLOWLIST), so the gate's decision logic — reachable
# vs informational, stdlib-ignored, allowlist suppression, fail-closed — is
# regression-locked without needing a real scan, network, or build.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GATE="$REPO_ROOT/scripts/govulncheck-gate.sh"

tmp="$(mktemp -d)"
trap 'rm -f "${fakebin:-}/govulncheck" 2>/dev/null; rm -rf "$tmp"' EXIT
fakebin="$tmp/bin"
mkdir -p "$fakebin"

pass=0
fail=0
check() { # check <desc> <expected-exit> <actual-exit>
  if [[ "$2" == "$3" ]]; then echo "  ok: $1"; pass=$((pass+1));
  else echo "  FAIL: $1 (expected exit $2, got $3)"; fail=$((fail+1)); fi
}

# Writes a fake govulncheck that prints $1 (a file of NDJSON) and exits $2.
make_fake() { # make_fake <ndjson-file> <exit-code>
  cat >"$fakebin/govulncheck" <<EOF
#!/usr/bin/env bash
cat "$1"
exit $2
EOF
  chmod +x "$fakebin/govulncheck"
}

run_gate() { # run_gate <allowlist-file> -> sets RC
  set +e
  GOVULNCHECK_ALLOWLIST="$1" PATH="$fakebin:$PATH" \
    bash "$GATE" >/dev/null 2>&1
  RC=$?
  set -e
}

# Canned scanner output: one reachable non-stdlib vuln, one reachable stdlib
# vuln (must be ignored), one imported-but-not-called vuln (must be ignored).
cat >"$tmp/findings.ndjson" <<'EOF'
{"config":{"protocol_version":"v1.0.0"}}
{"finding":{"osv":"GO-TEST-0001","trace":[{"module":"github.com/evil/dep","function":"Boom"}]}}
{"finding":{"osv":"GO-TEST-STDLIB","trace":[{"module":"stdlib","function":"Parse"}]}}
{"finding":{"osv":"GO-TEST-NOCALL","trace":[{"module":"github.com/x/y"}]}}
EOF

# Move into the repo so the gate's `git rev-parse` succeeds.
cd "$REPO_ROOT"

echo "govulncheck-gate_test:"

# 1. Reachable non-stdlib vuln IS allowlisted -> pass.
make_fake "$tmp/findings.ndjson" 0
echo "GO-TEST-0001  # test" >"$tmp/allow_yes"
run_gate "$tmp/allow_yes"
check "allowlisted reachable vuln -> pass" 0 "$RC"

# 2. Same vuln NOT allowlisted -> block.
: >"$tmp/allow_empty"
run_gate "$tmp/allow_empty"
check "un-allowlisted reachable vuln -> block" 1 "$RC"

# 3. Only stdlib + not-called findings -> pass (neither counts).
cat >"$tmp/findings_safe.ndjson" <<'EOF'
{"config":{"protocol_version":"v1.0.0"}}
{"finding":{"osv":"GO-TEST-STDLIB","trace":[{"module":"stdlib","function":"Parse"}]}}
{"finding":{"osv":"GO-TEST-NOCALL","trace":[{"module":"github.com/x/y"}]}}
EOF
make_fake "$tmp/findings_safe.ndjson" 0
run_gate "$tmp/allow_empty"
check "stdlib + not-called only -> pass" 0 "$RC"

# 4. Scanner exits non-zero (build error) -> fail closed.
make_fake "$tmp/findings.ndjson" 2
run_gate "$tmp/allow_yes"
check "scanner error -> fail closed" 1 "$RC"

# 5. Scanner emits nothing (no config object) -> fail closed.
: >"$tmp/empty.ndjson"
make_fake "$tmp/empty.ndjson" 0
run_gate "$tmp/allow_yes"
check "scanner produces no output -> fail closed" 1 "$RC"

echo "── $pass passed, $fail failed ──"
[[ "$fail" -eq 0 ]]
