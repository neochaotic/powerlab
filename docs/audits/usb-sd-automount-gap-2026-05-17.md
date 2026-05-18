# USB/SD auto-mount gap analysis (Sprint 23, #416)

Date: 2026-05-17
Author: Maintainer
Status: Audit. No code changes in this PR — only findings + Phase B-D tracking.

## TL;DR

PowerLab inherited a USB auto-mount surface area from CasaOS that looks
complete on the API + UI toggle layer but is **broken end-to-end on the
runtime layer**. The Go code paths that toggle and execute the mount
unconditionally `source /usr/share/powerlab/shell/local-storage-helper.sh`,
which **does not exist in the repo and does not ship in the tarball**.
Every USB-related operation that depends on it is a silent no-op (errors
get logged, callers ignore the return).

SD card support beyond what udev does on its own has never been wired.

## Headline user requirement (maintainer, 2026-05-17)

**"The mounted disk should appear in Files."** This is the
single-sentence acceptance criterion. Everything in Phase B/C is in
service of this; Phase D #5 (Files page sidebar listing mounted
USB/SD) is the visible deliverable. A Settings toggle that flips
without the Files surface is not "done."

## Inherited surface area (still in the codebase)

### Backend handlers — `backend/local-storage/route/v1/usb.go`

| Route | Handler | What it claims to do |
|---|---|---|
| `GET /v1/sys/usb` | `GetSystemUSBAutoMount` | Read auto-mount toggle state |
| `PUT /v1/sys/usb/off` | `PutSystemUSBAutoMount` | Set auto-mount on/off |
| `GET /v1/disks/usb` | `GetDisksUSBList` | List USB drives (LSBLK with `Tran == "usb"`) |
| `DELETE /v1/disks/usb` | `DeleteDiskUSB` | Unmount a USB by mount point |

### Backend service — `backend/local-storage/service/usb.go`

| Method | Implementation |
|---|---|
| `UpdateUSBAutoMount(state)` | Writes `[server] USBAutoMount` to local-storage.conf. Works (config persists). |
| `ExecUSBAutoMountShell(state)` | `command.OnlyExec("source $ShellPath/local-storage-helper.sh ;USB_Start_Auto"|"USB_Stop_Auto")` — **shell file missing**, silently fails |
| `GetSysInfo()` | gopsutil host info. Works. |
| `GetDeviceTree()` | Reads `/proc/device-tree/model` for SBC detection. Works. |

### Other broken call sites that depend on the missing helper

`backend/local-storage/service/disk.go`:

- L215: `command.ExecResultStr("source ... ;UDEVILUmount " + path)` — used by `DeleteDiskUSB` to actually unmount. **Broken.**
- L501: `command.OnlyExec("source ... ;do_mount " + path + " " + mountPoint)` — used by manual mount endpoints. **Broken.**

### Frontend integration

| Surface | State |
|---|---|
| Files page sidebar showing USB drives | **Not implemented** |
| Settings → Devices → USB toggle UI | **Not implemented** (toggle API exists but no UI calls it) |
| Notification when a USB is plugged | Event bus message `sys_usb` IS published; no UI listener |

### Udev integration

The original CasaOS architecture relied on a udev rule installed by
`USB_Start_Auto` (in the missing helper) to fire on hotplug and invoke a
mount script. With the helper missing, **no udev rule is installed**, so
hot-plugged USB drives never auto-mount on PowerLab. Operators currently
must SSH in and `mount /dev/sdX1 /media/foo`.

SD cards (`/dev/mmcblk*`) — never had upstream support; PowerLab inherits
nothing.

## Why this surfaced now

Sprint 22 ADR-0039 enterprise pivot raised the bar on operator UX. Up
to v0.7.0 nobody had hot-plugged a USB on a PowerLab box and reported
"hey this didn't auto-mount" — the homelab persona did it manually.
Enterprise IT will expect this to Just Work.

## Phase B — runtime audit on .142 (next session)

Before writing any code, validate the static findings against a real
PowerLab v0.7.0 install:

- [ ] SSH to .142, `ls /usr/share/powerlab/shell/` — confirm dir empty
      or missing
- [ ] Plug a USB drive in, watch `journalctl -u powerlab-local-storage -f`
      and `dmesg | tail` — confirm no auto-mount happens
- [ ] Hit `GET /v1/disks/usb` — confirm response shape is still sane
      even though nothing's mounted
- [ ] Test the toggle endpoint: `curl -X PUT .../v1/sys/usb/off -d '{"state":"on"}'` —
      confirm config flips but no udev rule appears in `/etc/udev/rules.d/`

Captures the actual breakage in journal logs for the implementation PR's
"before" state.

## Phase C — backend implementation (new issue, Sprint 24+)

1. **Write `scripts/local-storage-helper.sh`** with the missing functions:
   - `USB_Start_Auto` — install udev rule that calls a mount handler on
     `ACTION=="add" SUBSYSTEM=="block" ENV{ID_BUS}=="usb"`
   - `USB_Stop_Auto` — remove the udev rule
   - `do_mount <device> <mountpoint>` — actual mount call with sensible
     fs detection (`blkid -o value -s TYPE`)
   - `UDEVILUmount <path>` — clean unmount, doesn't error if already gone

2. **Ship the helper in package-linux.sh** — extend the tarball stage
   to install `local-storage-helper.sh` to `/usr/share/powerlab/shell/`.

3. **Replace `command.OnlyExec("source ...")` with direct system
   calls** wherever the helper is purely a wrapper. The Go code already
   has the data; calling out to bash for `mount`/`umount` is legacy
   CasaOS pattern. Where the udev rule MUST be shell (it has to write
   to `/etc/udev/rules.d/`), keep the helper.

4. **Add SD-card branch** — extend the udev rule + `do_mount` to handle
   `SUBSYSTEM=="mmc_host"` / `ENV{ID_DRIVE_FLASH_SD}=="1"`. CasaOS never
   shipped this; PowerLab's enterprise pivot includes Pi/SBC hosts where
   SD is the primary expansion slot.

## Phase D — UI implementation (new issue, Sprint 24+)

5. **Files page sidebar — "Drives" group** below the existing tree:
   - Polls `/v1/disks/usb` every 30s OR subscribes to the `sys_usb`
     event bus message
   - Renders each USB drive + child partition as a clickable entry
     that navigates the Files page into that mount point

6. **Settings → Devices pane**:
   - USB Auto-Mount toggle (calls existing `PUT /v1/sys/usb/off`)
   - List of currently-mounted USB devices with eject button
   - Status indicator when the helper script is missing (defensive
     against a regression of Phase C)

7. **Hotplug toast** — when the sidebar polling detects a new mount,
   show a brief "USB drive 'My Disk' mounted at /media/usb1" toast.

## Phase E — Tests (Sprint 24+)

- Unit: shell helper script's mount/umount with `dd`-backed loop devices
- Integration: Playwright spec mocking the USB endpoints, asserting
  the Files sidebar renders correctly
- E2E: manual checklist on .142 with a real USB drive (no good way to
  automate this in CI without nested virtualization)

## What this PR ships

Nothing executable. Only this audit doc + the tracking issues opened
from it. The implementation cost is high (multi-PR feature work) and
needs real-hardware validation that this doc can't substitute for.

## Issues opened from this PR

(To be filled in after this PR is merged — tracking entries for Phase
B/C/D/E.)
