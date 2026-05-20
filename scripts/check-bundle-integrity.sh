#!/usr/bin/env bash
# check-bundle-integrity.sh — validate that every app in community-catalog/Apps/
# has an x-powerlab block readable by PowerLab core's app-management backend.
#
# Run after scripts/bundle-store.sh (and as a CI gate on community-catalog/ changes)
# to catch format regressions before they reach a release build.
#
# Checks:
#   1. x-powerlab.title.en_us is a non-empty string
#   2. x-powerlab.tagline.en_us is a non-empty string
#   3. x-powerlab.port_map is a non-empty string  OR  headless: true
#   4. x-powerlab.icon is a non-empty string (URL)
#   5. .curated-manifest lists exactly the same app IDs as Apps/ directories
#
# Exit 0 — all apps pass
# Exit 1 — one or more apps fail

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CATALOG_DIR="$REPO_ROOT/community-catalog"
APPS_DIR="$CATALOG_DIR/Apps"
MANIFEST="$CATALOG_DIR/.curated-manifest"

echo "check-bundle-integrity: $CATALOG_DIR"

if [[ ! -d "$APPS_DIR" ]]; then
  echo "SKIP: $APPS_DIR does not exist (bundle-store.sh not run yet)"
  exit 0
fi

failures=0
checked=0

python3 - "$APPS_DIR" <<'PYEOF'
import sys, yaml
from pathlib import Path

apps_dir = Path(sys.argv[1])
failures = 0
checked = 0

for compose_path in sorted(apps_dir.glob("*/docker-compose.yml")):
    app_id = compose_path.parent.name
    errors = []

    try:
        doc = yaml.safe_load(compose_path.read_text())
    except Exception as e:
        print(f"  FAIL {app_id}: yaml parse error: {e}")
        failures += 1
        continue

    xp = doc.get("x-powerlab") or {}

    # title: must be {en_us: non-empty string}
    title = xp.get("title")
    if isinstance(title, dict):
        v = title.get("en_us", "")
        if not (isinstance(v, str) and v.strip()):
            errors.append("title.en_us is empty or missing")
    elif isinstance(title, str) and title.strip():
        # Flat string is also acceptable (new format — core handles both)
        pass
    else:
        errors.append(f"title must be {{en_us: str}} or flat string; got {type(title).__name__}")

    # tagline: same rules as title
    tagline = xp.get("tagline")
    if isinstance(tagline, dict):
        v = tagline.get("en_us", "")
        if not (isinstance(v, str) and v.strip()):
            errors.append("tagline.en_us is empty or missing")
    elif isinstance(tagline, str) and tagline.strip():
        pass
    else:
        errors.append(f"tagline must be {{en_us: str}} or flat string; got {type(tagline).__name__}")

    # port_map: non-empty string OR headless: true
    headless = bool(xp.get("headless"))
    pm = xp.get("port_map")
    if not headless:
        if not (isinstance(pm, str) and pm.strip()):
            errors.append(
                f"port_map must be a non-empty string (or headless: true); got {pm!r}"
            )

    # icon: non-empty string (URL or filename)
    icon = xp.get("icon")
    if not (isinstance(icon, str) and icon.strip()):
        errors.append(f"icon must be a non-empty string; got {icon!r}")

    if errors:
        print(f"  FAIL {app_id}:")
        for e in errors:
            print(f"       {e}")
        failures += 1
    else:
        checked += 1

print(f"\nApps checked: {checked} OK, {failures} FAIL")
if failures:
    sys.exit(1)
PYEOF

python_exit=$?
if [[ $python_exit -ne 0 ]]; then
  failures=$((failures + 1))
fi

# Check .curated-manifest is in sync with actual Apps/ directories
if [[ -f "$MANIFEST" ]]; then
  manifest_ids="$(sort "$MANIFEST")"
  disk_ids="$(find "$APPS_DIR" -mindepth 1 -maxdepth 1 -type d -exec basename {} \; | sort)"
  if [[ "$manifest_ids" != "$disk_ids" ]]; then
    echo ""
    echo "FAIL: .curated-manifest is out of sync with Apps/ directories"
    diff <(echo "$manifest_ids") <(echo "$disk_ids") || true
    failures=$((failures + 1))
  else
    echo "OK: .curated-manifest is in sync ($(echo "$disk_ids" | wc -l | tr -d ' ') entries)"
  fi
else
  echo "WARN: .curated-manifest not found — run bundle-store.sh to generate it"
fi

if [[ $failures -gt 0 ]]; then
  echo ""
  echo "FAIL: $failures integrity error(s) in community-catalog/"
  echo "Run ./scripts/bundle-store.sh <store-ref> to regenerate."
  exit 1
fi

echo "OK: community-catalog integrity check passed"
