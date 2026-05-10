# Work review — 2026-05-10

**Sprint:** 4 (closure + v0.5.x release wave + docs Phase 3)
**Reviewer:** self-review with senior-engineer hat on
**Method:** read what shipped, compare against what-good-looks-like, name what I'd change

Companion to `sprint-4-retrospective.md` (which covers process)
and `casaos-residue-2026-05-10.md` (which covers leftover work).
This doc is specifically about CODE + DESIGN quality of what
landed today.

## Day's output

17 PRs merged, 3 releases tagged (v0.5.8 → v0.5.10), ~5,500 LOC
churned (mostly net-add: +6,500 / -1,500). Top contributors by
diff size:

- #181 PR-A foundation (+791) — labels package + tests
- #180 split-brain detection (+844)
- #197 godoc + Scalar (+769)
- #192 v0.5.9 hot-fix (+672)
- #188 mkdocs foundation (+558)
- #177 JWT keypair persistence (+562)
- #194 README sweep (+49 / -913, biggest deletion)

## What I'd keep as-is

These I'd ship the same way again.

### `backend/common/utils/paths/db.go` design (PR #180 + #192)

The `AssertNoSplitBrain` + `AutoMoveLegacyAside` pair is the right
shape: one strict refuser for ambiguous cases, one auto-resolver
for unambiguous ones. Test coverage at **87.3%** on the package.
The split between "ambiguous = operator picks" vs "unambiguous =
auto-clean" maps directly to the actual operational reality.

Symmetric helpers per service (`UserServiceDBIn(base)` etc.)
correctly avoid the centralisation trap of one giant config map.

### ADR-0020 + ADR-0021 honesty

Both ADRs explicitly call out post-hoc rationalisations and
reject them. ADR-0020:
> "It was not actually deliberate. Git blame shows the code was
> inherited verbatim... the godoc comment claiming 'deliberate
> trade-off' was added much later by the same author writing this
> ADR. The comment was a post-hoc rationalization of inherited
> behavior, not a record of a real architectural choice."

This kind of self-correction is what ADRs are for. Future me
won't be misled.

### Test discipline on bug fixes

Every bug fix landed with a regression test, with the test name
locked to the bug:
- `TestLoadOrGenerate_StableAcrossCalls` — the THE-#176 regression
- `TestAutoMoveLegacyAside_AfterAssertNoSplitBrainPasses` — the
  THE-#179 v0.5.9 regression
- `scripts/test-upgrade-resolves-stale-legacy_test.sh` — the
  v0.5.4 mishap state simulation

Memory rule "bug fix = regression test, no exceptions" held under
pressure. This was the discipline that contained today's prod
incident at "5 minute mean time to detect, 30 minute mean time
to permanent fix."

## What I'd change soon (technical debt I created today)

### 1. Mermaid loaded from unpkg.com CDN (PR #195)

`mkdocs.yml` has:
```yaml
extra_javascript:
  - https://unpkg.com/mermaid@10.9.1/dist/mermaid.min.js
```

Problems:
- **No Subresource Integrity hash.** A compromised unpkg or DNS
  could inject malicious JS into the docs site. Low-impact (docs
  site is read-only) but bad form.
- **Offline broken.** The docs tarball has no mermaid.js bundled,
  so a self-hosted docs site (or CI build without internet)
  silently breaks all diagrams.
- **CDN version pinning is brittle.** unpkg respects `@10.9.1`
  but a typo or stale version means the site breaks for users
  even though the source is correct.

