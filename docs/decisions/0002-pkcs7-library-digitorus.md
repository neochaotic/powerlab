# 0002 — PKCS#7 signing library: `github.com/digitorus/pkcs7`

**Status:** accepted
**Date:** 2026-05-07
**Tags:** security, https, dependency, v0.2.7

## Context

The `.mobileconfig` payload that PowerLab serves to Apple devices must
be signed (PKCS#7 SignedData) for iOS / macOS to display
"Verified by PowerLab Local CA" instead of the red "Unverified"
banner. Conversion drops dramatically when users see that banner.

There are two reasonably-current Go libraries that handle PKCS#7:

| Library | Last commit | Maintenance |
|---|---|---|
| `go.mozilla.org/pkcs7` | 2018 (occasional patches since) | Effectively dormant |
| `github.com/digitorus/pkcs7` | 2024 (active commits) | Active fork |

Both implement the same API surface. The digitorus fork was made
specifically to keep the lib alive after Mozilla stopped investing,
and it carries fixes for a handful of subtle Apple-plist signing edge
cases that the original repo has open issues for but never merged.

## Decision

Use `github.com/digitorus/pkcs7`.

## Rationale

- Active maintenance: a CVE in the upstream openssl-style code paths
  gets a patch within days on digitorus, on the order of "never" on
  Mozilla's repo.
- Drop-in API: `pkcs7.NewSignedData`, `sd.AddSigner`, `sd.Finish` —
  same signatures, no migration risk if we have to swap back later.
- Apple compatibility patches: digitorus has merged fixes for the
  `pkcs9.OIDAttributeMessageDigest` signed-attribute that some iOS
  versions require for the profile to validate. Mozilla's repo has
  this as an open issue from 2021.
- Used in production by several Apple-management tools (Munki, MDM
  fleets), so the iOS plist signing path is well-trodden.

## Alternatives considered

- **`go.mozilla.org/pkcs7`**. Rejected: stale maintenance, missing
  Apple-specific signed-attributes that newer iOS versions check.
- **Roll our own ASN.1 / CMS encoder**. Rejected: PKCS#7 is a
  notoriously easy spec to get wrong (broken signatures fail silently
  in production months later). The 80 lines of code we'd save by not
  importing a library are worth less than the months of debugging
  saved by not getting CMS encoding wrong.
- **Shell out to `openssl smime -sign`**. Rejected: introduces a
  non-Go runtime dependency (we'd have to install openssl in the
  install.sh dependencies), and the openssl CLI's API for plist
  signing is undocumented enough that we'd be reverse-engineering it
  for every iOS/openssl version combination.

## Consequences

- New direct dep in `backend/gateway/go.mod`. Periodic `go get -u`
  to keep it current.
- If digitorus archives the project, we have an out: API is identical
  to `go.mozilla.org/pkcs7`, so a one-line import swap reverts the
  decision. Not strictly tied.
- We accept that PKCS#7 signing happens server-side per request.
  Could be cached if it ever becomes hot — currently every CA-cert
  download generates a fresh signed profile (sub-millisecond).

## References

- Issue [#19](https://github.com/neochaotic/powerlab/issues/19) — Backend
  HTTPS spec, "Open question" #1.
- [digitorus/pkcs7](https://github.com/digitorus/pkcs7).
- [go.mozilla.org/pkcs7](https://github.com/mozilla-services/pkcs7) —
  the legacy upstream we forked away from.
- [Apple Configuration Profile Reference](https://developer.apple.com/business/documentation/Configuration-Profile-Reference.pdf).
