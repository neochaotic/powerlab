# Catalog overlap audit — Umbrel × PowerLab existing sources

**Date:** 2026-05-11
**Author:** Phase 0 of [umbrel-catalog umbrella #307](https://github.com/neochaotic/powerlab/issues/307)
**Scope:** measure the gain (and the cost) of importing the Umbrel App Store catalog into PowerLab's `community-catalog/`, validate the ADR-0024 filter accuracy against real upstream YAMLs, lock the icon URL pattern, and write the Phase 1 priorities.

This is a **read-only audit**. No code, no catalog change. The sync binary lands in Phase 1 (umbrella #307).

## Headline numbers

| Source | App count | Format |
|---|---:|---|
| **PowerLab — CasaOS-AppStore** (`backend/data/appstore/cdn.jsdelivr.net/.../default.new/Apps/`) | 162 | Pascal-case IDs (`2FAuth`, `AdGuardHome`, …) |
| **PowerLab — Big-Bear** (`backend/data/appstore/github.com/.../big-bear-casaos-master/Apps/`) | 229 | kebab-case IDs (`2fauth`, `adguard-home`, …) |
| **Umbrel — `getumbrel/umbrel-apps`** | 330 | kebab-case IDs |

After normalising to lower-case + collapsing trivial naming variants:

| Set | Count | Notes |
|---|---:|---|
| Umbrel ∩ CasaOS | 62 | Apps that exist in both, Umbrel wins per the dedup priority decided in #307 |
| Umbrel ∩ Big-Bear | 54 | Same — Umbrel wins |
| Umbrel-only (NEW gain) | **235** | Apps PowerLab does not currently catalogue at all |
| CasaOS + Big-Bear minus Umbrel-overlap | ~301 | Stays in PowerLab catalogue as before |

**Net effect of full Umbrel import (after the 4-tier filter):** **~500+ apps** total catalogue assuming half the 235 Umbrel-only pass Tier 4. Today's catalogue is **~391 apps with CasaOS+Big-Bear redundancy**; the dedup priority means the actually-shown catalogue today is closer to **~330 unique app IDs**.

### Top-10 overlap examples (Umbrel ∩ CasaOS)

`albyhub`, `audiobookshelf`, `autobrr`, `bazarr`, `blinko`, `calibre-web`, `cloudflared`, `convertx`, `copyparty`, `databag` — these are exactly the apps where Umbrel's curation buys us the most: same app, but Umbrel's docker-compose is the actively maintained one.

### Top-10 overlap examples (Umbrel ∩ Big-Bear)

`adguard-home`, `appsmith`, `audiobookshelf`, `budibase`, `chromium`, `code-server`, `convertx`, `dockge`, `docmost`, `excalidraw` — same logic, against the Big-Bear set.

## ADR-0024 filter accuracy — spot-check on 10 real apps

Pulled `docker-compose.yml` for each via `gh api repos/getumbrel/umbrel-apps/contents/<app>/docker-compose.yml` and inspected for the Tier 1 / Tier 2 signatures.

### Tier 1 (HARD REJECT) — confirmed matches

| App | Trigger | Filter rule |
|---|---|---|
| `bitcoin` | `image: ghcr.io/getumbrel/umbrel-bitcoin:v1.2.2` | `image:` from `getumbrel/*` |
| `lightning` | `image: getumbrel/umbrel-lightning:v1.2.2` + uses `${APP_BITCOIN_NODE_IP}`, `${APP_BITCOIN_RPC_PORT}`, etc. | `getumbrel/*` image + cross-app sibling env vars |
| `electrs` | `image: getumbrel/umbrel-electrs:v1.0.4` + uses `${APP_BITCOIN_*}` env vars + has its own `${APP_ELECTRS_*}` siblings | Same — image + cross-app deps |

The filter spec from ADR-0024 catches all three with the literal rules as written. No spec adjustment needed.

### Tier 4 (ALLOW) — confirmed safe

| App | Image source | Notes |
|---|---|---|
| `nginx-proxy-manager` | `jc21/nginx-proxy-manager` | Third-party publisher; self-contained compose with internal `qoomon/docker-host` helper service. No cross-app deps. |
| `nextcloud` | `nextcloud:apache` + internal `mariadb`, `redis` | All internal services within the same compose project. Not Tier 1. |
| `jellyfin` | `linuxserver/jellyfin` | LinuxServer.io image, third-party. |
| `immich` | `ghcr.io/immich-app/immich-server` + internal Postgres, Redis | Immich's own org, not getumbrel. Internal siblings only. |
| `plex` | `linuxserver/plex` | Same as jellyfin. |
| `home-assistant` | `homeassistant/home-assistant` | App's own publisher. |
| `adguard-home` | `adguard/adguardhome` | App's own publisher. |

All 7 sample apps would correctly land in Tier 4 (allow).

### Filter rule refinement (small)

ADR-0024's Tier 1 rule "`depends_on:` references another Umbrel app id" needs a precise definition because compose's `depends_on` is also used **within the same project** for internal services (e.g. `nextcloud` depending on its own `mariadb` service). The implementation should distinguish:

- **Same-compose sibling** (`depends_on: [db]` where `db:` is defined in the same `services:` block) → not a Tier 1 violation
- **Different-app sibling** (env var `${APP_<OTHER>_*}` where `<OTHER>` is a known Umbrel app id from the catalog list) → Tier 1 violation

The implementation will resolve "known Umbrel app ids" by maintaining a set of upstream catalogue directory names and matching `${APP_<OTHER>_*}` against `OTHER.toLowerCase()`. Sample matches the bitcoin/lightning/electrs case correctly.

## Icon URL — confirmed pattern

Per the user direction (no re-host effort initially), the emit step writes the upstream URL directly. The canonical pattern is:

```
https://getumbrel.github.io/umbrel-apps-gallery/<id>/icon.svg
```

**Verified with `HEAD` requests:** all 6 sample apps (`nginx-proxy-manager`, `jellyfin`, `adguard-home`, `nextcloud`, `bitcoin`, `home-assistant`) return `HTTP 200 image/svg+xml`. The `raw.githubusercontent.com` path on the gallery repo's `main` branch returns 404 — icons are served only from GitHub Pages.

**Implication for Phase 1:** the sync binary does NOT need to fetch the icon binary. The `appfile.json` `icon:` field gets the GitHub Pages URL verbatim. The UI loads it as a cross-origin image. Failure mode: if the user's network blocks GitHub Pages, icons fail-soft (existing AppCard already shows the `Package` Lucide fallback when `<img onerror>` fires).

**Escape hatch tracked, not implemented:** config flag `community_catalog.icon_proxy: false`. When flipped, the sync step downloads + caches locally at `/static/catalog/icons/<id>.svg`. Add only if the cross-origin path causes real problems.

## Description sourcing — confirmed approach

`umbrel-app.yml` carries a `description:` field that is **Umbrel-curated content** — explicitly the kind of expressive material the legal posture in ADR-0024 says to skip. Inspected example from `nginx-proxy-manager/umbrel-app.yml`:

> Expose your apps to the internet easily and securely. ⚠️ Be cautious when exposing apps to the public internet…

This is Umbrel's curated marketing copy. NOT imported.

Instead, the sync step reads the upstream `repo:` field (the app maintainer's own GitHub URL), fetches `README.md` from `raw.githubusercontent.com/<repo>/<default-branch>/README.md`, strips the markdown headers + truncates to ~200 words for the catalogue tile description. The README is owned by the app maintainer under their own OSS license (typically MIT/Apache/GPL — all of which permit display).

**Maintainer override:** if a `description-powerlab.md` file exists in `community-catalog/apps/<id>/`, it wins over the auto-fetched README. This is the "make our own descriptions" surface the user asked for in the umbrella issue discussion.

## Sample size + caveats

- The Tier 1 + Tier 4 spot-check covered 10 apps out of 330 (3 %). Statistically tight but the trigger patterns (`getumbrel/*` image, `${APP_<OTHER>_*}` env vars) are simple text matches — the filter doesn't need broad coverage validation, only correctness validation. Coverage of edge cases will land in the test corpus (Phase 6 from #307).
- The 62 + 54 overlap numbers are based on case-insensitive exact name match. Apps with slightly different names (`adguard-home` vs `AdGuardHome` vs `adguard-home-host`) need a fuzzy-match pass in Phase 0.5 to confirm no false negatives, but the gross numbers are accurate.

## Recommended Phase 1 priorities

Given the audit, the implementation order for Phase 1 ([cmd/sync-catalog/](https://github.com/neochaotic/powerlab/issues/307)):

1. **`filter.go`** first — implement Tier 1 (`getumbrel/*` image regex + cross-app env-var detection). Validate against `bitcoin`, `lightning`, `electrs` test fixtures (all must be rejected). This is the load-bearing legal+technical gate.
2. **`parser.go`** — field allowlist; extract `id`, `name`, `tagline`, `category`, `port`, `image`, `ports`, `env keys` (not values), `volumes`. Drop `description`, `gallery`, `releaseNotes`, `submitter`, etc.
3. **`description.go`** — fetch `repo:` field's README; strip + truncate. Skip if `description-powerlab.md` override exists.
4. **`emit.go`** — generate `appfile.json` per app with the `source:` provenance block.
5. **`main.go`** — CLI glue.
6. **`icon` field handling** — emit the upstream URL verbatim; no download.

Tier 2 (category-based soft reject) ships in Phase 1 but with the **default-deny on Bitcoin/Lightning/Bitcoin Node** decided in #307. Sample: `bitcoin`, `lightning`, `electrs` already auto-rejected by Tier 1, so Tier 2 acts as a backstop. Other crypto-adjacent apps (`bitfeed`, `bitwatch`, `bitaxe-sentry`, `bleskomat-server`) — need a quick spot-check of their category before locking the default-deny list.

## Net code direction estimate (Phase 1+ combined)

- New: `backend/sync-catalog/` (~600 LOC Go); `.github/workflows/sync-umbrel-catalog.yml` (~80 LOC); `Makefile` target.
- Generated (per sync run): `community-catalog/apps/<id>/{appfile.json, description.md}` × ~250-280 apps after filter. ~5 MB.
- UI: extends AppCard with a source badge (~30 LOC); adds a catalogue filter (~50 LOC).

Total Phase 1 + 2 + 3 + 4: ~800 LOC committed code + ~5 MB of generated catalogue data on first sync. Subsequent syncs are diffs.

## References

- ADR-0024 — design + legal posture (`docs/decisions/0024-umbrel-catalog-clean-room-import.md`)
- Umbrella tracking — [issue #307](https://github.com/neochaotic/powerlab/issues/307)
- Upstream catalogue: <https://github.com/getumbrel/umbrel-apps>
- Upstream icons (separate repo): <https://github.com/getumbrel/umbrel-apps-gallery>
