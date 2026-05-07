# 0009 — Name this pattern: "HTTPS Trust Onboarding Pattern"

**Status:** accepted
**Date:** 2026-05-07
**Tags:** docs, security, https, framework, identity

## Context

ADRs 0001–0007 captured load-bearing decisions about the v0.2.7
HTTPS milestone individually: cert validity, signer library,
walkthrough UX, HSTS gate, etc. Externally, those decisions form
a coherent shape — a reusable choreography for getting a
self-signed local CA trusted by client devices on a LAN through
a UI walkthrough, without ever locking the user out.

That shape is portable. Other self-hosted projects facing the
same problem (homelab panels, internal tools, edge appliances)
are likely to converge on the same components. We've described
the shape ad-hoc across seven ADRs and `docs/HTTPS.md`; what is
missing is a single canonical name for it and a single document
that an outside implementer can read to port the pattern to
their own stack.

## Decision

Name the pattern **HTTPS Trust Onboarding Pattern**.

Promote the abstract description from
[`docs/HTTPS.md`](../HTTPS.md) into a dedicated reference at
[`docs/patterns/https-trust-onboarding-pattern.md`](../patterns/https-trust-onboarding-pattern.md),
formatted as a framework specification (RFC-style: glossary,
state machine, threat model, sequence diagrams, per-language
implementation guide, testing checklist, FAQ).

Place the pattern itself **in the public domain** so any project
— commercial, open-source, internal — can copy, port, rename, or
re-license it. The PowerLab reference implementation stays
AGPL-3.0; the pattern is the abstract shape, not the code.

## Rationale

- **Naming gives the pattern an identity.** Reviewers can now
  refer to "the Trust Onboarding Pattern" without re-explaining
  the choreography from scratch every time.
- **A single doc beats seven ADRs scattered across a repo for
  outside readers.** ADRs explain *why* PowerLab decided X; the
  pattern doc explains *what* the pattern is and *how* to
  implement it. Different audiences.
- **Public domain reduces friction for adoption.** Implementers
  in non-AGPL projects (commercial, MIT, Apache) can adopt the
  pattern without legal review.
- **The reference impl stays open** at AGPL-3.0 because the
  *code* is what the project ships and supports; the *pattern*
  is what we're trying to spread.

## Alternatives considered

- **Leave it ad-hoc, named only in conversation.** Rejected:
  costs every new contributor / outside reader a re-explanation.
- **Submit as an RFC.** Premature: the pattern is one
  implementation old. Document first, see if it gets adopted,
  then consider standards-track.
- **Trademark the name.** Rejected: defeats the goal of free
  adoption. "Trust Onboarding" is descriptive, not coinable.
- **Bundle the abstract pattern + ADRs into a `/spec` repo.**
  Worth considering later when the pattern stabilizes and we
  start tracking external implementations. For now keeping it
  in the main repo lowers friction for the author of the
  pattern (us) to keep it in sync with the reference impl.

## Consequences

- We commit to keeping `docs/patterns/https-trust-onboarding-pattern.md`
  in sync with the reference implementation. A future ADR that
  changes the pattern's contract (e.g. adding a new endpoint)
  also updates the pattern doc in the same commit.
- External implementers will reference our reference impl as
  ground truth. We should keep the load-bearing components
  (`backend/common/pkg/security/cert.go`,
  `backend/gateway/route/security_route.go`,
  `backend/gateway/route/gateway_route.go::WrapHSTS`) stable
  enough to read without surprise.
- A separate Go module ("extracted reusable middleware") is a
  natural follow-up tracked elsewhere — the public-domain pattern
  doc is the precursor, the MIT-licensed Go module is the
  drop-in.

## References

- [`docs/patterns/https-trust-onboarding-pattern.md`](../patterns/https-trust-onboarding-pattern.md) — the canonical pattern document
- ADRs 0001-0007 — the per-decision rationale this pattern aggregates
- `backend/common/pkg/security/cert.go` — CertManager reference impl
- `backend/gateway/route/security_route.go` — endpoint contract reference
- `backend/gateway/route/gateway_route.go` — `WrapHSTS` middleware
- `ui/src/lib/components/security/HttpBanner.svelte` — soft banner
- `ui/src/routes/settings/+page.svelte` — walkthrough UI + 4-guard probe
