#!/usr/bin/env bash
#
# govulncheck-gate.sh — blocking SAST gate with an allowlist.
#
# Runs govulncheck on the module in the current working directory and fails
# only on REACHABLE (called-symbol) vulnerabilities that are:
#   - not in the Go standard library (those are toolchain-managed; we keep Go
#     current via CI's floating go-version), and
#   - not listed in .govulncheck-allowlist.txt (the tracked, accepted backlog).
#
# Imported-but-not-called ("informational") vulns never fail the gate — only
# called code is real attack surface. The allowlist path is resolved from the
# repo root, so this works from any per-service working directory.
#
# Fails CLOSED: if govulncheck is missing, errors, or emits no parseable
# output, the gate fails rather than passing silently.
#
# Requires: govulncheck (PATH or $GOPATH/bin), jq.
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
# Allowlist path is overridable for testing (see govulncheck-gate_test.sh).
allowlist="${GOVULNCHECK_ALLOWLIST:-${repo_root}/.govulncheck-allowlist.txt}"

# Resolve the scanner: PATH first, then GOPATH/bin (where `go install` puts it).
gv="$(command -v govulncheck || true)"
if [[ -z "$gv" ]]; then
  gp="$(go env GOPATH)"
  if [[ -x "${gp}/bin/govulncheck" ]]; then
    gv="${gp}/bin/govulncheck"
  fi
fi
if [[ -z "$gv" ]]; then
  echo "ERROR: govulncheck not found on PATH or in \$GOPATH/bin." >&2
  exit 1
fi

command -v jq >/dev/null || { echo "ERROR: jq not found." >&2; exit 1; }

json="$(mktemp)"
trap 'rm -f "$json"' EXIT

# With -format json, govulncheck exits 0 and streams NDJSON; a non-zero exit
# means it failed to run (build error, etc.) — fail closed.
if ! "$gv" -format json ./... >"$json" 2>/dev/null; then
  echo "ERROR: govulncheck failed to run in $(pwd)." >&2
  exit 1
fi

# Sanity: govulncheck always emits a leading {"config": ...} object. Its
# absence means we captured nothing useful — fail closed rather than report
# a misleading clean bill of health.
if ! jq -e 'select(.config != null)' "$json" >/dev/null 2>&1; then
  echo "ERROR: govulncheck produced no parseable output — refusing to pass." >&2
  exit 1
fi

# Reachable = a finding whose top trace frame names a called function and is
# not in the stdlib module.
reachable="$(jq -r '
  select(.finding != null)
  | select((.finding.trace[0].function // null) != null)
  | select((.finding.trace[0].module // "") != "stdlib")
  | .finding.osv
' "$json" | sort -u)"

# Allowed ids = first token of each non-comment, non-blank line. The `|| true`
# keeps an empty/all-comment allowlist from tripping set -e (grep -v exits 1
# when nothing matches). [[:space:]] is portable (BSD grep lacks \s).
allowed=""
if [[ -f "$allowlist" ]]; then
  allowed="$(grep -vE '^[[:space:]]*(#|$)' "$allowlist" | awk '{print $1}' | sort -u || true)"
fi

blocking=""
suppressed=""
for id in $reachable; do
  if grep -qxF "$id" <<<"$allowed"; then
    suppressed+="$id"$'\n'
  else
    blocking+="$id"$'\n'
  fi
done

echo "── govulncheck gate ──────────────────────────────────────────"
if [[ -n "$suppressed" ]]; then
  echo "Suppressed (allowlisted) reachable vulns:"
  echo "$suppressed" | sed '/^$/d;s/^/  /'
fi

if [[ -n "$blocking" ]]; then
  echo ""
  echo "BLOCKING — reachable vulns NOT in the allowlist:"
  echo "$blocking" | sed '/^$/d;s/^/  /'
  echo ""
  echo "Fix the vuln, or (if accepted) add it to .govulncheck-allowlist.txt"
  echo "with a justification + tracking issue. See that file's header."
  exit 1
fi

echo "OK — no un-allowlisted reachable vulns."
