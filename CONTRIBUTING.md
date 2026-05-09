# Contributing to PowerLab

First off, thank you for considering contributing to PowerLab! It's people like you that make PowerLab such a great tool for the home server community.

## Tech Stack

- **Frontend:** SvelteKit (Svelte 5 Runes), TypeScript, Tailwind CSS v4, Lucide Icons.
- **Backend:** Go 1.25+, Echo v4, GORM, Docker SDK.
- **Testing:** Vitest (Frontend), Go `testing` package (Backend).

## Test rule (mandatory)

Every new feature lands with **test coverage** in the same commit:

- **Unit test** for the unit of logic introduced. Mock external
  dependencies (filesystem, network, Docker, PAM). No "I'll write
  the tests later" — `validate.sh` is the gate, `validate.sh` runs
  tests, untested code fails the gate.
- **Regression test** for every bug fix. The test must reproduce the
  bug (i.e. fail on the parent commit) and pass on the fix commit.
  No exceptions, even for one-line fixes.
- **Integration test** when the change crosses process boundaries
  (gateway → service, frontend → backend, install pipeline). Goes
  in `scripts/test-<topic>.sh` and runs as part of
  `validate.sh --full`.

PRs that ship code without tests get sent back. The same rule applies
to my own work — when I add a feature, I add the tests in the same
commit, and `scripts/validate.sh --quick` must stay green.

## Documentation rule (mandatory)

Any commit that lands a user-visible change MUST also update:

- **A changelog fragment under `.changes/unreleased/`** — run
  `changie new` to create one interactively (or hand-write a YAML
  file matching the format in existing fragments). The fragment
  declares its `kind` (Added / Changed / Deprecated / Removed /
  Fixed / Security / Internal) and a prose `body`. Two PRs never
  edit the same file, so this eliminates the "merge conflict on
  CHANGELOG.md" class entirely. See `.changes/header.tpl.md` for
  the format.
- **`README.md`** — when the change affects what's promised on the
  product page (install command, supported platforms, headline
  features). Skip for purely-internal refactors.
- **`SUPPORT.md`** — when the change affects platform support, distro
  compatibility, hardware tiers, or auth mechanism.

`CHANGELOG.md` is **generated** at release time by `changie batch
<version>`, which aggregates the unreleased fragments into a new
section and archives them under `.changes/<version>/`. **Do not
hand-edit `CHANGELOG.md`** unless you're fixing a typo in an
already-released section. CI gates that every code-touching PR
includes at least one fragment.

To install changie locally:

    go install github.com/miniscruff/changie@latest

Then `changie new` (interactive) or just drop a `.yaml` file like:

    kind: Fixed
    body: |
      Files: save toast invisible because z-index conflict between
      toast container and editor modal.
    custom:
      Issue: "3"

For roadmap items tracked as GitHub issues, the issue itself is the
working spec. As features land, copy the salient bits from the issue
into `CHANGELOG.md` + the relevant doc — close the issue with a
comment pointing at the release tag.

## Pre-push validation

Before pushing anything to `main`, run:

```bash
./scripts/validate.sh           # ~3 minutes — same checks the CI matrix runs
./scripts/validate.sh --quick   # ~30 seconds — frontend + native go test only
./scripts/validate.sh --full    # ~7 minutes — also runs the full package smoke + Docker CGO
```

The script bails on the first failure and prints exactly what broke. It catches roughly everything CI catches except real-arm64 hardware quirks and timing-sensitive race tests, so a green local run is a strong signal that CI will go green too. Run `--full` before tagging a release.

## Development Workflow

### Prerequisites

- [Go](https://golang.org/doc/install) 1.21 or higher.
- [Node.js](https://nodejs.org/en/download/) (v18+ recommended) and `npm`.
- [Docker](https://docs.docker.com/get-docker/) installed and running.

### Setting up the Backend

1. Navigate to the root directory.
2. Run the start script to initialize and build all services:
   ```bash
   ./start.sh --build
   ```
3. The gateway will start on `http://localhost:80` (or `8089` depending on config).

### Setting up the Frontend

1. Navigate to the `ui/` directory:
   ```bash
   cd ui
   ```
2. Install dependencies:
   ```bash
   npm install
   ```
3. Start the development server:
   ```bash
   npm run dev
   ```
4. Access the UI at `http://localhost:5173`.

## Coding Standards

### Frontend (Svelte 5)

- **Runes Only:** Use `$state`, `$derived`, and `$effect`. Legacy Svelte 4 stores are discouraged.
- **Derived Expressions:** Always use `$derived.by(() => { ... })` for multi-line logic.
- **Components:** Use snippets (`{#snippet}`) instead of slots.
- **Styling:** Use Tailwind CSS v4 utility classes. Design tokens are in `ui/src/app.css`.

### Backend (Go)

- **Standardized Responses:** Use `codegen.Response` for all API handlers to ensure structured JSON output.
- **Error Handling:** Return descriptive error messages.
- **Path Resolution:** Always use the portable path resolution logic defined in `backend/common/utils/constants/paths.go`.

## Testing

- **Backend:** Run `go test ./...` in the specific service directory (e.g., `backend/core`).
- **Frontend:** Run `npm run check` and `npx vitest` in the `ui/` directory.

## Pull Request Process

1. Fork the repository and create your branch from `main`.
2. Ensure your code passes all tests and lint checks.
3. Update the documentation if you're adding or changing features.
4. Submit a PR with a clear description of the changes and the problem they solve.

## License

By contributing, you agree that your contributions will be licensed under the PolyForm Noncommercial License 1.0.0.
