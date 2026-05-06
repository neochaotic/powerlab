#!/usr/bin/env bash
#
# PowerLab macOS dev-mode installer.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/neochaotic/powerlab/main/install-mac.sh | bash
#
# What it does:
#   1. Verifies you're on macOS with Apple Silicon.
#   2. Checks prerequisites (Homebrew, git, go, node, Docker).
#   3. Clones the PowerLab repo to ~/Documents/powerlab (or PWLAB_HOME).
#   4. Hands off to ./dev.sh — the standard developer entry point.
#
# Why "dev mode"? PowerLab in production is a Linux-only systemd panel.
# On macOS we run the same SvelteKit + Go stack via `dev.sh` so you can
# use the panel locally for development, demos, or experimentation —
# but the file manager (which depends on Linux fuse + xattr) is disabled,
# and there is no auto-start at boot. For a real production install,
# point a Pi or mini-PC at install.sh.
#
# Source: https://github.com/neochaotic/powerlab/blob/main/install-mac.sh

set -euo pipefail

GREEN=$'\033[0;32m'
CYAN=$'\033[0;36m'
RED=$'\033[0;31m'
YELLOW=$'\033[1;33m'
DIM=$'\033[2m'
RESET=$'\033[0m'
log()  { printf "%s▸%s %s\n" "$CYAN" "$RESET" "$1"; }
ok()   { printf "%s✓%s %s\n" "$GREEN" "$RESET" "$1"; }
warn() { printf "%s!%s %s\n" "$YELLOW" "$RESET" "$1"; }
die()  { printf "%s✗%s %s\n" "$RED" "$RESET" "$1" >&2; exit 1; }

PWLAB_HOME="${PWLAB_HOME:-$HOME/Documents/powerlab}"
REPO_URL="https://github.com/neochaotic/powerlab.git"

# ── Pre-flight ──────────────────────────────────────────────────────────
[[ "$(uname -s)" == "Darwin" ]] || die "this installer is for macOS — for Linux production install use install.sh"

ARCH="$(uname -m)"
if [[ "$ARCH" != "arm64" ]]; then
	warn "PowerLab dev mode is tested on Apple Silicon (arm64). You're on $ARCH — should still work, but is unverified."
fi

[[ "$EUID" -ne 0 ]] || die "do NOT run this with sudo — dev mode runs in your user account"

# ── Dependency checks ───────────────────────────────────────────────────
log "Checking prerequisites…"

MISSING=()
command -v git    >/dev/null || MISSING+=(git)
command -v go     >/dev/null || MISSING+=(go)
command -v node   >/dev/null || MISSING+=(node)
command -v docker >/dev/null || MISSING+=(docker)

if (( ${#MISSING[@]} > 0 )); then
	echo
	echo "Missing required tools: ${MISSING[*]}"
	echo
	echo "Install everything in one go with Homebrew:"
	echo "  brew install git go node"
	echo "  Get Docker Desktop from https://www.docker.com/products/docker-desktop"
	echo
	die "install the missing tools above, then re-run this script"
fi
ok "Prerequisites OK"

# ── Clone or update the repo ────────────────────────────────────────────
if [[ -d "$PWLAB_HOME/.git" ]]; then
	log "Existing checkout at $PWLAB_HOME — pulling latest…"
	(cd "$PWLAB_HOME" && git pull --ff-only) || die "git pull failed; resolve manually then re-run"
	ok "Updated"
else
	if [[ -e "$PWLAB_HOME" ]]; then
		die "$PWLAB_HOME exists but is not a git checkout. Move it aside or set PWLAB_HOME to a different path."
	fi
	log "Cloning PowerLab into $PWLAB_HOME…"
	mkdir -p "$(dirname "$PWLAB_HOME")"
	git clone --depth 1 "$REPO_URL" "$PWLAB_HOME"
	ok "Cloned"
fi

# ── Hand off to dev.sh ──────────────────────────────────────────────────
echo
log "Starting PowerLab dev mode…"
echo "  ${DIM}Repo: $PWLAB_HOME${RESET}"
echo "  ${DIM}To stop everything: Ctrl-C (or run \`./dev.sh --stop\`)${RESET}"
echo
cd "$PWLAB_HOME"
exec ./dev.sh
