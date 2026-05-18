#!/usr/bin/env bash
# local-storage-helper.sh — shell helper invoked by powerlab-local-storage
# for USB/SD hot-plug auto-mount, manual mount/umount, and the per-device
# udev rule lifecycle (#416 / #464).
#
# Functions called from Go via `source $ShellPath/local-storage-helper.sh
# ; FUNC args`:
#
#   USB_Start_Auto              install udev rule so hot-plug fires do_mount
#   USB_Stop_Auto               remove udev rule
#   do_mount    <dev> <mount>   mount a block device, auto-detecting fs
#   UDEVILUmount <mount>        unmount + remove the dir, idempotent
#
# Audit doc: docs/audits/usb-sd-automount-gap-2026-05-17.md
# Maps to call sites in backend/local-storage/service/{usb,disk}.go.
#
# Design choices:
#   - Single-file helper (vs N scripts in dir) so the existing Go callers
#     only need `source $ShellPath/local-storage-helper.sh; FUNC args`
#     without per-file path knowledge.
#   - SD card support via the same udev rule branch — ENV{ID_DRIVE_FLASH_SD}=1
#     OR SUBSYSTEM=="mmc_host". CasaOS upstream didn't ship this; we do.
#   - Mount root: /mnt/powerlab/<device-label-or-uuid>. Chosen over
#     /media/ (some distros reserve that for the desktop session) and
#     over /run/media (systemd-private to a user session).
#   - All functions are idempotent — calling twice does not error or
#     leak state. The Go callers swallow errors so loud failure would
#     just mean a journalctl entry the operator has to dig out.

set -uo pipefail

# Mount root for hot-plugged removable devices. Chosen to avoid colliding
# with /media (desktop session reserved on many distros) and /run/media
# (systemd-private).
POWERLAB_MOUNT_ROOT="${POWERLAB_MOUNT_ROOT:-/mnt/powerlab}"

# Path to the udev rule installed by USB_Start_Auto. Single .rules file
# covers USB block devices + SD cards.
POWERLAB_UDEV_RULE="/etc/udev/rules.d/99-powerlab-automount.rules"

log() { echo "[powerlab-helper] $*" >&2; }

# ─── do_mount <dev> <mount_point> ─────────────────────────────────────
#
# Mount the block device at the requested mount point. Auto-detects
# filesystem type via blkid. Mount point is created if missing.
# Idempotent: a second call for an already-mounted dev is a no-op.
#
# Args:
#   $1 — block device (e.g. /dev/sda1, /dev/mmcblk0p1)
#   $2 — absolute mount point (e.g. /mnt/powerlab/my-usb)
do_mount() {
  local dev="${1:-}"
  local mount_point="${2:-}"

  if [[ -z "$dev" || -z "$mount_point" ]]; then
    log "do_mount: missing args (dev=$dev mount=$mount_point)"
    return 2
  fi

  if [[ ! -b "$dev" ]]; then
    log "do_mount: $dev is not a block device"
    return 3
  fi

  # Idempotency: if already mounted at the requested point, exit success
  if mountpoint -q "$mount_point" 2>/dev/null; then
    log "do_mount: $mount_point already mounted, no-op"
    return 0
  fi

  mkdir -p "$mount_point" || {
    log "do_mount: failed to mkdir $mount_point"
    return 4
  }

  # Detect filesystem type. blkid is the canonical source on
  # Linux; util-linux ships it on every distro.
  local fstype
  fstype="$(blkid -o value -s TYPE "$dev" 2>/dev/null)"
  if [[ -z "$fstype" ]]; then
    log "do_mount: could not detect fs type for $dev"
    return 5
  fi

  local mount_opts="rw,noatime,nosuid,nodev"
  # exFAT / NTFS / vFAT mounts default to root-owned; the noauto/exec/dev
  # bits should match — keep the same opts here so the operator's UX is
  # consistent across drives.

  log "do_mount: mounting $dev ($fstype) at $mount_point"
  mount -t "$fstype" -o "$mount_opts" "$dev" "$mount_point"
}

