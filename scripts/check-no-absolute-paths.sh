#!/usr/bin/env bash
# Fails if any tracked file contains an absolute path that looks like
# a developer's home directory (`/Users/<name>/...` on macOS,
# `/home/<name>/...` on Linux). Catches the recurring regression where
# a generated runtime config (gateway.ini, core.conf, etc.) gets
# committed with the working dev's home path baked in — leaks the
# author's username and breaks every clone on someone else's machine.
#
# Allowlist: paths inside `dist/` are tarball staging output that is
# regenerated at every package step; paths inside `backend/.../build/sysroot/`
# are vendored prod templates that legitimately reference `/var/lib/...`,
# `/var/run/...`, etc. Test fixtures sometimes need absolute paths too —
# we exclude `*_test.go` and `*.test.ts` snapshots.

set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

# Pattern: `/Users/<word>/` (macOS home) OR `/home/<word>/` (Linux
# home), where <word> is at least two chars to avoid catching unrelated
# paths like `/home/server` (a public docs reference) or
# `/Users/Shared` (an Apple system path).
PATTERN='(/Users/[a-zA-Z][a-zA-Z0-9_.-]+/|/home/[a-zA-Z][a-zA-Z0-9_.-]+/)'

EXCLUDE_DIRS=(
  ':!dist/**'
  ':!**/build/sysroot/**'
  ':!ui/build/**'
  ':!**/node_modules/**'
)

# Allow legit test fixtures and docs that reference paths as examples.
# Each entry is a path glob OR a substring guard checked per match.
EXCLUDE_FILES=(
  ':!**/*_test.go'
  ':!**/*.test.ts'
  ':!**/*.test.js'
  ':!docs/**'
  ':!CHANGELOG.md'
  ':!**/CONTRIBUTING.md'
  ':!**/README.md'
  ':!scripts/check-no-absolute-paths.sh'
)

# Use git grep so we only scan TRACKED files. Untracked files (like
# the per-machine gateway.ini that lives next to the binary) are not
# our concern — they're already gitignored.
#
# Two passes:
#   1. STRICT — config-file types where any absolute home path is a
#      leak (gateway.ini, *.conf, *.yaml, *.json). These files exist
#      to be deployed; a developer's home path in there ships to prod.
#   2. ADVISORY — source code (.go, .sh, .ts) where absolute paths
#      sometimes appear in comments / example data / test fixtures.
#      We only flag matches against the CURRENT user's actual home,
#      because that's a 100% leak (the developer accidentally typed
#      their own path) versus a placeholder like `/home/user1`.
CONFIG_GLOBS=(
  '*.ini'
  '*.conf'
  '*.yaml'
  '*.yml'
  '*.json'
)

# Strict pass: any home-shaped path in a config file
STRICT=$(git grep -nE "$PATTERN" -- "${CONFIG_GLOBS[@]}" "${EXCLUDE_DIRS[@]}" "${EXCLUDE_FILES[@]}" 2>/dev/null || true)

# Advisory pass: only the runner's actual home path, anywhere
USER_HOME_RE=""
if [[ -n "${USER:-}" ]]; then
  USER_HOME_RE="(/Users/${USER}/|/home/${USER}/)"
  ADVISORY=$(git grep -nE "$USER_HOME_RE" -- "${EXCLUDE_DIRS[@]}" "${EXCLUDE_FILES[@]}" 2>/dev/null || true)
else
  ADVISORY=""
fi

HITS=""
[[ -n "$STRICT"   ]] && HITS+="${STRICT}"$'\n'
[[ -n "$ADVISORY" ]] && HITS+="${ADVISORY}"$'\n'
HITS="${HITS%$'\n'}"

if [[ -n "$HITS" ]]; then
  echo "absolute home paths leaked into tracked files:" >&2
  echo "$HITS" >&2
  echo "" >&2
  echo "Fix: replace the absolute path with a relative one or an env-var-driven default," >&2
  echo "or add the file to .gitignore if it is a per-machine generated artifact." >&2
  exit 1
fi

echo "no absolute home paths found in tracked files."
