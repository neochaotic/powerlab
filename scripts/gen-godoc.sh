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
OUT_DIR="$REPO_ROOT/docs/api/pkg"

# Modules currently surfaced. Add to this list ONLY when a module's
# godoc coverage is high enough to be useful (target: ≥70% of exported
# decls have a leading comment). See the audit at
# issue #196.
MODULES=("pkg")

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

mkdir -p "$OUT_DIR"

for mod in "${MODULES[@]}"; do
  mod_dir="$REPO_ROOT/backend/$mod"
  if [[ ! -d "$mod_dir" ]]; then
    echo "[gen-godoc] WARN: backend/$mod missing; skipping"
    continue
  fi
  echo "[gen-godoc] generating for backend/$mod ..."
  pushd "$mod_dir" > /dev/null

  if (( CHECK_MODE )); then
    "$GOMARKDOC" --check --output "$OUT_DIR/{{.Dir}}.md" ./...
  else
    "$GOMARKDOC" --output "$OUT_DIR/{{.Dir}}.md" ./...
  fi

  popd > /dev/null
done

# Index page so the docs nav has a single entry point.
if (( CHECK_MODE == 0 )); then
  cat > "$OUT_DIR/index.md" <<'EOF'
# Go API reference — pkg/*

These are the foundation packages every PowerLab service consumes. They are PowerLab-owned (`github.com/neochaotic/powerlab/backend/pkg/*`), have 100% godoc coverage, and are stable across releases.

The pages below are auto-generated from the Go source via [gomarkdoc](https://github.com/princjef/gomarkdoc) — every release rebuilds them so they never drift from the code.

## Packages

- [logging](logging.md) — structured logger built on log/slog
- [errors](errors.md) — typed errors with code, i18n key, HTTP status
- [lifecycle](lifecycle.md) — graceful shutdown + panic recovery
- [tracing](tracing.md) — correlation IDs via X-Request-Id
- [foundation](foundation.md) — composes the above into one Wrap call
- [migrations](migrations.md) — versioned migration runner over goose

## Service packages

Per-service Go packages (`backend/<svc>/`) are NOT in this site yet — godoc coverage there is below the 70% threshold for inclusion (tracked in [issue #196](https://github.com/neochaotic/powerlab/issues/196) with a per-module raise plan). They'll be surfaced once each service hits the bar.

For now, browse them on GitHub:

- [backend/gateway](https://github.com/neochaotic/powerlab/tree/main/backend/gateway)
- [backend/core](https://github.com/neochaotic/powerlab/tree/main/backend/core)
- [backend/user-service](https://github.com/neochaotic/powerlab/tree/main/backend/user-service)
- [backend/message-bus](https://github.com/neochaotic/powerlab/tree/main/backend/message-bus)
- [backend/app-management](https://github.com/neochaotic/powerlab/tree/main/backend/app-management)
- [backend/local-storage](https://github.com/neochaotic/powerlab/tree/main/backend/local-storage)
- [backend/common](https://github.com/neochaotic/powerlab/tree/main/backend/common)
EOF
  echo "[gen-godoc] wrote index at $OUT_DIR/index.md"
fi

echo "[gen-godoc] done."
