# PowerLab

> One pane of glass for everything you self-host. Apps, files, AI — your home server, finally beautiful.

PowerLab is an open-source self-hosted server panel. Run it on a Pi, a mini-PC, or any Linux box you already have, and get a single web UI for every Docker app on your machine, your files, and a built-in AI assistant.

This is the technical reference. For installation tldr, jump to **[Getting started → Install](getting-started/install.md)**. For the marketing-style intro, see the [project README on GitHub](https://github.com/neochaotic/powerlab).

## What's here

- **[Getting started](getting-started/install.md)** — install, first-boot, in-app updates, contributor guide.
- **[Architecture](architecture/README.md)** — service topology, request lifecycle, the foundation packages every service uses, the data persistence model.
- **[Coexistence with CasaOS](coexistence/README.md)** — PowerLab forked from CasaOS; both can run on the same host. Here's how.
- **[Concepts](concepts/glossary.md)** — glossary of project vocabulary and the security model.
- **[Operations](HTTPS.md)** — HTTPS setup, backup and restore, the update manifest contract, release checklist, troubleshooting.
- **[Audits](audits/db-paths.md)** — point-in-time engineering audits used by the team to plan structural work. Useful when reading a follow-up PR or wondering "why is X built that way".
- **[Decisions (ADRs)](decisions/README.md)** — every architectural decision recorded, with context and consequences. The first place to look when "why" matters.

## Project status

PowerLab is in active pre-1.0 development. The major-version 0.x line means breaking changes can ship between minor versions; we document them in the [release manifest](UPDATE_MANIFEST.md) and the in-app updater surfaces them as a confirmation gate.

The current focus is the **CasaOS-strip** wave — replacing CasaOS-specific assumptions with PowerLab-owned ones, service by service. Sprint progress lives in `docs/audits/sprint-N-*` documents.

## Where things live in this site

The repo's `docs/` directory IS the source for this site. Every page is a markdown file you can `git blame` straight to the commit that added it. The mkdocs build is a thin presentation layer on top.

Some files in `docs/` aren't listed in the navigation — they're still reachable by URL (so links from PRs and ADRs continue to resolve) but the curated nav shows the highlights.
