#!/usr/bin/env bash
# bundle-store.sh — pull a tagged release of powerlab-store into the
# community-catalog/Apps/ directory, converting the store's x-powerlab
# block to the format consumed by PowerLab core's app-management backend.
#
# Run before cutting a PowerLab release that bundles store apps.
# Commit the result; CI and install.sh pick it up automatically.
#
# Usage:
#   ./scripts/bundle-store.sh v0.1.0          # pull tagged release (preferred)
#   ./scripts/bundle-store.sh main            # pull main (dev/testing only)
#   ./scripts/bundle-store.sh --list v0.1.0   # dry-run: print what would change
#
# Format conversions applied (store → core legacy):
#   x-powerlab.title: "X"          → title: {en_us: "X"}
#   x-powerlab.tagline: "X"        → tagline: {en_us: "X"}
#   x-powerlab.port_map: [{...}]   → port_map: "host_port" (first http/https)
#   x-powerlab.icon: {file: name}  → icon: raw.githubusercontent.com URL
#   x-powerlab.category: "A & B"   → unchanged (UI displays as-is)
#
# Manually-curated apps in community-catalog/Apps/<id>/x-powerlab.yml are
# NEVER overwritten by this script — the store entry for those apps is
# silently skipped so the curated metadata takes precedence.
#
# Requires: curl, python3 (pip install pyyaml), jq

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CATALOG_DIR="$REPO_ROOT/community-catalog"
STORE_OWNER="neochaotic"
STORE_REPO="powerlab-store"

DRY_RUN=0
if [[ "${1:-}" == "--list" ]]; then
  DRY_RUN=1
  shift
fi

REF="${1:-main}"
RAW_BASE="https://raw.githubusercontent.com/${STORE_OWNER}/${STORE_REPO}/${REF}"

echo "powerlab bundle-store: ref=${REF} dry_run=${DRY_RUN}"

# Download index.json
INDEX_TMP="$(mktemp)"
trap 'rm -f "$INDEX_TMP"' EXIT

echo "  Fetching ${RAW_BASE}/index.json ..."
if ! curl -fsSL "${RAW_BASE}/index.json" -o "$INDEX_TMP"; then
  echo "ERROR: could not fetch index.json for ref=${REF}" >&2
  echo "       Check that the tag exists: https://github.com/${STORE_OWNER}/${STORE_REPO}/tags" >&2
  exit 1
fi

APP_COUNT="$(jq -r '.apps_count' "$INDEX_TMP")"
STORE_VERSION="$(jq -r '.store_version' "$INDEX_TMP")"
echo "  Store version: ${STORE_VERSION}, ${APP_COUNT} apps"

if [[ "$DRY_RUN" == "1" ]]; then
  echo ""
  echo "Apps that would be bundled (excluding manually-curated overrides):"
  # Find curated apps
  CURATED_SET=""
  while IFS= read -r -d '' f; do
    id="$(basename "$(dirname "$f")")"
    CURATED_SET="$CURATED_SET $id"
  done < <(find "$CATALOG_DIR/Apps" -name "x-powerlab.yml" -print0 2>/dev/null)

  jq -r '.apps[].store_app_id' "$INDEX_TMP" | while read -r id; do
    if echo "$CURATED_SET" | grep -qw "$id"; then
      echo "  [SKIP curated] $id"
    else
      echo "  [bundle]       $id"
    fi
  done
  exit 0
fi

# Build curated set (apps with x-powerlab.yml are manually curated — never overwrite)
CURATED_ARGS=()
while IFS= read -r -d '' f; do
  CURATED_ARGS+=("$(basename "$(dirname "$f")")")
done < <(find "$CATALOG_DIR/Apps" -name "x-powerlab.yml" -print0 2>/dev/null)
echo "  Preserving ${#CURATED_ARGS[@]} manually-curated app(s)${CURATED_ARGS:+: ${CURATED_ARGS[*]}}"

