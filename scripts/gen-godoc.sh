#!/usr/bin/env bash
# Generate per-package markdown documentation for selected Go modules
# from their godoc comments via gomarkdoc. Output lands in docs/api/<mod>/
# and is consumed by the mkdocs site build.
#
# Currently scoped to backend/pkg/* — those packages have 100% godoc
# coverage (Sprint 2 Phase 6) so the generated output is high signal.
# Other modules will be added once they hit a similar coverage bar; see
# issue #196 for the per-module scorecard.
#
# Usage:
#   ./scripts/gen-godoc.sh           # generate
#   ./scripts/gen-godoc.sh --check   # CI mode: fail if generated
#                                    # docs are out of sync with source
#
# Idempotent. Designed to run before `mkdocs build` (in CI) and on
# demand locally before reviewing the site.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_BASE="$REPO_ROOT/docs/api"

# Modules currently surfaced. Add to this list ONLY when a module's
# godoc coverage is ≥70% (target). See issue #196 for the per-module
# scorecard. Each module gets its own subdir under docs/api/.
MODULES=("pkg" "gateway" "user-service" "message-bus")

# Locate gomarkdoc — `go install` puts it under $(go env GOPATH)/bin.
GOMARKDOC="$(command -v gomarkdoc || true)"
if [[ -z "$GOMARKDOC" ]]; then
  GOMARKDOC="$(go env GOPATH)/bin/gomarkdoc"
fi
if [[ ! -x "$GOMARKDOC" ]]; then
  echo "[gen-godoc] gomarkdoc not found; installing..."
  go install github.com/princjef/gomarkdoc/cmd/gomarkdoc@v1.1.0
  GOMARKDOC="$(go env GOPATH)/bin/gomarkdoc"
fi

CHECK_MODE=0
if [[ "${1:-}" == "--check" ]]; then
  CHECK_MODE=1
fi

for mod in "${MODULES[@]}"; do
  mod_dir="$REPO_ROOT/backend/$mod"
  out_dir="$OUT_BASE/$mod"
  if [[ ! -d "$mod_dir" ]]; then
    echo "[gen-godoc] WARN: backend/$mod missing; skipping"
    continue
  fi
  mkdir -p "$out_dir"
  echo "[gen-godoc] generating for backend/$mod -> $out_dir/"
  pushd "$mod_dir" > /dev/null

  if (( CHECK_MODE )); then
    "$GOMARKDOC" --check --output "$out_dir/{{.Dir}}.md" ./...
  else
    "$GOMARKDOC" --output "$out_dir/{{.Dir}}.md" ./...
  fi

  popd > /dev/null
done

echo "[gen-godoc] done. (Per-module index pages live committed at docs/api/<mod>/index.md.)"
