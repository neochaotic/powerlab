# Supported platforms

This document lists the operating systems and authentication paths PowerLab
supports today, what the upgrade path looks like, and what is explicitly
out of scope.

## Operating systems

| Platform                     | Architecture       | Status      | Notes |
|------------------------------|--------------------|-------------|-------|
| **Ubuntu** 22.04 / 24.04 LTS | `amd64`, `arm64`   | ✅ Supported | tarball install via `install.sh`, native PAM auth |
| **Ubuntu** 20.04 LTS         | `amd64`, `arm64`   | ✅ Supported | systemd + Docker + libpam0g required |
| **Debian** 11 / 12           | `amd64`, `arm64`   | ✅ Supported | covers Raspberry Pi OS (Debian-based) |
| **Raspberry Pi OS** (Bookworm / Bullseye) | `arm64` | ✅ Supported | tested on Raspberry Pi 4 / 5 with 4 GB+ RAM |
| **Fedora** 38+               | `amd64`, `arm64`   | ⚠️ Untested | should work — same systemd + Docker base; please open an issue with results |
| **Arch Linux**               | `amd64`            | ⚠️ Untested | same as Fedora — likely works |
| **Alpine** 3.18+             | `amd64`, `arm64`   | ❌ Not supported | uses musl libc + OpenRC, not glibc/systemd. Out of scope for v0.1.x |
| **macOS** Sonoma / Sequoia / Tahoe | `arm64` (Apple Silicon) | ✅ Dev mode only | run via `./dev.sh`; not packaged for distribution |
| **Windows**                  | any                | ❌ Not planned | |

## Authentication

PowerLab signs you in with your **operating-system credentials** so you do
not have a separate "panel account" to forget. The implementation depends
on the host platform:

| Platform | Mechanism | Status |
|---|---|---|
| **macOS**   | `dscl . -authonly` against the local Directory Service | ✅ Working |
| **Linux** (`amd64`) | PAM via `libpam` (CGO)                          | ✅ Working (v0.2+). Sign in with your `useradd` password. |
| **Linux** (`arm64`) | bcrypt SetupWizard fallback                     | ⚠️ PAM not yet in arm64 release tarball — open as follow-up. SetupWizard works; OS auth lands in v0.2.x. |
| **Windows** | LSA / SSPI                                             | ❌ Not planned |

### How Linux auth works in v0.2+

PowerLab installs a minimal PAM service file at `/etc/pam.d/powerlab`
(written by `install.sh` only when absent — admins can edit it to add
2FA, faillock, etc., and upgrades will leave the file alone). The
service runs `pam_unix.so` against `/etc/shadow`, which means PowerLab
honours the host's hashing algorithm (yescrypt on Ubuntu 22.04+,
SHA-512 on older distros, bcrypt on FreeBSD-style configs, ...) and
the host's account-validity rules (locked accounts via `usermod -L`,
expired passwords, etc.) — all delegated to libxcrypt at runtime, not
re-implemented in Go.

The first time a user signs in, PowerLab mirrors the OS account into
its local database (`/var/lib/powerlab/db`) so it has a stable
`user.id` to mint JWTs against. The local DB never stores the
password; the source-of-truth stays in `/etc/shadow`.

### Bcrypt SetupWizard fallback

If PAM is unavailable on a host (CGO disabled at compile time, missing
libpam, custom PAM config that breaks `pam_unix`), the login screen
falls back to a one-shot Setup Wizard that registers a bcrypt password
in PowerLab's own DB. That password is then accepted by the same
login form. The fallback is also useful as a recovery path when the
admin mis-edits `/etc/pam.d/powerlab`.

### Why PAM via CGO and not a shell-out

We considered three shell-out alternatives before committing to CGO:

- `unix_chkpwd` (the PAM helper) silently returns exit 0 for invalid
  passwords when called outside `pam_unix` — a security feature for
  `pam_unix`'s own use, but a footgun for direct callers (verified
  empirically on Ubuntu 22.04). Using it as a stand-in for PAM
  creates a password-bypass.
- `mkpasswd` (from the `whois` Debian package) does not support
  yescrypt, the default hash algorithm on Ubuntu 22.04+ and Debian
  12+, so any host running those distros would silently lose support.
- `su -c true` cannot take a password on stdin without an external
  `expect`-style wrapper, which itself becomes a dependency we'd
  have to ship.

`pam_authenticate` via CGO + `libpam` is the path every other serious
Linux panel takes (Cockpit, Webmin, Wazuh) and it lets libxcrypt at
runtime worry about which hash algorithm the host uses — instead of
us trying to vendor every one. PAM is in PowerLab as of **v0.2.0**.

## Hardware

| Tier                       | Verdict | Notes |
|----------------------------|---------|-------|
| **Raspberry Pi Zero 2 W (512 MB)** | ⚠️ Tight | core panel runs; AI / heavy Docker apps will OOM. Use as a media server only. |
| **Raspberry Pi 4 / 5 (4 GB+)**     | ✅ Recommended for AI-light setups | Ollama with 7B-Q4 GGUFs runs at usable speed |
| **N100 / Intel Mini PC (8–16 GB)** | ✅ Sweet spot for general home server | |
| **GPU rig / Nvidia / Apple Silicon** | ✅ Best for AI-heavy setups | GPU auto-detected and surfaced on Dashboard |

## Docker

PowerLab requires Docker Engine on the host. Tested with:

- `docker.io` (Ubuntu/Debian apt) — versions 20.10+ through 27.x
- Docker CE (official `get.docker.com` install)
- Docker Desktop (development on macOS only)

Docker API version 1.44 is forced via `DOCKER_API_VERSION=1.44` environment
variable in the systemd units, so the v24 SDK we use can talk to modern
v25+ daemons.

## Filesystems

The local-storage service uses **mergerfs** + **fuse** for unified storage
across multiple drives. This is Linux-only and is the reason the Files
page is not available in the macOS dev experience.

## Reporting compatibility

If PowerLab works on a distro / hardware combination not listed here,
please open an issue on
[github.com/neochaotic/powerlab/issues](https://github.com/neochaotic/powerlab/issues)
with:

- distro + version (`cat /etc/os-release`)
- architecture (`uname -m`)
- kernel (`uname -r`)
- the install command you ran

We will add it to this matrix.
