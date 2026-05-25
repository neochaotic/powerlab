# 0043 — Embed the frontend (and static resources) into the gateway binary

- **Status:** proposed
- **Date:** 2026-05-25
- **Trigger:** Recurring UI↔binary version-skew bugs (the upgrade-401 class, v0.6.7→v0.6.10) plus the enterprise/security pivot raised the question: should PowerLab ship the web UI *inside* the Go binary via `go:embed` instead of serving it from a directory on disk?

## Context

Today the gateway serves the web UI from a filesystem directory:

```
ExecStart=/usr/bin/powerlab-gateway -w /usr/share/powerlab/www
```

The pipeline that puts it there:

1. The UI is built with SvelteKit's `adapter-static` (SPA mode, `fallback: index.html`) → a **pure static bundle** in `ui/build/` (currently **4.0 MB**, 91 files: `index.html`, `_app/immutable/*`, icons, docs images).
2. `scripts/package-linux.sh` copies `ui/build/*` → `$STAGE/www`, and the installer lays it down at `/usr/share/powerlab/www`.
3. The gateway (pure-Go, no CGO) reads that directory at runtime via the `-w` flag.

Two relevant facts make embedding cheap to adopt:

- **`go:embed` is already in the codebase.** `app-management` and `message-bus` embed `api/index.html`, their `openapi.yaml`, and `*.conf.sample` directly into their binaries. The pattern, the build wiring, and the team's familiarity already exist.
- **The UI bundle is already a flat static tree** — exactly the input `go:embed` expects. No SSR, no Node runtime, no server-side routing to reconcile.

### The cost we actually pay today

The UI and the binary are two artifacts that travel together but are coupled only by *convention*, not by *construction*. When they drift, users get subtle, hard-to-diagnose failures. PowerLab has paid for this drift repeatedly:

- The **upgrade-401 saga** (v0.6.7→v0.6.10): a new binary served against a stale or mismatched UI surface.
- An entire **defense-in-depth stack exists solely to detect drift**:
  - **L1** — `package-linux.sh` re-syncs `ui/package.json` on every build.
  - **L1.5** — `prepare-release.sh` bumps `ui/package.json` on main.
  - **L2** — `check-ui-package-version-fresh_test.sh` gates CI on a stale `package.json`.
  - **L3** — `package-linux.sh` greps the built JS for the version literal.

All four layers are compensating controls for a structural problem: *the binary and the UI can be at different versions on the same box.* Embedding removes the problem at the root rather than detecting it after the fact.

## Decision

**Embed the static UI bundle into the gateway binary via `go:embed`, and serve it from an in-binary filesystem. Embedding is the default; the existing `-w <dir>` flag is retained as a dev/debug override.**

Concretely:

- The gateway gains `//go:embed all:www` (the `all:` prefix so dotfiles and `_app/` are included) bound to an `embed.FS`.
- The UI is served via `http.FS(embeddedFS)` (through Echo's static handler) with an SPA fallback: any unmatched non-asset path returns the embedded `index.html`, preserving client-side routing.
- **Precedence:** if `-w <dir>` is passed *and* the dir exists, serve from disk (dev/debug/hot-iteration). Otherwise serve the embedded bundle. This keeps `npm run dev` and live-debugging workflows intact with zero loss.
- **Build ordering becomes explicit:** the UI (`ui/build/`) must be built and copied into the gateway's embed source dir *before* `go build` of the gateway. CI and `package-linux.sh` enforce this ordering.
- **Scope is the gateway only.** The other five services are unaffected; only the gateway binary grows (by ~4 MB, the uncompressed bundle). Blast radius is contained to one binary.

### What this lets us retire (follow-up, not part of this ADR)

Once the UI ships *inside* the binary, the version-skew compensations (L1.5, L2, L3) lose their reason to exist — the binary's version *is* the UI's version, atomically. We do not remove them in the same PR; we mark them for retirement in a follow-up once embedded delivery is proven on a release.

## Consequences

### Positive

- **The UI↔binary version-skew bug class disappears by construction.** Shipping one binary ships one UI. The upgrade-401 failure mode cannot recur from drift.
- **Atomic upgrades.** Replacing `/usr/bin/powerlab-gateway` replaces the UI in the same `mv`. No window where the binary is new and `www/` is old (or vice-versa).
- **Integrity / tamper-resistance.** UI assets live inside the (checksummed, signable) binary instead of as loose mutable files under `/usr/share`. This fits the enterprise-pivot lens ("would enterprise IT accept this in production") and the security-is-priority floor.
- **Simpler ops & packaging.** Staging deploys become a single-binary `scp`. `package-linux.sh` drops the `www/` stage + install steps. Fewer paths, fewer install-time failure points.

### Negative / costs

- **Binary size +~4 MB.** Negligible for a server OS. Can be reduced later with build-time asset compression if ever warranted (not now — YAGNI).
- **Build coupling.** A UI-only change now requires rebuilding the gateway binary, and CI must build the UI before the gateway. In practice they already ship together every release; this only formalizes the ordering.
- **No loose hot-patching.** You can no longer drop a fixed `index.html` onto a box without a new binary. For the enterprise/integrity posture this is a feature, not a regression.
- **Dev ergonomics depend on the `-w` override** working correctly; it must be covered by a test so it does not silently rot.

## Alternatives considered

1. **Status quo — serve `www/` from disk (`-w`).** Rejected as the default: it is the source of the drift bug class and keeps the L1–L3 compensation stack alive indefinitely.
2. **Dedicated static server (nginx / Caddy) fronting the gateway.** Rejected: adds a process, a config surface, and a new failure mode to a single-box appliance — the opposite of the "fewer moving parts" goal.
3. **Embed compressed and decompress at startup.** Rejected for now: premature optimization. 4 MB uncompressed is fine; revisit only if binary size becomes a real constraint.
4. **Embed in a *new* dedicated "web" service.** Rejected: the gateway is already the HTTP front door and already holds the `-w` contract; a new service is unjustified scope.

## Migration plan (phased)

1. **Wire embed + serve, keep `-w` override** (one gateway PR). Add a real-server test: (a) embedded mode serves `index.html` + an `_app/` asset + SPA fallback; (b) `-w <dir>` override still serves from disk.
2. **CI/build ordering** — ensure `ui/build/` is produced and staged into the gateway embed dir before `go build`; `package-linux.sh` honours `POWERLAB_SKIP_FRONTEND_BUILD` the same way it does today.
3. **Drop `www/` from packaging + the systemd `-w` flag** once embedded serving is verified on a release (Lima + a real box).
4. **Retire L1.5 / L2 / L3 version-skew compensations** in a follow-up, leaving L1 (build-time sync) only if still useful for the dev `-w` path.

Each phase is independently shippable and reversible; the `-w` override means we can fall back to disk serving at any point without a code change.

## Verification

- Real-browser E2E (Playwright) against the embedded gateway: the SPA loads, client routes resolve via the `index.html` fallback, and a hashed `_app/immutable/*` asset is served with the right content-type.
- A regression test that the `-w` override still serves from disk (protects the dev workflow).
- Manual: install the embedded build on Lima, confirm the UI loads with **no** `/usr/share/powerlab/www` present; then upgrade-in-place and confirm the UI version moves atomically with the binary.
