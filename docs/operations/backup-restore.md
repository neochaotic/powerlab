# Backup and restore

PowerLab snapshots its install state automatically on every upgrade, and exposes a one-shot rollback command for the case where an upgrade misbehaves. This page covers what is in those snapshots, where AppData lives (which is NOT in the snapshots), how to take a snapshot manually, and how to roll back.

For the upgrade flow itself — how the in-app updater triggers `install.sh --upgrade` and where pre-install checks happen — see [Updating](../getting-started/updating.md).

## What `install.sh --upgrade` snapshots

Every upgrade run creates a snapshot directory under `/var/lib/powerlab/backups/` named `pre-upgrade-<TIMESTAMP>/`. The snapshot is taken BEFORE binaries are swapped, so a failed upgrade has a known-good state to restore.

Contents of each snapshot:

| Path snapshotted | Why |
|---|---|
| `/etc/powerlab/` | All config files, the security CA + leaf, the version stamp, the HSTS gate file |
| `/var/lib/powerlab/db/` | Every per-service SQLite database (user, app-management, etc.) |
| `/usr/bin/powerlab-*` | The service binaries themselves, so a rollback is byte-exact |
| `/etc/systemd/system/powerlab-*.service` | The systemd units in case unit-shape changed between versions |

Snapshots are pruned by an age-based retention policy — by default the **last 3 snapshots** are kept. Older ones are removed during the upgrade flow itself, before the new snapshot is written. The retention check is exercised by `scripts/check-backup-retention_test.sh`.

The full list of paths and their preservation guarantees is the source of truth in [data persistence](../architecture/data-persistence.md).

## What is NOT in the snapshot

**App data is intentionally excluded.** The per-app persistent volumes — `/DATA/AppData/<app>/` for legacy apps and `/DATA/PowerLabAppData/<app>/` for canonical (post-Sprint-4) apps — are bind-mounted into containers and are not part of the install state. They are also typically MUCH larger than the install state (databases, media libraries, etc.) and snapshotting them on every upgrade would be both slow and disk-expensive.

If you want app data backed up, treat it as an independent concern: rsync `/DATA/` to remote storage, snapshot the underlying filesystem (ZFS, Btrfs), or use the per-app backup features that some apps ship with (e.g. Nextcloud's own export).

The two AppData paths and the rationale for the split are documented in the [coexistence overview](../coexistence/README.md) and [ADR-0021](../decisions/0021-docker-label-namespace-and-appdata-path.md).

## Taking a manual snapshot

The simplest way is to invoke the same code path the upgrader uses, with no actual upgrade behind it:

```bash
sudo install.sh --snapshot
```

This writes a fresh `pre-upgrade-<TIMESTAMP>/` under `/var/lib/powerlab/backups/` and rotates older ones per the retention policy. Useful before:

- A risky config change you want a guaranteed point-in-time fallback for.
- A manual schema migration outside the upgrade flow.
- Trying out something invasive and wanting an "oops" button.

If the `--snapshot` flag is not yet wired in your installed version (introduced as a polish in v0.5.x), the same effect can be reproduced by hand:

```bash
sudo bash -c '
  TS=$(date -u +%Y%m%dT%H%M%SZ)
  DEST=/var/lib/powerlab/backups/pre-upgrade-$TS
  mkdir -p "$DEST"
  cp -a /etc/powerlab "$DEST/etc-powerlab"
  cp -a /var/lib/powerlab/db "$DEST/var-lib-db"
  cp -a /usr/bin/powerlab-* "$DEST/"
  cp -a /etc/systemd/system/powerlab-*.service "$DEST/"
  echo "Snapshot written to $DEST"
'
```

Mirror that to a remote destination if disk loss is the failure mode you are guarding against.

## Restoring a snapshot

The supported path is the bundled `powerlab-rollback` helper:

```bash
sudo /usr/bin/powerlab-rollback
```

Without arguments, this restores the **most recent snapshot** under `/var/lib/powerlab/backups/`, restarting services in the correct boot order. The helper is what the in-app updater invokes automatically when an upgrade health check fails — see step 7 of the upgrade flow in [data persistence](../architecture/data-persistence.md#upgrade-flow-preservation-guarantees).

To restore a specific older snapshot:

```bash
sudo /usr/bin/powerlab-rollback /var/lib/powerlab/backups/pre-upgrade-20260301T143052Z
```

Cross-link: [Updating](../getting-started/updating.md#rollback) walks through the typical "the upgrade looked fine but the UI is misbehaving" recovery flow end to end.

## Disaster scenarios

| Scenario | Recovery |
|---|---|
| Upgrade health check failed automatically | The updater already rolled back. No action needed. Check `/var/lib/powerlab/last-upgrade.json` for the failure reason. |
| Upgrade succeeded, UI is broken hours later | `sudo /usr/bin/powerlab-rollback` restores the last `pre-upgrade-*` snapshot. |
| Lost the entire `/var/lib/powerlab/db/` directory | Restore from the latest snapshot. If snapshots are also gone, you reinstall + re-onboard (the SetupWizard re-runs). App data on `/DATA/` is untouched and re-attaches when apps are reinstalled. |
| Lost the entire host disk | Restore `/etc/powerlab/`, `/var/lib/powerlab/`, and `/DATA/` from your off-host backups. PowerLab reinstall + restoring those three trees is the supported full-recovery path. |

## To expand

- An off-host backup recipe (rsync target + cron) once the v0.6 backup-config UI lands.
- A guided restore-to-different-host walkthrough for migration scenarios.

Track gaps under the docs site polish issue series.
