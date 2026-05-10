# Security model

This page summarizes PowerLab's security posture: how passwords are stored, how sessions survive restarts, how HTTPS is bootstrapped, and what an attacker with disk access can actually do. The full rationale for each piece lives in a dedicated ADR — links throughout.

The throughline: PowerLab's primary user is a self-hosted home-server operator. Defaults match that threat model. Higher-trust deployments can opt into stricter postures explicitly, but the out-of-the-box experience optimizes for the home-lab case.

## Authentication

### Password storage

User passwords are hashed with **bcrypt** before they ever touch disk. The hash lives in `o_users.password` in the user-service SQLite database (`/var/lib/powerlab/db/user.db` on Linux; the dev path under `runtimePath/db/` on macOS dev).

Two authentication paths can verify a login:

- **OS-native (PAM on Linux, dscl on macOS)** when the user-service was built with `CGO_ENABLED=1` and `libpam0g-dev` is present at runtime. This is the production posture on amd64 Linux.
- **Bcrypt SetupWizard fallback** when OS-native is unavailable (no-CGO build, arm64 today, or PAM lookup fails). The bcrypt path is what the SetupWizard creates on first boot, and it is also what every deployment falls back to if PAM is misconfigured.

The auth code is tagged-build separated: `auth_os_pam_linux.go`, `auth_os_pam_stub.go`, `auth_os.go`. A no-CGO build still compiles and runs; it just authenticates exclusively via the bcrypt fallback.

### JWT signing keypair

The user-service signs JWT cookies with an ECDSA keypair. Since v0.5.7 ([ADR-0020](../decisions/0020-jwt-keypair-persisted-by-default.md)), this keypair **persists in the user-service database** by default. Restarts and in-app upgrades no longer invalidate outstanding cookies.

Operators who need the prior behavior (fresh keypair per startup, never persisted) can opt in via the `POWERLAB_EPHEMERAL_JWT_KEY=true` environment variable. ADR-0020 covers the threat-model trade-off in detail and is worth reading before flipping the switch.

The persisted keypair lives in a single-row table (`jwt_keypair`, `id INTEGER PRIMARY KEY CHECK (id = 1)`). On first boot the row is absent; the service generates and writes a fresh keypair. Manually deleting the row forces rotation on next startup.

## HTTPS

PowerLab ships a self-signed local CA and a leaf certificate covering every way you reach the host from inside your trust boundary (mDNS name, LAN IPs, localhost). The trust onboarding flow is documented as a portable pattern in [`docs/patterns/https-trust-onboarding-pattern.md`](../patterns/https-trust-onboarding-pattern.md); the user-facing instructions are in [`docs/HTTPS.md`](../HTTPS.md).

### Default-disabled in v0.5.x

HTTPS is **disabled by default in the v0.5.x line** per a v0.5.2 polish (issue #130). The rationale: first-time users were getting tripped up by browser cert warnings before they had a chance to install the CA via the in-UI walkthrough. Re-enable HTTPS explicitly in `/etc/powerlab/gateway.ini` once you've completed the trust dance, or wait for the v0.5.x polish issues (#101, #106, #104, #118) to land before defaulting on again.

[ADR-0007 (internal-network-only initial deployment)](../decisions/0007-internal-network-only-initial-deployment.md) frames the broader posture: PowerLab assumes a LAN trust boundary out of the box. The v0.5.2 disable is consistent with that — HTTPS is a hardening step, not a prerequisite for safe local use.

### Trust dance

When HTTPS is enabled, getting the green-lock state on a client requires:

1. The host's CA cert is delivered to the client (`.mobileconfig` for Apple, `.crt` for everything else, served from `/v1/sys/ca-certificate` with UA-aware redirect).
2. The client installs the CA into its trust store.
3. The client confirms trust by issuing `POST /v1/sys/trust-confirmed` over HTTPS from a non-localhost address. This is the gate that arms HSTS — until the gate fires, the gateway does NOT send HSTS, so a misconfigured trust install can never lock the operator out.

The full state machine and the per-platform install steps are in the pattern doc and `docs/HTTPS.md`. [ADR-0009 (trust onboarding pattern)](../decisions/0009-https-trust-onboarding-pattern.md) names the pattern and places it in the public domain; the load-bearing per-decision ADRs are 0001–0007.

## Threat model: what can an attacker with disk access do?

This is the question [ADR-0020](../decisions/0020-jwt-keypair-persisted-by-default.md) had to confront when persisting the JWT keypair. The honest answer:

An attacker who can read the contents of `/var/lib/powerlab/` already has:

- **Bcrypt password hashes** in `user.db` (offline crack possible; difficulty depends on password strength).
- **Every config file under `/etc/powerlab/`** — DB paths, secrets, environment.
- **Container app data under `/var/lib/powerlab/apps/`** — often raw credentials in compose env vars (an inherited Docker norm; not a PowerLab-specific weakness).
- **The persisted JWT signing key** since v0.5.7 — they can forge tokens for any user.
- **The ability to install backdoors** in the binary or the systemd unit and wait for the operator to power the host back on.

JWT forge is a microscopic incremental capability against an attacker who already has all of the above. ADR-0020's reasoning was: the cost of NOT persisting the key (every user logged out on every upgrade) is large and certain; the security gain is contingent on a threat model the typical PowerLab operator does not have.

**For higher-trust deployments**, layer in:

- Disk encryption at rest (`/var/lib/powerlab` on a LUKS volume).
- Backups to a destination with a different trust boundary than the source disk.
- `POWERLAB_EPHEMERAL_JWT_KEY=true` to revert to per-restart key rotation.
- Strong host-level access controls (no shared accounts, no plaintext SSH keys near `/var/lib/powerlab`).

## To expand

- Refresh-token flow for mobile clients — tracked as a follow-up in ADR-0020's "alternatives considered".
- A dedicated threat-model document covering network-borne attackers, casual physical access, and malicious local users — pending the v0.6 security pass.
- Multi-tenant deployment story — currently undefined; ADR-0020's defaults assume single-operator.

For each gap, file an issue under the docs site polish series so the gap is tracked rather than implicitly deferred.
