# PowerLab

> One pane of glass for everything you self-host. Apps, files, AI — your home server, finally beautiful.

PowerLab is an open-source self-hosted server panel. Run it on a Pi, a mini-PC, or any Linux box you already have, and get a single web UI for every Docker app on your machine, your files, and a built-in AI assistant.

This is the technical reference. For the install tldr, jump to **[Getting started → Install](getting-started/install.md)**. For the marketing-style intro, see the [project README on GitHub](https://github.com/neochaotic/powerlab).

## What's here

| Start here | Build on the system | Why we built it that way |
|---|---|---|
| [Install](getting-started/install.md) | [Architecture overview](architecture/README.md) | [Decisions (ADRs)](decisions/README.md) |
| One curl, one box, one panel. | The six Go services, the SvelteKit SPA, how they compose. | Every load-bearing call recorded with context and consequences. |

The full nav (top of the page) covers the rest: first-boot, updating, the foundation packages each service stands on, the data persistence map, the CasaOS coexistence story, the operations runbooks, and the point-in-time engineering audits used to scope structural work.

## Project status

PowerLab is in active **pre-1.0 development**. The 0.x line means breaking changes can ship between minor versions; they are documented in the [release manifest](UPDATE_MANIFEST.md), gated by an in-app confirmation, and held to a rule that v1.0 is not tagged without explicit alignment.

The current focus is the **CasaOS-strip wave** — replacing CasaOS-specific assumptions with PowerLab-owned ones, service by service. The live tracker is the [strangler page](architecture/casaos-strangler.md); per-sprint progress lives in `docs/audits/sprint-N-*` documents.

## Where things live in this site

The repo's `docs/` directory IS the source for this site. Every page is a markdown file you can `git blame` straight to the commit that added it. The mkdocs build is a thin presentation layer on top.

Some files in `docs/` aren't listed in the navigation — they're still reachable by URL (so links from PRs and ADRs continue to resolve), but the curated nav shows the highlights.
