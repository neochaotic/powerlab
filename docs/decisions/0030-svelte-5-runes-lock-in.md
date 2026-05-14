# 0030. Svelte 5 Runes Lock-in

- **Status:** accepted
- **Date:** 2026-05-14

## Context

The UI layer is a single-page application built with SvelteKit (`adapter-static`). We are migrating from Svelte 4 to Svelte 5. Svelte 5 introduces "Runes" (`$state`, `$derived`, `$effect`) as a fundamental shift in how reactivity is handled, replacing the older `svelte/store` (`writable`, `readable`) paradigm and `export let` component props.

Mixing Svelte 4 reactivity patterns with Svelte 5 Runes leads to unpredictable state updates, hard-to-debug reactivity bugs, and a fractured developer experience.

## Decision

We enforce a **strict Svelte 5 Runes lock-in** across the entire `ui/` codebase.

- **No Svelte 4 stores:** Usage of `svelte/store` (`writable`, `readable`, `derived`) is strictly forbidden. All global or shared state must be managed via Svelte 5 runes (`$state` inside `.svelte.ts` files).
- **No Svelte 4 component props:** Usage of `export let` is forbidden. All component inputs must be defined using the `$props()` rune.
- **Strict `index.svelte.ts` naming:** Our internal i18n system relies on reactive objects exported from a `.svelte.ts` file. These files must retain the `.svelte.ts` extension to ensure the Svelte compiler correctly processes the Runes within them.

## Consequences

- **Positive:** A single, consistent reactivity model across the entire frontend.
- **Positive:** Better performance and finer-grained reactivity provided by Runes.
- **Negative:** A steep learning curve for contributors familiar with older Svelte versions.
- **Constraint:** All future UI PRs must be audited to reject Svelte 4 syntax.
