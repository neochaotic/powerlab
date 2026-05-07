# Local HTTPS Guide (Apple-grade)

This guide explains how to establish a secure, "green lock" connection to your PowerLab instance without using external services or public domains.

> **For implementers and pattern adopters:** the abstract framework
> behind this guide — components, state machine, threat model,
> per-language implementation notes — is documented separately at
> [`docs/patterns/https-trust-onboarding-pattern.md`](patterns/https-trust-onboarding-pattern.md).
> This page is the user-facing instructions; that page is the
> framework spec.

## Overview

PowerLab uses a custom **Internal Root Certificate Authority (CA)** to sign its own certificates. By trusting this Root CA on your devices, you enable encrypted HTTPS connections to `powerlab.local` and the host's LAN address.

The leaf certificate covers every way you reach the box from inside your trust boundary:

| How you connect                | Hostname / IP                                | Trusted? |
|--------------------------------|----------------------------------------------|----------|
| Same Wi-Fi / Ethernet (mDNS)   | `powerlab.local`, `<host>.local`             | ✅ Yes    |
| Same Wi-Fi / Ethernet (LAN IP) | `192.168.x.x`, `10.x.x.x`, `172.16-31.x.x`   | ✅ Yes    |
| Same machine                   | `localhost`, `127.0.0.1`, `::1`              | ✅ Yes    |

The IP-change watcher re-issues the leaf within seconds when the host's bound IP set changes (DHCP renewal, multi-NIC toggle, etc.), so the SAN list stays in sync without any user action.

## Installation Steps

### 📱 iOS (iPhone/iPad)
1. Open PowerLab in **Safari**.
2. Click the **Enable Secure Connection** banner or go to **Settings → Security**.
3. Download the **Security Profile** (`.mobileconfig`).
4. Go to **Settings → Profile Downloaded** and click **Install**.
5. Finally, go to **Settings → General → About → Certificate Trust Settings** and enable full trust for the **PowerLab Root CA**.

### 💻 macOS
1. Download the **Security Profile** from the Security settings.
2. Open the downloaded file. It will be added to **System Settings → Profiles**.
3. Install the profile.
4. Alternatively, download the `.crt` file and add it to **Keychain Access** under the **System** keychain, then set trust to **Always Trust**.

### 🤖 Android
1. Download the **CA Certificate** (`.crt`).
2. Go to **Settings → Security → More Security Settings → Encryption & credentials → Install a certificate → CA certificate**.
3. Select the downloaded file and confirm.

### 🪟 Windows
1. Download the **CA Certificate** (`.crt`).
2. Right-click the file and select **Install Certificate**.
3. Choose **Local Machine**.
4. Select **Place all certificates in the following store**.
5. Click **Browse** and select **Trusted Root Certification Authorities**.
6. Finish the wizard.

## Technical Details

- **Algorithm**: ECDSA P-256 (high security, small footprint).
- **Subject Alternative Names (SAN)**: `powerlab.local`, `<system-hostname>.local`, `localhost`, plus every RFC1918 IPv4 (`10/8`, `172.16/12`, `192.168/16`) and IPv6 ULA (`fc00::/7`) bound to a host interface.
- **HSTS gate**: Strict-Transport-Security is **not** armed until you confirm trust over HTTPS from a non-localhost peer. Prevents the classic lock-out where HSTS ships before the CA is installed.
- **Automatic renewal**: Leaf certificates are rotated 60 days before expiry, or immediately when the host's bound IP set changes (DHCP renewal, multi-NIC toggle).
- **CA download endpoints**: `/v1/sys/ca-certificate` redirects by User-Agent — Apple devices get a signed `.mobileconfig`, Windows gets `.cer` (DER), everyone else gets the raw `.crt`.

## Resetting Trust

If you need to reset the security configuration:
1. Delete the Root CA from your device's trust store.
2. On the server, the HSTS gate can be reset via the management API (Seniors only).
