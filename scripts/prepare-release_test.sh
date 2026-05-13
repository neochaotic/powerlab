#!/usr/bin/env bash
# Regression tests for scripts/prepare-release.sh — the atomic cut
# preparation script (v0.6.6 retro defense layer).
#
# Mix of structural-grep tests (cheap, lock the script's shape) and
# sandboxed behavioral tests. Behavioral tests run in a temp git repo
# under a stubbed PATH that intercepts changie + npm + git so we
# never mutate the actual working tree.
#
# Key invariant locked here: prepare-release.sh MUST NOT commit, push,
# or tag. The project's release-auth rule (memory
# feedback_require_explicit_release_auth) requires explicit user
# authorization for those operations — staging files is fine,
# tagging is gated.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TARGET="$REPO_ROOT/scripts/prepare-release.sh"

failures=0

assert_grep() {
  local description="$1"
  local pattern="$2"
  if grep -q -F -- "$pattern" "$TARGET"; then
    echo "  PASS: $description"
  else
    echo "  FAIL: $description (pattern '$pattern' not found)" >&2
    failures=$((failures + 1))
  fi
}

assert_grep_extended() {
  local description="$1"
  local pattern="$2"
  if grep -qE -- "$pattern" "$TARGET"; then
    echo "  PASS: $description"
  else
    echo "  FAIL: $description (extended-regex '$pattern' not found)" >&2
    failures=$((failures + 1))
  fi
}

assert_no_grep_extended() {
  local description="$1"
  local pattern="$2"
  if ! grep -qE -- "$pattern" "$TARGET"; then
    echo "  PASS: $description"
  else
    echo "  FAIL: $description (forbidden pattern '$pattern' present)" >&2
    failures=$((failures + 1))
  fi
}

echo "Test: required release-prep steps wired in"
assert_grep "changie batch step"  "changie batch \"v\$VERSION\""
assert_grep "changie merge step"  "changie merge"
assert_grep "npm version sync"    "npm version \"\$VERSION\" --no-git-tag-version --allow-same-version"
assert_grep "git add stages"      "git add .changes ui/package.json CHANGELOG.md release-manifest.yaml"

echo
echo "Test: release-auth invariant — script does NOT commit/push/tag"
assert_no_grep_extended "no 'git commit' invocation"  '^[^#]*git[[:space:]]+commit'
assert_no_grep_extended "no 'git push' invocation"    '^[^#]*git[[:space:]]+push'
assert_no_grep_extended "no 'git tag -a' invocation"  '^[^#]*git[[:space:]]+tag[[:space:]]+-a'

echo
echo "Test: semver validation present"
assert_grep_extended "version semver regex anchored" 'does not look like semver'
assert_grep_extended "leading v strip"               'VERSION="\$\{VERSION#v\}"'

echo
echo "Test: clean-tree precondition guard"
assert_grep "diff --quiet check"   "git diff --quiet HEAD"

echo
echo "Behavioral tests (sandbox with stubbed PATH)"

SANDBOX=$(mktemp -d)
trap 'rm -rf "$SANDBOX"' EXIT

# Stub binaries that record calls but mutate nothing.
mkdir -p "$SANDBOX/stubs"
cat >"$SANDBOX/stubs/changie" <<'STUB'
#!/usr/bin/env bash
echo "[stub-changie] $*" >>"$SANDBOX_LOG"
exit 0
STUB
cat >"$SANDBOX/stubs/npm" <<'STUB'
#!/usr/bin/env bash
echo "[stub-npm] $*" >>"$SANDBOX_LOG"
exit 0
STUB
# Wrap git with a stub that records ALL invocations but defers to the
# real git only for the read-only ops the script needs to pass its
# clean-tree precheck. This forces commit/push/tag attempts to be
# captured AND no-op.
cat >"$SANDBOX/stubs/git" <<'STUB'
#!/usr/bin/env bash
echo "[stub-git] $*" >>"$SANDBOX_LOG"
case "$1" in
  diff)
    # Always report clean tree for the precheck.
    exit 0
    ;;
  status)
    exit 0
    ;;
  add)
    # Pretend success.
    exit 0
    ;;
  rev-parse|config|log|tag)
    # Defer to real git for any read-only flow the script doesn't
    # actually exercise but might in the future.
    exec /usr/bin/env -i PATH="$REAL_PATH" /usr/bin/git "$@"
    ;;
  *)
    echo "[stub-git] UNEXPECTED git subcommand: $*" >&2
    exit 1
    ;;
esac
STUB
chmod +x "$SANDBOX/stubs/changie" "$SANDBOX/stubs/npm" "$SANDBOX/stubs/git"

export SANDBOX_LOG="$SANDBOX/calls.log"
export REAL_PATH="$PATH"
touch "$SANDBOX_LOG"

run_sandboxed() {
  PATH="$SANDBOX/stubs:$REAL_PATH" "$TARGET" "$@"
}

echo
echo "  -- invalid version arg rejected"
if run_sandboxed "not-a-semver" >/dev/null 2>&1; then
  echo "  FAIL: accepted invalid version 'not-a-semver'" >&2
  failures=$((failures + 1))
else
  echo "  PASS: rejected invalid version 'not-a-semver'"
fi

echo
echo "  -- missing version arg rejected"
if run_sandboxed >/dev/null 2>&1; then
  echo "  FAIL: accepted empty version arg" >&2
  failures=$((failures + 1))
else
  echo "  PASS: rejected empty version arg"
fi

echo
echo "  -- valid version with leading 'v' is stripped + accepted"
: >"$SANDBOX_LOG"
if ! run_sandboxed "v1.2.3" >/dev/null 2>&1; then
  echo "  FAIL: 'v1.2.3' was rejected by the script" >&2
  failures=$((failures + 1))
elif ! grep -q 'batch v1.2.3' "$SANDBOX_LOG"; then
  echo "  FAIL: changie not called with 'v1.2.3' (log:)" >&2
  cat "$SANDBOX_LOG" >&2
  failures=$((failures + 1))
elif ! grep -q 'version 1.2.3' "$SANDBOX_LOG"; then
  echo "  FAIL: npm version not called with stripped '1.2.3' (log:)" >&2
  cat "$SANDBOX_LOG" >&2
  failures=$((failures + 1))
else
  echo "  PASS: leading 'v' stripped + changie/npm called with right values"
fi

echo
echo "  -- script never invoked 'git commit / push / tag -a' during run"
if grep -qE '^\[stub-git\] (commit|push|tag -a)' "$SANDBOX_LOG"; then
  echo "  FAIL: forbidden git op ran during prepare-release (log:)" >&2
  grep -E '^\[stub-git\]' "$SANDBOX_LOG" >&2
  failures=$((failures + 1))
else
  echo "  PASS: no git commit/push/tag in sandbox run"
fi

echo
if [[ "$failures" == "0" ]]; then
  echo "OK: all checks passed"
  exit 0
else
  echo "FAIL: $failures failure(s)" >&2
  exit 1
fi
