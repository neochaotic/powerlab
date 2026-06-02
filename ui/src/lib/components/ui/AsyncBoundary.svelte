<script lang="ts">
	import type { Snippet } from 'svelte';

	// Pattern extracted from the 5 settings panes (CatalogPane, PowerPane,
	// LogsPane, AuditPane, …) that each rendered their own
	// `{#if loading}…{:else if empty}…{:else}…` cascade in the content
	// area. Centralising this:
	//   1. removes ~6-10 lines of state-branch boilerplate per pane,
	//   2. gives every loading / empty state a consistent role + aria
	//      so screen readers + Playwright a11y assertions don't drift,
	//   3. lets the next pane built skip the design decision entirely.
	//
	// Scope: this component handles loading / empty / children only.
	// The error banner stays a per-pane concern — error chrome (placement,
	// retry button, icon, additional context) varies across panes enough
	// that a single shared design would force lossy compromises. When that
	// converges, factor an `<ErrorBanner>` peer.
	//
	// Precedence: error > loading > empty > children.

	interface Props {
		/** Optional error message — when set, replaces the rendered area. */
		error?: string | null;
		/** Show the loading state. */
		loading?: boolean;
		/** Treat the underlying data as empty. */
		empty?: boolean;
		/** Override the default empty copy. Defaults to a generic "Nothing here yet." */
		emptyText?: string;
		/** Override the default loading copy. Defaults to "Loading…". */
		loadingText?: string;
		/**
		 * Chrome variant for the loading + empty states:
		 *   "card"   — rounded-2xl border + padding (default; matches the
		 *              CatalogPane sources list, PowerPane services list).
		 *   "inline" — no border, just centered padding (matches the rows
		 *              inside table cards — AuditPane, LogsPane — where
		 *              the parent already provides the chrome).
		 * The error state always uses the card chrome — a banner is the
		 * right shape regardless of where it appears.
		 */
		variant?: 'card' | 'inline';
		/** Content rendered when not loading, not empty, no error. */
		children?: Snippet;
	}

	let {
		error = null,
		loading = false,
		empty = false,
		emptyText = 'Nothing here yet.',
		loadingText = 'Loading…',
		variant = 'card',
		children
	}: Props = $props();

	const cardChrome =
		'rounded-2xl border border-white/[0.06] bg-white/[0.02] p-6 text-center text-sm text-zinc-500';
	const inlineChrome = 'px-4 py-8 text-center text-sm text-zinc-500';
	const stateChrome = $derived(variant === 'card' ? cardChrome : inlineChrome);
</script>

{#if error}
	<div
		class="rounded-2xl border border-red-500/20 bg-red-500/[0.05] p-4 text-sm text-red-400"
		role="alert"
		data-testid="async-boundary-error"
	>
		{error}
	</div>
{:else if loading}
	<div
		class={stateChrome}
		aria-live="polite"
		aria-busy="true"
		data-testid="async-boundary-loading"
	>
		{loadingText}
	</div>
{:else if empty}
	<div class={stateChrome} data-testid="async-boundary-empty">
		{emptyText}
	</div>
{:else if children}
	{@render children()}
{/if}
