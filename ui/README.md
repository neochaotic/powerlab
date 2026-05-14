# PowerLab UI

The web interface for PowerLab — a SvelteKit single-page application built with Svelte 5 Runes.

## Tech Stack

- **Framework:** [SvelteKit](https://kit.svelte.dev) with [`adapter-static`](https://kit.svelte.dev/docs/adapter-static) (no SSR — fully static SPA)
- **Language:** TypeScript
- **Reactivity:** Svelte 5 Runes (`$state`, `$derived`, `$effect`) — legacy Svelte 4 stores are not used
- **Styling:** [Tailwind CSS v4](https://tailwindcss.com/) — design tokens defined in `src/app.css`
- **Icons:** [Lucide](https://lucide.dev/)
- **Code Editor:** [CodeMirror](https://codemirror.net/) (Files page inline editor)
- **Testing:** [Vitest](https://vitest.dev/) (unit) + [Playwright](https://playwright.dev/) (e2e)

## Getting Started

### Prerequisites

- Node.js 20+
- `npm`

### Development

```bash
npm install
npm run dev
```

The dev server runs at `http://localhost:5173` and proxies API calls to the gateway on port 80. See `vite.config.ts` for the proxy configuration.

> **Tip:** Use `./dev.sh` from the repo root to start the full stack (backend + UI) in one command.

### Build

```bash
npm run build
```

Produces a static bundle in `build/` — this is what the gateway serves in production.

## Project Structure

```
src/
├── app.css              # Tailwind v4 config + design tokens
├── app.html             # HTML shell
├── lib/
│   ├── components/      # Reusable UI components
│   │   ├── apps/        # App store + install UI
│   │   ├── dashboard/   # Dashboard widgets (gauges, sparklines)
│   │   ├── files/       # File manager components
│   │   └── settings/    # Settings panes (extracted from god-page)
│   ├── stores/          # Svelte 5 rune-based stores
│   ├── utils/           # Pure utility functions
│   └── i18n/            # Internationalization (en, pt-BR, es)
├── routes/
│   ├── +page.svelte     # Launchpad (home)
│   ├── apps/            # App store + install flow
│   ├── dashboard/       # System monitoring
│   ├── files/           # File manager
│   └── settings/        # Settings (5 extracted panes)
└── tests/               # Playwright e2e tests
```

## Conventions

### Components

- Use **snippets** (`{#snippet}`) instead of slots.
- Use `$derived.by(() => { ... })` for multi-line derived logic.
- Keep components focused — large pages should be split into pane components (see `settings/` as the pattern).

### Styling

- Use Tailwind CSS v4 utility classes.
- Design tokens (colors, spacing, typography) are in `src/app.css`.
- Avoid inline `style` attributes.

### i18n

All user-facing strings go through `$lib/i18n`. Three locales are maintained: `en`, `pt-BR`, `es`. Use `{t('key.path')}` in templates.

## Testing

```bash
# Unit tests
npx vitest run

# Unit tests in watch mode
npx vitest

# Coverage report
npm run test:coverage

# Type checking
npm run check

# Playwright e2e
npx playwright test
```

## Related Documentation

- [CONTRIBUTING.md](../CONTRIBUTING.md) — coding standards, PR process
- [Architecture overview](../docs/architecture/README.md) — how the UI fits into the system
- [API reference](../docs/operations/api-reference.md) — the REST endpoints the UI consumes