# ─── UDEVILUmount <mount_point> ──────────────────────────────────────
#
# Unmount + remove the mount-point dir. Idempotent — if nothing's
# mounted at the path, just removes the empty dir (if it exists).
# Named with the legacy "UDEVIL" prefix because the Go call site
# expects exactly that name (Go: source ...; UDEVILUmount <path>).
UDEVILUmount() {
  local mount_point="${1:-}"
  if [[ -z "$mount_point" ]]; then
    log "UDEVILUmount: missing mount_point"
    return 2
  fi

  if mountpoint -q "$mount_point" 2>/dev/null; then
    log "UDEVILUmount: unmounting $mount_point"
    if ! umount "$mount_point"; then
      log "UDEVILUmount: clean umount failed, trying lazy"
      umount -l "$mount_point" || {
        log "UDEVILUmount: lazy umount failed for $mount_point"
        return 3
      }
    fi
  fi

  # Remove the dir only if it's empty + under POWERLAB_MOUNT_ROOT
  # (defence: never rmdir / by accident if mount_point was garbage)
  if [[ "$mount_point" == "$POWERLAB_MOUNT_ROOT"/* ]]; then
    rmdir "$mount_point" 2>/dev/null || true
  fi
  return 0
}

# ─── USB_Start_Auto ──────────────────────────────────────────────────
#
# Install the udev rule that fires do_mount() on hot-plug. Rule covers:
#   - USB block devices: SUBSYSTEM=="block" ENV{ID_BUS}=="usb"
#   - SD cards: SUBSYSTEM=="block" ENV{ID_DRIVE_FLASH_SD}=="1"
#     (some kernels also fire as SUBSYSTEM=="mmc_host" but the block
#     subsystem is what produces partition devices we can mount)
#
# Idempotent: writes the rule file (overwriting), then reloads udev.
USB_Start_Auto() {
  log "USB_Start_Auto: installing udev rule at $POWERLAB_UDEV_RULE"

  mkdir -p "$(dirname "$POWERLAB_UDEV_RULE")" "$POWERLAB_MOUNT_ROOT"

  cat > "$POWERLAB_UDEV_RULE" <<'EOF'
# PowerLab USB / SD auto-mount rule (installed by USB_Start_Auto, #416).
# Fires on hot-plug + partition-add for USB block devices and SD cards.
# Calls /usr/share/powerlab/shell/local-storage-helper.sh do_mount
# with the per-partition device + a stable mount point under
# /mnt/powerlab/.
#
# Mount point name: prefer ID_FS_LABEL (operator-visible), fall back
# to ID_FS_UUID (stable across plug events). Both come from blkid via
# udev's built-in env.

ACTION=="add", SUBSYSTEM=="block", ENV{ID_BUS}=="usb", ENV{ID_FS_TYPE}!="", \
    RUN+="/bin/bash -c '/bin/bash /usr/share/powerlab/shell/local-storage-helper.sh do_mount /dev/%k /mnt/powerlab/${ENV{ID_FS_LABEL:-%k}}'"

ACTION=="add", SUBSYSTEM=="block", ENV{ID_DRIVE_FLASH_SD}=="1", ENV{ID_FS_TYPE}!="", \
    RUN+="/bin/bash -c '/bin/bash /usr/share/powerlab/shell/local-storage-helper.sh do_mount /dev/%k /mnt/powerlab/${ENV{ID_FS_LABEL:-%k}}'"

ACTION=="remove", SUBSYSTEM=="block", ENV{ID_BUS}=="usb", \
    RUN+="/bin/bash -c '/bin/bash /usr/share/powerlab/shell/local-storage-helper.sh UDEVILUmount /mnt/powerlab/${ENV{ID_FS_LABEL:-%k}}'"

ACTION=="remove", SUBSYSTEM=="block", ENV{ID_DRIVE_FLASH_SD}=="1", \
    RUN+="/bin/bash -c '/bin/bash /usr/share/powerlab/shell/local-storage-helper.sh UDEVILUmount /mnt/powerlab/${ENV{ID_FS_LABEL:-%k}}'"
EOF

  # Reload udev so the rule takes effect without a host reboot.
  if command -v udevadm >/dev/null 2>&1; then
    udevadm control --reload-rules 2>/dev/null || log "USB_Start_Auto: udevadm reload failed (non-fatal)"
    udevadm trigger --subsystem-match=block 2>/dev/null || log "USB_Start_Auto: udevadm trigger failed (non-fatal)"
  else
    log "USB_Start_Auto: udevadm not on PATH — rule installed but won't fire until reboot"
  fi
  return 0
}

# ─── USB_Stop_Auto ───────────────────────────────────────────────────
#
# Remove the udev rule. Hot-plugged devices stop auto-mounting.
# Already-mounted devices stay mounted (no force-umount — operator's
# files might be open). Operator can umount via UDEVILUmount.
USB_Stop_Auto() {
  log "USB_Stop_Auto: removing udev rule"
  rm -f "$POWERLAB_UDEV_RULE"
  if command -v udevadm >/dev/null 2>&1; then
    udevadm control --reload-rules 2>/dev/null || log "USB_Stop_Auto: udevadm reload failed (non-fatal)"
  fi
  return 0
}
