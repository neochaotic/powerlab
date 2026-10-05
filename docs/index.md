# PowerLab

> One pane of glass for everything you self-host. Apps, files, AI — your home server, finally beautiful. Plus a built-in **MCP server** so your AI agents (Claude, Cursor, Code) read the same data the dashboard shows you.

PowerLab is an open-source self-hosted server panel. Run it on a Pi, a mini-PC, or any Linux box you already have, and get a single web UI for every Docker app on your machine, your files, a built-in AI assistant — and an **agent-ready surface** that turns the entire homelab into an MCP resource an LLM can reason over.

This is the technical reference. For installation tldr, jump to **[Getting started → Install](getting-started/install.md)**. For the marketing-style intro, see the [project README on GitHub](https://github.com/neochaotic/powerlab). For the MCP angle in 5 minutes, jump straight to **[MCP operator quickstart](operations/mcp-quickstart.md)**.

## What's here

**For operators**

- **[Install](getting-started/install.md)** — install, first boot, in-app updates, and putting PowerLab behind a [reverse proxy](operations/reverse-proxy.md).
- **[Use](operations/backup-restore.md)** — backup and restore, the security model, the REST API portal, the glossary.
- **[Apps](architecture/community-catalog.md)** — how the bundled catalogue works and the [compose conventions](concepts/compose-conventions.md) apps follow.
- **[AI / MCP](concepts/mcp-server.md)** — the Model Context Protocol server: resources, tools, threat model, and Claude Desktop / Cursor / Code wire-up. Quickstart at **[MCP operator quickstart](operations/mcp-quickstart.md)**.
- **[Troubleshoot](troubleshooting.md)** — common failures, [lockout recovery](operations/lockout-recovery.md), and the [`powerlab-logs`](operations/powerlab-logs.md) survival CLI.
- **[Migrate from CasaOS](coexistence/README.md)** — PowerLab forked from CasaOS; both can run on the same host, and apps can move over one at a time.

**For contributors**

- **[Contributors](getting-started/contributing-guide.md)** — contributor guide, architecture, release process, Go API reference, audits, and every architectural decision record ([ADR index](decisions/README.md)). Sprint plans and retrospectives are still published under `audits/` for link stability but are kept out of the navigation.

## Project status

PowerLab is in **beta** (latest release v0.7.7). The 0.x line means breaking changes can ship between minor versions; we document them in the [release manifest](UPDATE_MANIFEST.md) and the in-app updater surfaces them as a confirmation gate. It is aimed at someone running a Raspberry Pi or mini-PC who is comfortable with Docker.

## Where things live in this site

The repo's `docs/` directory IS the source for this site. Every page is a markdown file you can `git blame` straight to the commit that added it. The mkdocs build is a thin presentation layer on top.

Some files in `docs/` aren't listed in the navigation — they're still reachable by URL (so links from PRs and ADRs continue to resolve) but the curated nav shows the highlights.