# Main conversion pass
python3 - "$INDEX_TMP" "$CATALOG_DIR" "$RAW_BASE" "${CURATED_ARGS[@]+"${CURATED_ARGS[@]}"}" <<'PYEOF'
import json, sys, urllib.request, urllib.error
from pathlib import Path
import yaml

index_path = sys.argv[1]
catalog_dir = Path(sys.argv[2])
raw_base = sys.argv[3]
curated_set = set(sys.argv[4:])

with open(index_path) as fh:
    index = json.load(fh)

apps_dir = catalog_dir / "Apps"
bundled = skipped = failed = 0


def fetch(url: str) -> bytes | None:
    try:
        with urllib.request.urlopen(url) as r:
            return r.read()
    except urllib.error.HTTPError as e:
        if e.code == 404:
            return None
        raise


def convert_xp(xp: dict, app_id: str, raw_base: str) -> dict:
    """Convert store x-powerlab block to core legacy format."""
    out = dict(xp)

    # title + tagline: flat string → {en_us: ...}
    for field in ("title", "tagline"):
        val = out.get(field)
        if isinstance(val, str) and val:
            out[field] = {"en_us": val}

    # port_map: list of {container, host, protocol} → "host" string
    pm = out.get("port_map")
    if isinstance(pm, list):
        host_port = None
        for entry in pm:
            if not isinstance(entry, dict):
                continue
            proto = (entry.get("protocol") or "http").lower()
            if proto in ("http", "https"):
                host_port = str(entry.get("host", ""))
                break
        out["port_map"] = host_port or ""

    # icon: {file: "icon.svg", ...} → raw.githubusercontent.com URL
    icon_meta = out.get("icon")
    if isinstance(icon_meta, dict):
        icon_file = icon_meta.get("file", "icon.svg")
        out["icon"] = f"{raw_base}/Apps/{app_id}/{icon_file}"

    return out


for app in index["apps"]:
    app_id = app["store_app_id"]

    if app_id in curated_set:
        print(f"  skip (curated): {app_id}")
        skipped += 1
        continue

    app_dir = apps_dir / app_id
    app_dir.mkdir(parents=True, exist_ok=True)

    # Fetch docker-compose.yml
    compose_url = f"{raw_base}/Apps/{app_id}/docker-compose.yml"
    raw = fetch(compose_url)
    if raw is None:
        print(f"  FAIL (404): {app_id}/docker-compose.yml")
        failed += 1
        continue

    try:
        doc = yaml.safe_load(raw.decode())
    except Exception as e:
        print(f"  FAIL (yaml): {app_id}: {e}")
        failed += 1
        continue

    # Apply x-powerlab conversion
    if isinstance(doc.get("x-powerlab"), dict):
        doc["x-powerlab"] = convert_xp(doc["x-powerlab"], app_id, raw_base)

    # Serialize — PyYAML reformats keys but preserves all values
    converted = yaml.dump(doc, default_flow_style=False, allow_unicode=True, sort_keys=False)
    (app_dir / "docker-compose.yml").write_text(converted)

    # Fetch description.md (optional)
    desc = fetch(f"{raw_base}/Apps/{app_id}/description.md")
    if desc is not None:
        (app_dir / "description.md").write_bytes(desc)

    print(f"  bundled: {app_id}")
    bundled += 1

print(f"\nResult: bundled={bundled}  skipped_curated={skipped}  failed={failed}")
if failed:
    sys.exit(1)
PYEOF

# Regenerate .curated-manifest (one app slug per line, sorted)
MANIFEST_FILE="$CATALOG_DIR/.curated-manifest"
find "$CATALOG_DIR/Apps" -mindepth 1 -maxdepth 1 -type d -exec basename {} \; \
  | sort > "$MANIFEST_FILE"
MANIFEST_COUNT=$(wc -l < "$MANIFEST_FILE" | tr -d ' ')
echo "  Updated .curated-manifest (${MANIFEST_COUNT} entries)"

echo ""
echo "Done. Next steps:"
echo "  1. Review: git diff community-catalog/"
echo "  2. Commit: git add community-catalog/ && git commit -m 'chore(catalog): bundle store ${REF}'"
