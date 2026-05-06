# Supported platforms

This document lists the operating systems and authentication paths PowerLab
supports today, what the upgrade path looks like, and what is explicitly
out of scope.

## Operating systems

| Platform                     | Architecture       | Status      | Notes |
|------------------------------|--------------------|-------------|-------|
| **Ubuntu** 22.04 / 24.04 LTS | `amd64`, `arm64`   | ✅ Supported | tarball install via `install.sh` |
| **Ubuntu** 20.04 LTS         | `amd64`, `arm64`   | ✅ Supported | systemd + Docker required |
| **Debian** 11 / 12           | `amd64`, `arm64`   | ✅ Supported | covers Raspberry Pi OS (which is Debian-based) |
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
| **Linux**   | PAM via `libpam` (CGO)                                 | 🛠 Roadmap (v0.2). For now, bcrypt fallback via SetupWizard. |
| **Windows** | LSA / SSPI                                             | ❌ Not planned |

### How the bcrypt fallback works on Linux today

On the very first run on a Linux host, PowerLab shows a short **Setup
Wizard** asking you to register a username + password. That password is
stored as a bcrypt hash in PowerLab's own database (`/var/lib/powerlab/db`)
and is what you use to sign in until native PAM auth lands in v0.2.

The Setup Wizard is shown only once; subsequent visits go straight to
the regular Login screen.

### Why we are not using `unix_chkpwd` or `mkpasswd`

We deliberately **do not** shell out to either of these:

- `unix_chkpwd` (the PAM helper) silently returns exit 0 for invalid
  passwords when called outside `pam_unix` — a security feature for
  `pam_unix`'s own use, but a footgun for direct callers (verified
  empirically on Ubuntu 22.04). Using it as a stand-in for PAM creates a
  password-bypass vulnerability.
- `mkpasswd` (from the `whois` Debian package) does not support
  yescrypt, the default hash algorithm on Ubuntu 22.04+ and Debian 12+.

The clean answer is `pam_authenticate` via CGO. We are deferring it to
v0.2 because adding CGO to the cross-compile pipeline is a non-trivial
change and we wanted v0.1 to install cleanly on every Linux that has
PAM (rather than only the ones we have build coverage for).

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
