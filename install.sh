#!/usr/bin/env bash
#
# PowerLab one-line installer / upgrader.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/neochaotic/powerlab/main/install.sh | sudo bash
#
# Or, for a specific version:
#   curl -fsSL https://raw.githubusercontent.com/neochaotic/powerlab/main/install.sh | sudo bash -s -- --version v0.1.5
#
# What it does:
#   1. Detects your CPU architecture (amd64 / arm64).
#   2. Downloads the matching PowerLab release tarball into a temp dir.
#   3. Verifies it actually downloaded (catches CDN cache 404s).
#   4. Extracts and runs the bundled install.sh.
#   5. Cleans up.
#
# Re-run this any time you want to upgrade — it is fully idempotent.
#
# Source: https://github.com/neochaotic/powerlab/blob/main/install.sh

set -euo pipefail

# ── Pretty output ───────────────────────────────────────────────────────
GREEN=$'\033[0;32m'
CYAN=$'\033[0;36m'
RED=$'\033[0;31m'
DIM=$'\033[2m'
RESET=$'\033[0m'
log()  { printf "%s▸%s %s\n" "$CYAN" "$RESET" "$1"; }
ok()   { printf "%s✓%s %s\n" "$GREEN" "$RESET" "$1"; }
die()  { printf "%s✗%s %s\n" "$RED" "$RESET" "$1" >&2; exit 1; }

# ── Args ────────────────────────────────────────────────────────────────
VERSION="latest"
while (( $# > 0 )); do
	case "$1" in
		--version)
			VERSION="${2:?}"; shift 2 ;;
		--help|-h)
			sed -n '2,21p' "$0"; exit 0 ;;
		*) die "unknown argument: $1 — try --help" ;;
	esac
done

# ── Pre-flight ──────────────────────────────────────────────────────────
[[ "$EUID" -eq 0 ]] || die "this installer must run as root — try: curl … | sudo bash"

case "$(uname -s)" in
	Linux) ;;
	*) die "PowerLab is Linux-only. For macOS, use the dev script: ./dev.sh" ;;
esac

ARCH_RAW="$(uname -m)"
case "$ARCH_RAW" in
	x86_64|amd64)         ARCH=amd64 ;;
	aarch64|arm64)        ARCH=arm64 ;;
	*) die "unsupported architecture: $ARCH_RAW (PowerLab ships amd64 and arm64 only)" ;;
esac

command -v curl   >/dev/null || die "curl is required"
command -v tar    >/dev/null || die "tar is required"
command -v docker >/dev/null || printf "%s!%s docker not found — App Store install will not work without it. Continuing.\n" "$DIM" "$RESET"

# ── Build the download URL ──────────────────────────────────────────────
if [[ "$VERSION" == "latest" ]]; then
	URL="https://github.com/neochaotic/powerlab/releases/latest/download/powerlab-linux-${ARCH}.tar.gz"
	TAG_LABEL="latest"
else
	# Versioned tarball: e.g. powerlab-0.1.5-linux-amd64.tar.gz
	V="${VERSION#v}"  # strip leading v if present
	URL="https://github.com/neochaotic/powerlab/releases/download/v${V}/powerlab-${V}-linux-${ARCH}.tar.gz"
	TAG_LABEL="v${V}"
fi

# ── Download into a sandboxed temp dir ──────────────────────────────────
TMPDIR="$(mktemp -d -t powerlab-install.XXXXXX)"
trap 'rm -rf "$TMPDIR"' EXIT
TARBALL="$TMPDIR/powerlab.tar.gz"

log "Downloading PowerLab $TAG_LABEL ($ARCH)…"
log "  $DIM$URL$RESET"
if ! curl -fsSL --retry 3 --retry-delay 2 -o "$TARBALL" "$URL"; then
	die "download failed (URL: $URL)"
fi

# Sanity-check: GitHub's CDN sometimes serves a 9-byte 'Not Found' body
# under a 200 redirect for very fresh releases. Refuse anything implausible.
SIZE=$(stat -c%s "$TARBALL" 2>/dev/null || stat -f%z "$TARBALL" 2>/dev/null || echo 0)
if (( SIZE < 1000000 )); then
	die "downloaded tarball is only ${SIZE} bytes — release probably propagating, retry in a minute"
fi
ok "Downloaded ${SIZE} bytes"

# ── Extract into the same temp dir ──────────────────────────────────────
log "Extracting…"
EXTRACT_DIR="$TMPDIR/extracted"
mkdir -p "$EXTRACT_DIR"
if ! tar -xzf "$TARBALL" -C "$EXTRACT_DIR"; then
	die "tar extraction failed"
fi

# The release tarball contains a single top-level directory named
# powerlab-<version>-linux-<arch>/. Find it (handles both versioned and
# stable filenames, since both ship the versioned inner directory).
INNER="$(find "$EXTRACT_DIR" -mindepth 1 -maxdepth 1 -type d | head -1)"
[[ -n "$INNER" ]] || die "expected a single top-level directory in the tarball, found none"
# Use -f instead of -x so this also works on hosts where /tmp is mounted
# noexec (Docker, kiosks, hardened distros). We invoke the bundled
# install.sh via `bash` below, which does not need the exec bit.
[[ -f "$INNER/install.sh" ]] || die "$INNER/install.sh missing"
ok "Extracted to $INNER"

# ── Run the bundled installer ───────────────────────────────────────────
log "Running bundled installer…"
echo
bash "$INNER/install.sh"

# Cleanup happens via the EXIT trap. Done.
