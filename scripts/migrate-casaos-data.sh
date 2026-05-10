#!/usr/bin/env bash
# Migrate /var/lib/casaos/* → /var/lib/powerlab/* on upgrade.
#
# Why this exists: PR #140 (v0.5.4) flipped service data paths but
# install.sh forgot to migrate existing data. A user upgrading
# v0.5.x → v0.5.4 ended up with user-service reading an empty
# /var/lib/powerlab/db/, login returning 400, UI unusable.
# Hot-fixed manually by copying DBs; permanent fix is this script.
# See issue #158.
#
# Sourced by install.sh (which calls migrate_casaos_data). Also
# directly invokable for testing — see scripts/migrate-casaos-data_test.sh.
#
# PREFIX env var lets the regression test point the migration at a
# sandbox root instead of the real /var/lib/. Production install.sh
# leaves it unset → migration runs against /var/lib/.
#
# Idempotent. Safe to run on every install. No-op on fresh hosts.
# Source dirs (/var/lib/casaos/*) are preserved — never deleted —
# so a sysadmin can manually roll back if anything goes sideways.

migrate_casaos_data() {
  local prefix="${PREFIX:-}"
  local casaos="$prefix/var/lib/casaos"
  local powerlab="$prefix/var/lib/powerlab"

  if [[ ! -d "$casaos" ]]; then
    return 0
  fi

  install -d -m 0755 "$powerlab"

  # Per-subdirectory copy. Skip when destination already exists —
  # never overwrite live data.
  local sub src dst
  for sub in db apps appstore conf 1; do
    src="$casaos/$sub"
    dst="$powerlab/$sub"
    if [[ -d "$src" ]] && [[ ! -e "$dst" ]]; then
      echo "[powerlab-install] migrating $src → $dst"
      cp -a "$src" "$dst"
    fi
  done

  # File-level catch for cases where the destination dir EXISTS
  # (e.g. message-bus already created /var/lib/powerlab/db/) but
  # specific files are missing. Without this loop, the user.db
  # critical to login would be left behind.
  local f
  for f in db/message-bus.db db/casaOS.db db/user.db db/local-storage.db db/local-storage.json; do
    src="$casaos/$f"
    dst="$powerlab/$f"
    if [[ -f "$src" ]] && [[ ! -f "$dst" ]]; then
      echo "[powerlab-install] migrating file $src → $dst"
      install -d -m 0755 "$(dirname "$dst")"
      cp -a "$src" "$dst"
    fi
  done
}

# When invoked directly (not sourced), run the migration immediately.
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  migrate_casaos_data
fi
