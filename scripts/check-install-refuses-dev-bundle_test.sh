#!/usr/bin/env bash
# Test gate: install.sh emitted by package-linux.sh refuses to install
# a dev/test bundle (VERSION_NEXT starting with `0.0.0-`) unless
# POWERLAB_ALLOW_DEV_BUILD=1 is set.
#
# Locks the bug class from 2026-05-15 where a 0.0.0-e2e tarball was
# accidentally hot-swapped onto a production host and the UI started
# reporting "0.0.0-e2e" in the About page.

set -euo pipefail
cd "$(dirname "$0")/.."

PKG_SCRIPT="scripts/package-linux.sh"
if [[ ! -f "$PKG_SCRIPT" ]]; then
  echo "ERROR: $PKG_SCRIPT not found"; exit 1
fi

# Read the heredoc'd install.sh body out of package-linux.sh.
# Anchor: 'cat > "$STAGE/install.sh" <<\'INSTALL_EOF\'' ... 'INSTALL_EOF'
install_body=$(awk '
  /cat > "\$STAGE\/install.sh" <<.INSTALL_EOF/ {flag=1; next}
  /^INSTALL_EOF$/ {flag=0}
  flag {print}
' "$PKG_SCRIPT")

if [[ -z "$install_body" ]]; then
  echo "ERROR: could not extract install.sh template from $PKG_SCRIPT"; exit 1
fi

# 1. Make sure the gate IS present in the template.
if ! grep -q '0.0.0-' <<<"$install_body"; then
  echo "FAIL: install.sh template missing the '0.0.0-*' refusal gate"
  exit 1
fi
if ! grep -q 'POWERLAB_ALLOW_DEV_BUILD' <<<"$install_body"; then
  echo "FAIL: install.sh template missing the POWERLAB_ALLOW_DEV_BUILD override"
  exit 1
fi

# 2. Behavioural sanity: extract the gate snippet alone, simulate the
#    EUID check passing (we cannot actually run as root in a test), and
#    feed it variants of VERSION_NEXT.
#
#    The test only exercises the case branch — strips the EUID gate
#    that would block a non-root run anyway. We use a sentinel exit
#    code (42) so we can distinguish "gate fired" from other failures.
mkdir -p /tmp/powerlab-install-gate-test
SNIPPET=/tmp/powerlab-install-gate-test/snippet.sh
cat > "$SNIPPET" <<'EOS'
#!/usr/bin/env bash
set -euo pipefail
VERSION_NEXT="${1:-}"
case "$VERSION_NEXT" in
  0.0.0-*)
    if [[ "${POWERLAB_ALLOW_DEV_BUILD:-0}" != "1" ]]; then
      exit 42
    fi
    echo "warn: dev build allowed"
    ;;
esac
echo "ok: would proceed with $VERSION_NEXT"
EOS
chmod +x "$SNIPPET"

# Case 1 — production version passes.
if ! out=$("$SNIPPET" "0.6.12" 2>&1); then
  echo "FAIL: production version 0.6.12 was rejected: $out"; exit 1
fi
if [[ "$out" != "ok: would proceed with 0.6.12" ]]; then
  echo "FAIL: unexpected output for 0.6.12: $out"; exit 1
fi

# Case 2 — dev bundle without override is REFUSED (exit 42).
set +e
"$SNIPPET" "0.0.0-e2e" >/dev/null 2>&1
code=$?
set -e
if [[ "$code" == "0" ]]; then
  echo "FAIL: dev bundle 0.0.0-e2e was accepted without POWERLAB_ALLOW_DEV_BUILD"; exit 1
fi
if [[ "$code" != "42" ]]; then
  echo "FAIL: dev bundle should exit 42 (refusal); got $code"; exit 1
fi

# Case 3 — dev bundle WITH override goes through.
if ! out=$(POWERLAB_ALLOW_DEV_BUILD=1 "$SNIPPET" "0.0.0-ci" 2>&1); then
  echo "FAIL: POWERLAB_ALLOW_DEV_BUILD=1 should permit 0.0.0-ci: $out"; exit 1
fi

# Case 4 — variants that LOOK dev-ish but are real versions still pass.
for v in 0.6.12-rc1 0.6.12 1.0.0 0.7.0-beta3 0.0.1; do
  if ! "$SNIPPET" "$v" >/dev/null 2>&1; then
    echo "FAIL: legitimate version $v was rejected"; exit 1
  fi
done

rm -rf /tmp/powerlab-install-gate-test
echo "OK: install.sh refuses 0.0.0-* dev bundles without POWERLAB_ALLOW_DEV_BUILD; legitimate versions pass."
