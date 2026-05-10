# Security policy

## Reporting a vulnerability

If you find a security issue in PowerLab, **do not open a public GitHub issue**. Email the maintainer directly:

- `alisson.david.rosa@gmail.com` (subject prefix `[powerlab security]`)

Expect an acknowledgement within 5 business days. We'll coordinate a fix + disclosure timeline.

For the avoidance of doubt: PowerLab is forked from CasaOS but is **not** the CasaOS project. Do not send PowerLab security reports to `wiki@casaos.io` or any IceWhaleTech address — see [ADR-0022](docs/decisions/0022-casaos-upstream-is-abandoned-no-new-dependencies.md) for the relationship.

## Supported versions

PowerLab is in active pre-1.0 development (current line: v0.5.x). Security fixes ship in the latest minor release; older 0.5.x releases do **not** get back-ported. After v1.0 we'll publish a real support matrix.

## What's in scope

- Anything in this repository
- The pre-built tarballs published as GitHub Releases
- The install scripts at `https://raw.githubusercontent.com/neochaotic/powerlab/main/install*.sh`

## What's out of scope

- Vulnerabilities in third-party app images PowerLab can install (report those to the upstream image)
- Issues that require physical access to the host
- Anything affecting only the upstream `IceWhaleTech/CasaOS` project
