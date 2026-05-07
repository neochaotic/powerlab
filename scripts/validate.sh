#!/usr/bin/env bash
#
# Pre-flight validation — runs every check the CI matrix runs, locally,
# before you push. The goal is to catch ~99% of CI failures here, in
# minutes, instead of pushing to main and finding out 6 minutes later
# from a red badge.
#
# Usage:
#   ./scripts/validate.sh           # everything except the full package smoke
#   ./scripts/validate.sh --full    # also run package-linux.sh + Docker CGO
#   ./scripts/validate.sh --quick   # skip cross-compile, native Go only
#   ./scripts/validate.sh --no-ui   # skip the frontend pass
#
# Exit codes:
#   0  all checks passed
#   1  a check failed — output above explains which
#
# Time budget on a 2024 Mac (M-series):
#   --quick : ~30s  (frontend + native go test, single arch)
#   default : ~3m   (+ cross-compile to linux/amd64 and linux/arm64)
#   --full  : ~7m   (+ full package-linux.sh tarball + Docker CGO test)
#
# Source: see CONTRIBUTING.md "Pre-push validation".

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GREEN=$'\033[0;32m'
CYAN=$'\033[0;36m'
RED=$'\033[0;31m'
YELLOW=$'\033[1;33m'
DIM=$'\033[2m'
RESET=$'\033[0m'
step() { printf "\n%s▸ %s%s\n" "$CYAN" "$1" "$RESET"; }
ok()   { printf "%s✓%s %s\n" "$GREEN" "$RESET" "$1"; }
warn() { printf "%s!%s %s\n" "$YELLOW" "$RESET" "$1"; }
fail() { printf "%s✗%s %s\n" "$RED" "$RESET" "$1" >&2; exit 1; }

MODE="default"
SKIP_UI=0
while (( $# > 0 )); do
	case "$1" in
		--quick) MODE="quick"; shift ;;
		--full)  MODE="full"; shift ;;
		--no-ui) SKIP_UI=1; shift ;;
		--help|-h)
			sed -n '3,21p' "$0"; exit 0 ;;
		*) fail "unknown argument: $1 — try --help" ;;
	esac
done

START_TIME=$(date +%s)

# ── 0. Repo hygiene ─────────────────────────────────────────────────────
step "Repo: no developer home paths in tracked files"
./scripts/check-no-absolute-paths.sh || fail "absolute home paths leaked into tracked files — see output above"

# ── 1. Frontend ─────────────────────────────────────────────────────────
if (( SKIP_UI == 0 )); then
	step "Frontend: svelte-check"
	(cd ui && npx svelte-check --threshold error 2>&1 | tail -5) || fail "svelte-check failed"

	step "Frontend: vitest"
	(cd ui && npx vitest run 2>&1 | tail -5) || fail "vitest failed"

	step "Frontend: production build"
	(cd ui && npm run build > /tmp/pwlab-validate-build.log 2>&1) || {
		tail -20 /tmp/pwlab-validate-build.log
		fail "npm run build failed"
	}
	ok "frontend OK"
else
	warn "Skipping frontend (--no-ui)"
fi

# ── 2. Backend native (each service has its own go.mod) ─────────────────
SERVICES=(common gateway core user-service message-bus app-management local-storage cli)

for svc in "${SERVICES[@]}"; do
	step "Backend: $svc · go generate"
	# go generate is best-effort — the gateway has no directives, and
	# upstream URLs in cli sometimes 404. Failures here are caught
	# downstream by the build/test step, not now.
	(cd "backend/$svc" && go generate ./... > /dev/null 2>&1) || true

	# local-storage requires Linux fuse + xattr — even `go vet` chokes
	# on macOS because the syscalls (Setxattr, NETLINK_*, etc) are
	# Linux-only. Skip the entire backend phase for it on Darwin.
	# CI runs on Ubuntu and exercises the real path there.
	if [[ "$svc" == "local-storage" ]] && [[ "$(uname -s)" == "Darwin" ]]; then
		warn "Skipping local-storage on macOS (Linux fuse + xattr only) — CI runs it on Ubuntu"
		continue
	fi

	step "Backend: $svc · go vet"
	# vet is what caught the Go 1.25 fmt.Errorf regression. Run it
	# explicitly even though `go test` already does it, because we
	# want a separate, fast pass before the slower test phase.
	(cd "backend/$svc" && go vet ./... 2>&1) || fail "$svc: go vet"

	step "Backend: $svc · go test -race"
	(cd "backend/$svc" && go test -race ./... 2>&1 | tail -8) || fail "$svc: tests"
done

ok "backend native OK"

