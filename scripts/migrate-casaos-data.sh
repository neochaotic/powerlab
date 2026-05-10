#!/usr/bin/env bash
# Migrate /var/lib/casaos/* → /var/lib/powerlab/* on upgrade.
#
# Why this exists: PR #140 (v0.5.4) flipped service data paths but
# install.sh forgot to migrate existing data. A user upgrading
# v0.5.x → v0.5.4 ended up with user-service reading an empty
# /var/lib/powerlab/, login returning 400, UI unusable.
#
# v0.5.7 follow-up (issue #179): the original hot-fix copied
# user.db + local-storage.db to /var/lib/powerlab/db/, but those
# services don't actually use the /db/ subdir — they read from
# /var/lib/powerlab/<svc>.db directly. The mistake created stale
# duplicates that produced split-brain risk on subsequent upgrades.
# This script now uses the canonical destination paths from
# backend/common/utils/paths/db.go.
#
# Per-service path map (kept in sync with docs/audits/db-paths.md):
#
#   casaos source                          powerlab destination
#   ─────────────────────────────────────  ───────────────────────────────
#   /var/lib/casaos/user.db                /var/lib/powerlab/user.db
#   /var/lib/casaos/local-storage.db       /var/lib/powerlab/local-storage.db
#   /var/lib/casaos/local-storage.json     /var/lib/powerlab/local-storage.json
#   /var/lib/casaos/db/casaOS.db           /var/lib/powerlab/db/casaOS.db
#   /var/lib/casaos/db/message-bus.db      /var/lib/powerlab/db/message-bus.db
#
# (core + message-bus still use the /db/ subdir in their service code as
# of v0.5.7; migration to canonical /var/lib/powerlab/<svc>.db is a
# separate PR per docs/audits/db-paths.md.)
#
# Sourced by install.sh (which calls migrate_casaos_data). Also
# directly invokable for testing — see scripts/migrate-casaos-data_test.sh.
#
# PREFIX env var lets the regression test point the migration at a
# sandbox root instead of the real /var/lib/. Production install.sh
# leaves it unset → migration runs against /var/lib/.
#
# Idempotent. Safe to run on every install. No-op on fresh hosts.
# Source files (/var/lib/casaos/*) are preserved — never deleted —
# so a sysadmin can manually roll back if anything goes sideways.
# The boot-time split-brain check (paths.AssertNoSplitBrain in each
# service's main.go) is the safety net if any future migration
# accidentally writes to two paths.

migrate_casaos_data() {
  local prefix="${PREFIX:-}"
  local casaos="$prefix/var/lib/casaos"
  local powerlab="$prefix/var/lib/powerlab"

  if [[ ! -d "$casaos" ]]; then
    return 0
  fi

  install -d -m 0755 "$powerlab"

  # Subdirectory copy. Skip when destination already exists —
  # never overwrite live data. apps/appstore/conf are the bulk
  # data; db/ is the catch-all source for the file-level loop below.
  local sub src dst
  for sub in db apps appstore conf 1; do
    src="$casaos/$sub"
    dst="$powerlab/$sub"
    if [[ -d "$src" ]] && [[ ! -e "$dst" ]]; then
      echo "[powerlab-install] migrating $src → $dst"
      cp -a "$src" "$dst"
    fi
  done

  # File-level copy with explicit src→dst mapping. The pattern is
  # "src_relative_to_casaos|dst_relative_to_powerlab" so each entry
  # documents the per-service destination convention.
  local entry s d
  for entry in \
      "user.db|user.db" \
      "local-storage.db|local-storage.db" \
      "local-storage.json|local-storage.json" \
      "db/casaOS.db|db/casaOS.db" \
      "db/message-bus.db|db/message-bus.db"; do
    s="$casaos/${entry%%|*}"
    d="$powerlab/${entry##*|}"
    if [[ -f "$s" ]] && [[ ! -f "$d" ]]; then
      echo "[powerlab-install] migrating file $s → $d"
      install -d -m 0755 "$(dirname "$d")"
      cp -a "$s" "$d"
    fi
  done
}

# When invoked directly (not sourced), run the migration immediately.
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  migrate_casaos_data
fi
