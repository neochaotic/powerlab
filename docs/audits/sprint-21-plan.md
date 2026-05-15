# Sprint 21 — Plan (Logs Phase 4 + catalog sweep)

**Date drafted:** 2026-05-15 (immediately after v0.6.15 cut)
**Predecessor:** Sprint 20 (`sprint-20-retrospective.md`)
**Target release:** v0.7.0 (minor bump — new feature surface + CI gate strict-mode flips, no API breakage)
**Theme:** complete the logs scope that's been open since Sprint 14 + finish the catalog hostname cleanup started in Sprint 20.

## Goal

Two threads closed:

1. **Logs Phase 4 (#259)** — Settings → Logs grows from the Sprint 20 file-tail MVP into a real per-service journald viewer with tabs, live follow, and severity coloring. Operator parity with what was promised for Sprint 14 Phase 4.
2. **Catalog hostname sweep (#402)** — finish the 66-app mechanical edit, add a `sync-catalog` transform so future upstream syncs stay clean, and flip the lint to strict.

Plus the v0.7.0 ramp:
- Flip `Backend deadcode` CI gate to strict (Sprint 19 PR 5 scheduled this for v0.7.0)
- Flip `Catalog hostnames` CI gate to strict (after the sweep)

## Why minor bump (v0.7.0)

Per semver and memory `feedback_no_v1_without_alignment`:
- v0.7.0 is a pre-v1 minor bump — fine. The major (v1.0) is the contract; v0.x can carry feature additions in minor bumps.
- **New user-facing feature surface**: per-service journald viewer + severity coloring. This is more than a patch.
- **CI gate strict-mode flips**: developers pushing dead code or catalog hostname regressions will see hard-fail builds going forward. Worth flagging in the release notes.
- No production API breakage. No breaking-changes manifest entry needed.

## Strategy

Five rules:

1. **SSE territory is high-risk.** Memory `feedback_sse_test_real_browser_headers` applies — every endpoint that emits `text/event-stream` ships with a real-browser test using `Accept-Encoding: gzip` to catch buffering regressions. The Sprint 18 fix (#384) covered `/logs` suffix; Phase 4 endpoints use `/stream` so the skipper needs extension.
2. **Catalog sweep is mechanical, not creative.** A script does the find-and-replace; reviewer's job is to spot-check 5 apps, not to read 66 diffs. Don't gold-plate.
3. **journald reads need OS-level care.** `journalctl -u powerlab-<svc>` requires the gateway process to have read access to the journal. Default on Debian/Ubuntu: yes (no sudo needed for own units, but cross-unit reads require `systemd-journal` group). Document in the PR.
4. **Service-name allowlist is non-negotiable.** Path-traversal hardening from the Logs MVP (`^[A-Za-z0-9._-]+\.log$`) needs an analog: `^[a-z][a-z0-9-]+$` for service names, validated BEFORE `exec.Command`. Defeats shell injection.
5. **Subprocess lifecycle matters.** When the SSE client disconnects, the journalctl subprocess must die. Context cancellation + `cmd.Process.Kill()` in a defer. Leaked journalctl processes would balloon over an active operator session.

## Execution order

Small first, SSE last. Each PR independently mergeable.

### PR 1 — Catalog hostname sweep (~2-3h)

Mechanical sed across the 66 affected catalog compose files. For each `<project>_<svc>_<idx>` hostname pattern in env vars, replace with the service-name alias (`db`, `redis`, etc.).

**Methodology:**

```bash
# Per-file diff (review-friendly)
for f in $(grep -rln "_[a-z]*_1" community-catalog/Apps/*/docker-compose.yml \
            | grep -v _test); do
  proj=$(grep -E "^name:" "$f" | head -1 | awk '{print $2}')
  # In env values only, replace <proj>_<svc>_<idx> with <svc>
  sed -i.bak -E "s/${proj}_([a-z][a-z0-9]*)_[0-9]+/\1/g" "$f"
  rm "${f}.bak"
done
```

**Verification (mandatory before push):**
- `./scripts/check-catalog-hostnames.sh` → 0 findings
- Spot-check 5 of the 66 apps (one from each category — DB, cache, queue, search, web): `docker compose -f <file> config` parses clean
- Confirm `x-powerlab` block intact (the `port_map` field MUST stay)

**Risk:** false-positive matches. Mitigation: the regex anchors with `_[0-9]+$` per env line, so URLs like `app_db_1.example.com` could match wrong. Spot-check covers this.

### PR 2 — `sync-catalog` transform rule (~1.5h)

Add a transform in `backend/sync-catalog` that auto-rewrites `<project>_<svc>_<idx>` to the service alias when syncing from upstream. Prevents the bug class from re-introducing on every catalog refresh.

**Files:**
- `backend/sync-catalog/transform/hostnames.go` (new) — pure function `RewriteCompose(yaml []byte) []byte`
- `backend/sync-catalog/transform/hostnames_test.go` — 6 cases (broken, clean, prefix-only guard, no-name-header, multi-project, idempotent)
- Wired into the existing transform pipeline

**Risk:** transform fires on a string that LOOKS like the pattern but is intentional (cross-app reference). Mitigation: only rewrite within env-value contexts, not in `container_name:` or `name:` fields.

### PR 3 — Backend journald SSE endpoint (~half day)

New endpoint: `GET /v1/logs/services/{service}/stream` on the gateway public mux.

**Implementation:**

```go
// backend/common/utils/logs/journald_stream.go
//
// Service name allowlist (strict regex), validated BEFORE exec.
// On disconnect, kill the subprocess (defer cmd.Wait + cancel).

func ServiceStreamHTTPHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        svc := lastPathSegment(r.URL.Path)
        if !serviceNameAllowlist.MatchString(svc) {
            http.Error(w, "invalid service", http.StatusBadRequest)
            return
        }

        w.Header().Set("Content-Type", "text/event-stream")
        w.Header().Set("Cache-Control", "no-cache")
        flusher, _ := w.(http.Flusher)

        ctx, cancel := context.WithCancel(r.Context())
        defer cancel()
        cmd := exec.CommandContext(ctx, "journalctl",
            "-u", "powerlab-"+svc,
            "-f", "-o", "json", "--no-pager")
        stdout, _ := cmd.StdoutPipe()
        if err := cmd.Start(); err != nil { ... }
        defer cmd.Wait()

        scanner := bufio.NewScanner(stdout)
        scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
        for scanner.Scan() {
            line := scanner.Bytes()
            // Parse journald JSON, extract MESSAGE + PRIORITY
            // Emit as SSE: data: {"msg":"...","priority":N,"ts":"..."}
            ...
            flusher.Flush()
        }
    }
}
```

**Audit middleware gzip skipper:** extend the path match from `HasSuffix("/logs")` to include `"/stream"` so journald responses don't get gzipped.

**Tests:**
- Unit: `serviceNameAllowlist` rejects path traversal, shell injection, empty, etc.
- Unit: parses journald JSON correctly, maps priority to severity string
- Integration (real journald on Linux CI runner): subscribe to `gateway` service, assert chunks arrive line-by-line

### PR 4 — Frontend per-service tabs + live follow + severity coloring (~1 day)

Extend `LogsPane.svelte` with tabs:
- **Files** tab (current MVP) — read static `.log` files
- **Live** tab (new) — per-service journald stream

Or split into a separate `ServicesLogPane.svelte` and add a second Settings entry — tbd at implementation time, whichever feels cleaner.

**Live tab features:**
- Service picker (gateway / app-management / user-service / message-bus / local-storage / core / sync-catalog)
- EventSource subscription to `/v1/logs/services/{svc}/stream?token=...`
- Severity coloring:
  - ERROR (priority 0-3) — red
  - WARN (priority 4) — amber
  - INFO/NOTICE (priority 5-6) — default
  - DEBUG (priority 7) — muted
- Auto-scroll lock: when user scrolls up, pause auto-scroll until they hit "Resume" or scroll to bottom
- Pause/resume button (kills + restarts EventSource)
- Severity filter chips
- Clear button (empties the visible buffer; doesn't affect journald)

**Tests:**
- vitest: render the live tab, mock EventSource, assert severity classes apply
- Real-backend Playwright: subscribe + assert chunks arrive within 5s + assert no Content-Encoding: gzip

### PR 5 — Real-backend Playwright for journald stream (~2h)

Sister to the existing `logs-pane.smoke.spec.ts`. New spec verifies the wire-level contract of the new endpoint:

```typescript
test('GET /v1/logs/services/gateway/stream emits chunks live', async ({ request }) => {
    // SSE assertion pattern from install-uninstall.smoke.spec.ts
});

test('invalid service name rejected: ../etc/passwd → 400', ...);

test('Content-Encoding is NOT gzip even with Accept-Encoding: gzip', ...);
```

### PR 6 — Flip catalog-hostnames lint to strict (~10 min)

Single-line change in `.github/workflows/ci.yml`:
```diff
-          POWERLAB_CATALOG_LINT_STRICT: '0'
+          POWERLAB_CATALOG_LINT_STRICT: '1'
```

Comes AFTER PR 1 lands so the gate passes from day one.

### PR 7 — Flip `Backend deadcode` to strict (~10 min, after sweep round)

Same pattern. Sprint 19 PR 5 planned this for v0.7.0. Confirm the 86 remaining deadcode findings either land as a quick targeted sweep first OR get `//deadcode:ignore`-marked.

**Decision point at PR start:** do we do another deadcode sweep round (1h, brings 86 → ~30) before flipping? Recommend yes — strict mode with 86 findings is too noisy for a clean v0.7.0.

### PR 7.5 — Quick deadcode targeted sweep (~1h)

Pick the 30-40 most obvious from the remaining 86 (whole-file deletes if any survive). Each one verified, race-tested.

### PR 8 — Sprint 21 retrospective + v0.7.0 cut

Same recipe as Sprint 20: retro doc, manifest summary update, version bump, changie batch, tag.

## Out of scope (carried forward)

- **#260** per-service restart buttons (Power pane + hardened sudoers). Sprint 22+ target.
- **#414** Mac build fix (Makefile + workflow_dispatch artifact). Sprint 22+ target.
- **#295** `apps/+page.svelte` split (1561 LOC god component). Sprint 22+ target — its own focused sprint.
- **Backend Go coverage push** — PR 8 audit doc from Sprint 20 has the targets; incremental over subsequent sprints, not bundled here.

## Acceptance for the sprint

- [ ] PR 1 merged: 66 catalog apps sweep clean
- [ ] PR 2 merged: sync-catalog transform + 6-case test
- [ ] PR 3 merged: backend journald SSE endpoint + hardening tests
- [ ] PR 4 merged: frontend per-service tabs + live follow + severity coloring
- [ ] PR 5 merged: real-backend Playwright smoke for journald stream
- [ ] PR 6 merged: catalog lint strict-mode flip
- [ ] PR 7 + 7.5 merged: deadcode targeted sweep + strict-mode flip
- [ ] PR 8: Sprint 21 retro + `release-manifest.yaml` summary updated for v0.7.0
- [ ] User explicit cut authorization (memory `feedback_require_explicit_release_auth`)
- [ ] Staging .142 validated end-to-end before cut

## Risk surface

- **PR 3 SSE territory** — the highest-risk PR. SSE has bitten us in v0.6.7, v0.6.12, v0.6.13. Mitigations: extend the audit-middleware gzip skipper, add `scripts/check-sse-not-gzipped.sh` to cover the new endpoint, ship the real-backend smoke spec in the same PR.
- **PR 1 catalog regex false positives** — manual spot-check on 5 representative apps + the lint gate's own meta-test fixtures.
- **PR 3 journalctl subprocess leak** — defer kill + `cmd.Wait` test. Manual load-test: open 5 streams simultaneously, close all, verify `ps aux | grep journalctl` is empty.
- **PR 6/7 strict-mode flips on broken builds** — both flips come AFTER the sweeps. If anyone pushes a hostname underscore or new dead code between merge of sweep and merge of flip, CI rejects them with the helpful pointer to the issue.

## Memory invariants reinforced

- `feedback_sse_test_real_browser_headers` — PR 3 + 5 lock the wire contract with `Accept-Encoding: gzip` assertions.
- `feedback_tdd_strict` — every PR starts failing-test-first.
- `feedback_bug_regression_discipline` — journald subprocess leak gets its own regression test.
- `feedback_no_apagar_test_para_passar` — if a real-backend smoke flakes, investigate root cause, don't disable.
- `feedback_run_all_check_scripts_before_release_push` — including the new strict-mode gates.

## v0.7.0 release implications

This release flips two CI gates from warn-only to strict. Documented in the manifest summary so operators reading the changelog know what changed:
- New feature: per-service journald viewer (operator gets gateway/app-management/user-service/etc live logs in the browser)
- Cleanup: 66 catalog apps now use Compose v2 hostnames (some apps that "worked despite the broken hostname" via DB retry now connect on first try)
- Internal: `deadcode` + `catalog-hostnames` CI gates now hard-fail on findings — only affects developer-pushed code, not running installs