# ── 3. Backend cross-compile (catches arch-specific build failures) ─────
if [[ "$MODE" != "quick" ]]; then
	for svc in "${SERVICES[@]}"; do
		# CGO services need a target-specific C toolchain, which we
		# typically only have on Linux. Skip cross-compile for them
		# from a non-Linux host; --full covers them via Docker below.
		if [[ "$svc" == "user-service" ]] || [[ "$svc" == "local-storage" ]]; then
			if [[ "$(uname -s)" != "Linux" ]]; then
				warn "Skipping cross-compile of $svc on non-Linux host (needs CGO target toolchain)"
				continue
			fi
		fi
		# cli depends on a CasaOS-CLI codegen sub-repo we have not
		# forked; skip on non-Linux too because it would otherwise
		# require network on every validation.
		if [[ "$svc" == "cli" ]]; then
			continue
		fi

		for arch in amd64 arm64; do
			step "Backend: $svc · cross-compile linux/$arch"
			(cd "backend/$svc" && \
				GOOS=linux GOARCH="$arch" CGO_ENABLED=0 go build -o /dev/null ./... 2>&1) \
				|| fail "$svc: cross-compile linux/$arch"
		done
	done
	ok "backend cross-compile OK"
fi

# ── 4. Package smoke (optional — covers the install pipeline) ──────────
if [[ "$MODE" == "full" ]]; then
	step "Package smoke (linux/amd64)"
	# Reuse the existing ui/build to skip the npm rebuild — the
	# frontend test phase above already validated it.
	POWERLAB_SKIP_FRONTEND_BUILD=1 \
		./scripts/package-linux.sh amd64 0.0.0-validate > /tmp/pwlab-validate-pkg.log 2>&1 || {
			tail -30 /tmp/pwlab-validate-pkg.log
			fail "package-linux.sh amd64 failed"
		}
	ok "package smoke OK"

	# CGO + PAM cross-validation: spin up an Ubuntu container, build
	# user-service with libpam0g-dev present, and confirm the binary
	# links against libpam.so.0. Catches the v0.2 bug where we
	# almost shipped a CGO_ENABLED=0 user-service that silently
	# fell back to the stub.
	if command -v docker >/dev/null 2>&1; then
		step "Backend: user-service · CGO+PAM in Docker (linux/amd64)"
		docker run --rm --platform linux/amd64 \
			-v "$ROOT":/repo:ro \
			ubuntu:22.04 bash -c '
				set -e
				apt-get update -qq >/dev/null
				apt-get install -y -qq libpam0g-dev wget gcc >/dev/null 2>&1
				wget -q https://go.dev/dl/go1.25.0.linux-amd64.tar.gz -O /tmp/go.tar.gz
				tar -C /usr/local -xzf /tmp/go.tar.gz
				export PATH=$PATH:/usr/local/go/bin
				cp -R /repo /work
				cd /work/backend/user-service
				CGO_ENABLED=1 go build -o /tmp/us . > /tmp/build.log 2>&1
				ldd /tmp/us | grep -q libpam.so || { cat /tmp/build.log; echo "FAIL: libpam not linked"; exit 1; }
				echo "OK: libpam linked"
			' 2>&1 | tail -3 || fail "Docker CGO+PAM build failed"
		ok "Docker CGO+PAM OK"

		# Full Linux end-to-end: install in 3 docker scenarios (clean,
		# casaos-present-refuse, casaos-coexist) and exercise login,
		# editor, terminal websocket, app listing, and file upload.
		# This is the gate that catches Linux-specific regressions
		# that were invisible on macOS dev (paths, systemd ordering,
		# CasaOS coexistence).
		step "Linux E2E: install + login + editor + terminal + apps + upload"
		./scripts/test-linux-e2e.sh "$ROOT/dist/powerlab-0.0.0-validate-linux-amd64.tar.gz" \
			>/tmp/pwlab-validate-e2e.log 2>&1 || {
			tail -30 /tmp/pwlab-validate-e2e.log
			fail "Linux E2E failed (see /tmp/pwlab-validate-e2e.log)"
		}
		ok "Linux E2E OK (3 scenarios pass)"

		# Store-sample: install ≥10 apps and assert ≥94% pass. Catches
		# install-flow regressions that affect specific compose YAML
		# patterns (privileged, multi-service depends_on, secrets,
		# host networking, etc). Set-cover analysis (issue #42) shows
		# the default 10-app sample covers ~95% of the App Store
		# install code paths. Use --full for tag-time runs (18 apps,
		# ~99% coverage); default mode runs in CI on every release
		# tag without burning budget on patch releases.
		step "Store sample: install ≥10 apps, ≥94% pass"
		./scripts/test-store-sample.sh "$ROOT/dist/powerlab-0.0.0-validate-linux-amd64.tar.gz" \
			>/tmp/pwlab-validate-store.log 2>&1 || {
			tail -40 /tmp/pwlab-validate-store.log
			fail "Store sample failed (see /tmp/pwlab-validate-store.log)"
		}
		ok "Store sample OK"
	else
		warn "Docker not available — skipping CGO+PAM smoke, Linux E2E, store sample"
	fi
fi

ELAPSED=$(( $(date +%s) - START_TIME ))
echo
ok "All validations passed in ${ELAPSED}s"
