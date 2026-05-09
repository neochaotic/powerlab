---
title: "0019 — Tech debt lives in audits + ADRs + labeled issues, not in a TECH-DEBT.md"
status: accepted
date: 2026-05-09
tags: process, docs, maintenance
---

# 0019 — Tech debt lives in audits + ADRs + labeled issues, not in a `TECH-DEBT.md`

**Status:** accepted
**Date:** 2026-05-09
**Tags:** process, docs, maintenance

## Context

A natural reflex when a project starts to accumulate known-but-deferred
work is to add a `TECH-DEBT.md` (or `TODO.md`, `KNOWN_ISSUES.md`) at
the repo root and append to it forever. The cost of that reflex is
predictable:

- Static markdown decays. Items get fixed but the bullet stays. New
  items get added in the issue tracker but the file is forgotten. Six
  months in, the file lies more than it informs.
- Two sources of truth disagree. A reader sees a debt entry; a grep
  shows the underlying code is gone. Trust drops.
- The file becomes append-only. Nobody removes lines because they're
  not sure who else relied on them. Cruft compounds.

PowerLab already has three places where debt is captured, each with a
clear refresh discipline:

1. **`docs/audits/*.md`** — one-shot point-in-time audits of structural
   surfaces (CasaOS dependencies, dead code, endpoint usage, UI feature
   map). Each carries a date and a sprint header. Refreshed at sprint
   boundaries when the surface materially changes.

2. **`docs/decisions/*.md`** — ADRs. Locked-in technical choices.
   Status field tracks whether the decision still stands or was
   superseded. Never deleted; superseded ADRs link to the replacement.

3. **GitHub issues with structured labels** — the live work queue.
   Labels in use:
   - `casaos-strip` — anything removing CasaOS legacy
   - `foundation` — internal `pkg/*` work
   - `sprint-1` … `sprint-4` — the kill-CasaOS roadmap (#67)
   - `someday` — deferred indefinitely; may revisit
   - `separate-repo` — strategic marker, work happens elsewhere

A user asked whether to consolidate this into a single `TECH-DEBT.md`.
The answer is no, and this ADR records why so the next person who has
the reflex finds the reasoning before they create one.

## Decision

**Tech debt lives in three places, by kind, with these refresh rules:**

| Kind                                              | Lives in                          | Refreshed when                                                                                  |
|---------------------------------------------------|------------------------------------|-------------------------------------------------------------------------------------------------|
| Point-in-time structural audit                    | `docs/audits/<topic>.md`           | At sprint boundary OR when the audited surface changes by ≥20%. Date header gets updated each refresh. |
| Locked-in technical decision (incl. process)      | `docs/decisions/NNNN-*.md` (ADR)   | Never edited destructively. Superseded ADRs get `Status: superseded by NNNN` and a link.        |
| Live work queue, including known unfixed bugs     | GitHub issues with labels above    | Continuously. The issue tracker is the live state; closed issue + linked PR is the proof.       |

**No `TECH-DEBT.md` / `TODO.md` / `KNOWN_ISSUES.md` at the repo root.**
If a contributor needs an at-a-glance "what's left?" they get it from
the open-issue list filtered by label, not a stale markdown file.

Inline `// TODO` / `// FIXME` comments are allowed but constrained to
the `non-obvious why` rule (CLAUDE.md): only when the WHY of a deferred
fix is something a reader can't reconstruct from the code. As of this
ADR there are 27 such comments across the backend; that order of
magnitude is the ceiling.

## Rationale

The three buckets each have a natural refresh trigger. Audits get
re-run at sprint boundaries because that's when the surface they
measure changes substantively. ADRs are forever (their value IS their
permanence). Issues are continuously refreshed because that's literally
the tool's purpose.

A consolidated `TECH-DEBT.md` has no natural refresh trigger. It has
to be manually kept in sync with all three above sources, which means
it inevitably falls out of sync. The marginal information it provides
(a flat list) is already obtainable via:

```bash
gh issue list --label casaos-strip --state open
gh issue list --label someday      --state open
ls docs/audits/
ls docs/decisions/
```

…which is the actual command a maintainer would run anyway. Codifying
that into a flat file just means the file becomes wrong six months
later.

## Consequences

**Positive:**
- Single source of truth per debt-kind.
- Stale items are visible (closed issue, dated audit) instead of
  rotting in plain sight.
- New contributors learn the labeling convention once and can find
  any debt category by query.

**Negative / accepted:**
- "Show me everything that's left" requires knowing which queries to
  run. Mitigated by:
  - Issue #67 serves as the master roadmap with sprint phasing.
  - This ADR + `docs/audits/casaos-dependencies.md` together describe
    the structural surface.
  - `docs/decisions/README.md` indexes ADRs by tag.
- A `TECH-DEBT.md` would be slightly more discoverable for an outside
  contributor doing first-time orientation. We accept that cost in
  exchange for not letting the doc lie.

## Alternatives considered

1. **Single `TECH-DEBT.md` updated per release.** Rejected — the file
   would still drift between releases. Per-release updates also become
   ceremonial: nobody enjoys editing a flat file at release time.

2. **Auto-generated `TECH-DEBT.md` from labels at CI time.** Rejected
   — the cost of adding (and maintaining) the generator outweighs the
   value of the static rendering. Anyone who wants the list runs `gh`
   on demand.

3. **Use GitHub Projects exclusively.** Rejected — Projects don't
   capture the *why* the way labels + issue body + ADR cross-link do,
   and they live outside the repo.

## Refresh discipline

- **Audits:** `docs/audits/casaos-dependencies.md` is the canonical
  example. After every sprint that changes the CasaOS surface (Sprint
  1 / 2 / 3 each did), the audit gets a `# Update — YYYY-MM-DD` section
  appended (not a rewrite — the original sprint snapshots stay as a
  historical record, the latest update goes at the top of the file
  in a "Current state" section). Sprint-3 closeout (this PR) is the
  first such update.
- **ADRs:** Append-only. If a decision is reversed, the new ADR
  references the old one and the old one gets `Status: superseded by
  NNNN`.
- **Issues:** Closed when the work is done. Labels never removed
  retroactively (they're history).

## Reference

- Audit pattern: `docs/audits/casaos-dependencies.md`
- ADR conventions: `docs/decisions/README.md`
- Master roadmap: #67 (issue)
- Sprint-3 closeout that codified this pattern: this commit