**Fix:** vendor mermaid.js into `docs/js/mermaid.min.js` (it's
~3MB, that's fine for a docs repo). Pin via filename. SRI not
needed for local file. ETA: 30 minutes.

### 2. Generated godoc markdown committed to git (PR #197)

`docs/api/pkg/{logging,errors,...}.md` are 100% machine-generated
by `scripts/gen-godoc.sh`. Committing them means:
- Diffs are noisy on every refactor of internal types
- Two sources of truth (Go source + committed markdown)
- Reviewer sees "doc churn" and tunes out

**Fix:** delete the committed copies + run gen-godoc as a CI step
before mkdocs build. The script already supports `--check` mode
for drift detection.

ETA: 30 minutes. Justification for current state: I wanted the
PR diff to be tangible so the reviewer could see what gomarkdoc
actually produces. Was the right call for foundation PR; should
flip to CI-only now.

### 3. `updater.test.ts` is a contract sketch, not a test (PR #192)

```typescript
it('shows success toast when succeeded_at changes', async () => {
    // Simulate the polling logic's success branch directly.
    toast.success('PowerLab updated successfully — reloading…', 3000);
    // ... assertion
});
```

The test calls `toast.success(...)` DIRECTLY rather than calling
the updater store's polling code that would emit the toast. So
it tests "toast.success works" (which we already test in
`toast.test.ts`), not "the updater store calls toast.success
when succeeded_at changes."

**Fix:** mock the api layer (`installUpdate`, `getUpgradeStatus`)
and exercise `updaterStore.install()` end-to-end. Real test of
the state machine.

ETA: 1.5 hours. Decided against today because:
- Manually validated on prod host (works)
- The path it covers is short (one observable, two branches)
- Would have delayed the v0.5.9 hot-fix while the user was still
  locked out

But it's debt. Should land in Sprint 5 alongside the per-service
godoc raise.

### 4. Dual-write window has no teardown PR yet

ADR-0021 commits to "ONE release window" of dual-write for
container labels. v0.5.8 starts the window. Without a tracking
issue + scheduled removal PR, this becomes "dual-write forever"
by inertia.

**Fix:** open issue now: "Drop legacy unnamespaced container
label writes after v0.6.x" with the specific files to edit
(`common/labels.go::BuildLabels` — remove the legacy write
half). ETA to track: 5 minutes. ETA to do: 30 minutes when the
window closes.

### 5. ADR count = 24, no index page

`docs/decisions/` has 24 ADRs now (added 0020, 0021 today).
README.md exists but the mkdocs nav points at it as a single
"Index" entry — users have no way to discover ADRs by topic
(security, foundation, casaos-strip).

**Fix:** add tag-based grouping. Each ADR already has frontmatter
with `tags:`. A 50-line generator could produce
`docs/decisions/by-tag/security.md` etc. Or use mkdocs-awesome-pages.

ETA: 1 hour. Sprint 5 docs polish.

## What I'd change later (not urgent)

### 6. Compose extension translation chain has 3 names

`x-powerlab` (canonical), `x-web` (intermediate inherited from
upstream), `x-casaos` (legacy). Each compose file might use any.
The translation layer in `service/extension.go` handles all
three with priority chain.

This is correct for ecosystem compatibility but adds a 3-way
fork-and-rename surface. After ADR-0021 fully lands (i.e. 2-3
releases out, when the dual-write window closes), revisit
whether `x-web` can be dropped (it was an intermediate naming,
no current store apps use it as the primary).

### 7. Per-service godoc coverage <50% across the board

Tracked in #196. Today's audit:
- pkg/* 100% ✅
- common 49%, user-service 40%, core 39%, app-management 35%,
  local-storage 27%, gateway 21%, message-bus 17%

Sprint 2 Phase 6 was supposed to bring the killed services to
high coverage; gateway 21% says that didn't really happen.
Sprint 5 raise plan exists but enforcement pattern doesn't —
need a CI gate that flags drops below threshold.

### 8. `--allow-coexist` flag is now no-op but still parsed

After PR #183 relaxed the CasaOS coexistence block, the
`--allow-coexist` flag is silently accepted for backward compat
but does nothing. install.sh still has the parsing logic + the
"unknown argument" error path that mentions it. Smell — either
deprecate explicitly with a warning or remove parsing.

ETA to remove cleanly: 15 minutes.

## What I'd change but it's actually fine

### 9. 5 layers of split-brain prevention feels like overkill

L1 (helpers) + L2 (migration script) + L3 (boot-time check) +
L4 (regression tests) + L5 (install.sh audit) + the v0.5.9
auto-clean = arguably 6 layers now.

Counterargument: each layer caught something different. The boot
check caught the v0.5.4 sobra in production; the migration test
caught a YAML rewrite issue in review; install.sh L5 will catch
the case the boot check misses (split-brain across services).
The cost (each layer is ~50-100 LOC + tests) is small relative
to the cost of one prod incident.

Verdict: **keep all layers**, but the file
`docs/audits/db-paths.md` is the ONLY place to look when
debugging — make sure it stays the source of truth.

### 10. 17 PRs in one day is a lot

Reviewer might think "no PR was reviewed properly." Counterargument:
- 6 of them were release/docs-polish PRs (low risk)
- 5 were the #85 sub-PRs (each ~100 LOC, scoped)
- 3 were follow-up fixes catching issues caught in same-day testing
- 3 had real CI runs (~5min each) verifying correctness

The high PR count actually reflects the GOOD pattern: each PR
has one concern, one reviewer pass, then merge. Beats one
mega-PR with 5,500 LOC.

Verdict: **fine, even good**. The integration test suite (~80
new tests across the day) is the real safety net.

## Risks I'm watching

### A. Dual-write window expiry awareness

If the issue tracking the dual-write removal isn't opened before
v0.6.0 ships, we'll have a slow drift where legacy labels get
written forever. Setting a calendar reminder is not enough —
need a CI assertion that fails AFTER the planned removal date.

### B. Test debt compounding

`updater.test.ts` is a sketch (item 3 above). The integration
test `test-upgrade-resolves-stale-legacy_test.sh` only covers
ONE upgrade-mishap scenario. Sprint 5's real upgrade test has
to actually exist or the next bug ships.

### C. Docs site has 5 broken-pre-existing INFO warnings

`mkdocs build --strict` passes (those are INFO not WARNING) but
3 of them are real broken anchors in
`patterns/https-trust-onboarding-pattern.md`. Worth a 10-minute
cleanup PR — not urgent but easy.

## Recommended Sprint 5 order

Weighted by leverage (impact / effort):

1. **Vendor mermaid.js + delete generated godoc commits**
   (item 1 + 2) — ~1h, eliminates 2 sources of fragility
2. **Open follow-up issues** for items 4, 8 — ~15 min, prevents
   inertia
3. **Real updater test** (item 3) — ~1.5h, locks the v0.5.9 UX
   contract
4. **Per-service godoc raise** (#196 — already tracked) — gateway
   first, ~1h
5. **Bug-hunt sweep** + dead-code per #185 — bigger work, parallel
6. **Then HTTPS prep work** for Sprint 6

Stuff NOT on this list (deliberate non-action): item 5 (ADR
index), item 6 (extension chain) — wait until the dual-write
window closes; item 9 (split-brain layers) — keep all 5+1.
