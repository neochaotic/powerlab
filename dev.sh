#!/usr/bin/env bash
# PowerLab — one-shot developer bootstrap
#
# Brings up the entire stack with a single command:
#   1. starts every backend service (./start.sh --build on first run)
#   2. installs UI dependencies if needed
#   3. starts the Vite dev server in the foreground
#
# Stops cleanly with Ctrl-C — both backend and UI are torn down.
#
# Usage:
#   ./dev.sh              first run, or after pulling new code (rebuilds binaries)
#   ./dev.sh --no-build   skip the backend rebuild (faster restarts)
#   ./dev.sh --stop       stop everything started by this script

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
UI_DIR="$SCRIPT_DIR/ui"

CYAN=$'\033[0;36m'
GREEN=$'\033[0;32m'
YELLOW=$'\033[1;33m'
RED=$'\033[0;31m'
RESET=$'\033[0m'

step() { printf "%s▸ %s%s\n" "$CYAN" "$1" "$RESET"; }
ok()   { printf "%s✓ %s%s\n" "$GREEN" "$1" "$RESET"; }
warn() { printf "%s! %s%s\n" "$YELLOW" "$1" "$RESET"; }
die()  { printf "%s✗ %s%s\n" "$RED" "$1" "$RESET" >&2; exit 1; }

# ── Stop mode ───────────────────────────────────────────────────────────
if [[ "${1:-}" == "--stop" ]]; then
	step "Stopping backend services"
	"$SCRIPT_DIR/start.sh" --stop || true
	step "Stopping any leftover Vite dev server"
	pkill -f "vite.*--port 5173" 2>/dev/null || true
	ok "Stopped"
	exit 0
fi

# ── Dependency checks ───────────────────────────────────────────────────
step "Checking prerequisites"
command -v go     >/dev/null || die "Go is required. Install from https://go.dev/dl/"
command -v node   >/dev/null || die "Node.js 20+ is required. Install from https://nodejs.org/"
command -v npm    >/dev/null || die "npm is required (ships with Node.js)"
command -v docker >/dev/null || warn "Docker not found — App Store install will not work without it"
ok "Prerequisites OK"

# ── UI deps ─────────────────────────────────────────────────────────────
if [[ ! -d "$UI_DIR/node_modules" ]]; then
	step "Installing UI dependencies (one-time, ~30s)"
	(cd "$UI_DIR" && npm install --silent)
	ok "UI dependencies installed"
fi

# ── Backend ─────────────────────────────────────────────────────────────
BUILD_FLAG="--build"
[[ "${1:-}" == "--no-build" ]] && BUILD_FLAG=""

step "Starting backend services${BUILD_FLAG:+ (with rebuild)}"
"$SCRIPT_DIR/start.sh" $BUILD_FLAG
ok "Backend up — gateway listening on http://localhost"

# ── Cleanup trap ────────────────────────────────────────────────────────
cleanup() {
	echo
	step "Shutting down"
	"$SCRIPT_DIR/start.sh" --stop 2>/dev/null || true
	ok "All services stopped"
}
trap cleanup INT TERM

# ── UI dev server ───────────────────────────────────────────────────────
echo
step "Starting Vite dev server"
echo
echo "  ${GREEN}→  http://localhost:5173${RESET}    (UI with hot reload)"
echo "  ${GREEN}→  http://powerlab.local${RESET}    (backend gateway, reachable on the LAN)"
echo
echo "  Press Ctrl-C to stop everything."
echo
cd "$UI_DIR"
npm run dev -- --host 0.0.0.0 --port 5173
