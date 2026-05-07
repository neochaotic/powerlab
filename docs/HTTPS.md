# Local HTTPS Guide (Apple-grade)

This guide explains how to establish a secure, "green lock" connection to your PowerLab instance without using external services or public domains.

## Overview

PowerLab uses a custom **Internal Root Certificate Authority (CA)** to sign its own certificates. By trusting this Root CA on your devices, you enable encrypted HTTPS connections to `powerlab.local` and its local IP address.

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

- **Algorithm**: ECDSA P-256 (High security, small footprint).
- **Subject Alternative Names (SAN)**: Includes `powerlab.local`, `localhost`, and all local IPv4/IPv6 addresses.
- **HSTS**: Once trust is confirmed, the server enables Strict-Transport-Security (HSTS) to prevent accidental fallback to unencrypted HTTP.
- **Automatic Renewal**: Server certificates are automatically rotated 60 days before expiry or immediately upon IP change.

## Resetting Trust

If you need to reset the security configuration:
1. Delete the Root CA from your device's trust store.
2. On the server, the HSTS gate can be reset via the management API (Seniors only).
