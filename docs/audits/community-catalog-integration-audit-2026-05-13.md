# Community catalog integration audit — 2026-05-13

Comprehensive audit of the Umbrel community-catalog integration after v0.6.5 ship. Triggered by user-reported install bugs (gitingest "Invalid host header", adventurelog "bad request"). Scans every emitted `community-catalog/Apps/*/docker-compose.yml` for patterns that PowerLab can't handle at install time.

## Method

Python script walks each compose, classifies env vars / volumes / ports / healthchecks by:

1. Remaining `${VAR}` placeholders in env section (Umbrel runtime-substituted vars that stay literal in PowerLab)
2. Multi-service apps without `x-powerlab.main` (UI falls back to alphabetical first → wrong service)
3. External networks referenced but not declared at top level
4. Healthchecks with unsubstituted placeholders

Script committed at `scripts/audit-catalog.py` (one-shot, not part of CI).

## Findings

### 1. Remaining `${VAR}` placeholders in env values

27 unique placeholder names found. Top 10 by occurrence:

| Var | Apps | Class | v0.6.6 action |
|---|---|---|---|
| `${APP_SEED}` | 49 | per-app random secret (Umbrel-runtime) | 🔴 **install-time substitution needed** — defer to v0.6.7 |
| `${APP_PASSWORD}` | 39 | per-app random password | 🔴 same as APP_SEED |
| `${APP_DOMAIN}` | 8 | user's chosen domain (tunnel/onion/LAN) | 🟡 install-time |
| `${APP_GHOSTFOLIO_DB_*}` | 9 (3 vars × 3 svcs) | sibling-app credentials (Ghostfolio) | 🟡 install-time |
| `${APP_<APP>_PORT}` (env) | 4 | port refs (volumes/ports already done) | 🟢 keep (env-side OK) |
| `${TOR_PROXY_*}` | 2 | Tor sibling (dcrpulse) | 🟢 expected to fail (no Tor on PowerLab) |
| `${DEVICE_DOMAIN_NAME}` | 30 | umbrel host name | ✅ **v0.6.6 partial** (pure host-list values only) |
| `${DEVICE_HOSTNAME}` | 12 | umbrel hostname | ✅ same |
| `${APP_*_LOCAL_IPS}` | 3 | sibling-detected LAN IPs | ✅ same |

The host-validation fix (v0.6.6) ONLY substitutes the bottom three when they appear in pure host-list values like `ALLOWED_HOSTS=${DEVICE_DOMAIN_NAME},${DEVICE_HOSTNAME}`. URL-embedded uses (e.g. `ORIGIN=http://${DEVICE_DOMAIN_NAME}:8015` in adventurelog) are LEFT IN PLACE — substituting them produces invalid URLs and breaks SvelteKit/Django-style CSRF/CORS checks.

For URL-embedded cases the install-time substitution layer (Sprint 14) is the right answer: it knows the actual host the user is reaching the app at, and can substitute `${DEVICE_DOMAIN_NAME}` with the right value per-install.

### 2. Multi-svc apps without `x-powerlab.main`

89 apps had no `main` field set. Backend's `compose_app_metadata.go:58` falls back to "first service alphabetically" — wrong for apps like agora (svcs: agora/filebrowser/nginx, real entry is `agora`).

**v0.6.6 fix**: `emit.go` now resolves the main service via app_proxy's `APP_HOST` (same logic that already drives `ports:` synthesis). Falls back to:

1. Service whose name matches the storeAppID (agora → `agora` service)
2. First non-proxy service in alphabetical order (deterministic — Go map iteration is randomized)

Reduced from 89 wrong-defaults to 0.

### 3. External networks

0 apps reference external networks not declared at top level. No action.

### 4. Healthchecks with unsubstituted placeholders

2 apps:
- `dcrpulse/dcrd` — `${APP_SEED}` in basic-auth health probe (will fail until APP_SEED fix lands)
- `kitchenowl/db` — `$${POSTGRES_DB}` is a compose-escaped literal `$POSTGRES_DB`, evaluated by the container, NOT a sync-time placeholder. Actually correct as-is.

## v0.6.6 scope (this PR)

| Item | Status |
|---|---|
| 13.2.1 Loading bar visible from start (#329) | ✅ merged |
| 13.2.2 Launchpad install ghost tile (#330) | ✅ merged |
| 13.4.x gitingest "Invalid host header" (host-validation pure-list) | ✅ included |
| 13.4.x adventurelog "bad request" (URL-embedded preservation) | ✅ included |
| Multi-svc `x-powerlab.main` resolution | ✅ included |
| Deterministic main extraction (no Go-map-iter flakiness) | ✅ included |

## v0.6.7+ scope (Sprint 14)

| Item | Why deferred |
|---|---|
| Install-time `${APP_SEED}` / `${APP_PASSWORD}` random generation | Requires backend (app-management) hook into the install pipeline — bigger refactor than v0.6.6 should carry |
| Install-time `${APP_DOMAIN}` / `${DEVICE_DOMAIN_NAME}` substitution for URL-embedded contexts | Same — needs to know the actual host the user is hitting |
| Sibling-app credential substitution (Ghostfolio DB user/pass) | Needs cross-app coordination at install — not generic enough to fix at sync time |

## What the audit's CI-gate analog already covers

`backend/app-management/service/production_catalog_test.go`:
- `TestProductionCatalog_AllParseThroughComposeLoader` — every YAML must parse through the SAME loader BuildCatalog uses. Catches structural breakage.
- `TestProductionCatalog_NoUnknownPlaceholdersInDangerousPositions` — scans volumes + ports for any `${VAR}` survival. Catches new upstream placeholder kinds.

What the gate does NOT cover (and this audit caught):
- Env-section semantic issues (host validation, URL embedding, secret literals)
- `x-powerlab.main` correctness for multi-svc apps
- Runtime-only failures (Invalid host header, bad request) — these only surface when the app actually runs

Adding a third gate `TestProductionCatalog_EnvSemanticHints` is on the v0.6.7 list — it would walk env vars and emit `t.Logf` warnings (not errors) for known-problematic patterns, helping future audits without breaking CI on every upstream change.
