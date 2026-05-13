<script lang="ts">
	import {
		RefreshCw, ExternalLink, Sparkles, Container, Zap, Heart,
		Code2, Info, Scale, Boxes
	} from 'lucide-svelte';
	import { cn } from '$lib/utils';
	import { t } from '$lib/i18n/index.svelte';
	import { updaterStore } from '$lib/stores/updater.svelte';
	import { upgradeProgress } from '$lib/stores/upgradeProgress.svelte';
	import { summarizeReleaseNotes } from '$lib/utils/release-notes-summary';

	let summaryExpanded = $state(false);
	const summary = $derived(
		summarizeReleaseNotes(updaterStore.check?.release_summary ?? '')
	);

	function startUpgrade() {
		const target = updaterStore.check?.available;
		if (!target) return;
		upgradeProgress.start(target);
	}

</script>

<div class="space-y-8">
	<!-- Hero -->
	<div class="relative overflow-hidden rounded-3xl border border-white/[0.06] bg-gradient-to-br from-zinc-950 via-zinc-950 to-emerald-950/30 p-8">
		<!-- Ambient glow -->
		<div class="pointer-events-none absolute -right-20 -top-20 h-64 w-64 rounded-full bg-emerald-500/[0.08] blur-3xl"></div>
		<div class="pointer-events-none absolute -bottom-20 -left-20 h-64 w-64 rounded-full bg-teal-500/[0.06] blur-3xl"></div>

		<div class="relative">
			<div class="mb-2 inline-flex items-center gap-1.5 rounded-full border border-emerald-400/20 bg-emerald-400/[0.08] px-2.5 py-1 text-[10px] font-medium uppercase tracking-widest text-emerald-300">
				<Sparkles class="h-3 w-3" />
				Pre-release
			</div>

			<h1 class="bg-gradient-to-br from-white via-white to-zinc-400 bg-clip-text text-5xl font-bold tracking-tight text-transparent">
				PowerLab<span class="bg-gradient-to-br from-emerald-300 to-teal-500 bg-clip-text">.</span>
			</h1>
			<p class="mt-3 max-w-xl text-base leading-relaxed text-zinc-400">
				The headless OS panel for home servers and edge boxes. Lightning-fast, minimal, and built to run flawlessly on hardware everyone else gave up on.
			</p>

			<div class="mt-6 flex flex-wrap items-center gap-2">
				<span class="rounded-lg border border-white/[0.08] bg-white/[0.03] px-2.5 py-1 font-mono text-[11px] text-zinc-300">v{__APP_VERSION__}</span>
				<span class="rounded-lg border border-white/[0.08] bg-white/[0.03] px-2.5 py-1 text-[11px] text-zinc-400">AGPL-3.0</span>
			</div>
		</div>
	</div>

	<!-- Updates card (issue #21) -->
	<div class="rounded-2xl border border-white/[0.06] bg-white/[0.02] p-5">
		<div class="mb-4 flex items-start justify-between gap-4">
			<div class="flex items-center gap-3">
				<div class="flex h-9 w-9 items-center justify-center rounded-xl bg-emerald-500/[0.12]">
					<RefreshCw class={cn('h-4 w-4 text-emerald-400', updaterStore.loading && 'animate-spin')} strokeWidth={2} />
				</div>
				<div>
					<h3 class="text-sm font-semibold text-white">Updates</h3>
					<p class="text-[11px] text-zinc-500">Checks the PowerLab GitHub release manifest hourly.</p>
				</div>
			</div>
			<button
				class="rounded-lg border border-white/[0.06] bg-white/[0.02] px-3 py-1.5 text-[11px] font-medium text-zinc-300 transition-colors hover:border-white/10 hover:bg-white/[0.04] hover:text-white disabled:opacity-50"
				onclick={() => updaterStore.refresh()}
				disabled={updaterStore.loading}
			>
				{updaterStore.loading ? 'Checking…' : 'Check now'}
			</button>
		</div>

		{#if updaterStore.error}
			<p class="text-[12px] text-amber-400">
				Could not reach the release manifest: {updaterStore.error}
			</p>
		{:else if updaterStore.check?.decision === 'up_to_date'}
			<p class="text-[12px] text-zinc-400">
				<span class="font-mono text-emerald-400">v{updaterStore.check.current}</span> is the latest release.
			</p>
		{:else if updaterStore.check?.decision === 'update_ok'}
			<div class="space-y-3">
				<p class="text-[13px] leading-relaxed text-zinc-300">
					<span class="font-mono text-emerald-400">v{updaterStore.check.available}</span> is available.
				</p>
				{#if updaterStore.check.release_summary}
					<div
						data-testid="release-summary"
						class="rounded-lg border border-white/[0.04] bg-white/[0.02] p-3 text-[12px] leading-relaxed text-zinc-400"
					>
						{#if summaryExpanded}
							<p class="whitespace-pre-wrap">{summary.full}</p>
						{:else}
							<p>{summary.text}</p>
						{/if}
						{#if summary.truncated}
							<button
								type="button"
								data-testid="release-summary-toggle"
								class="mt-2 text-[11px] font-semibold text-emerald-400 hover:text-emerald-300"
								onclick={() => (summaryExpanded = !summaryExpanded)}
							>
								{summaryExpanded ? t('about.showLess') : t('about.showMore')}
							</button>
						{/if}
					</div>
				{/if}
				{#if updaterStore.check.changelog_url}
					<a
						href={updaterStore.check.changelog_url}
						target="_blank"
						rel="noopener"
						class="inline-flex items-center gap-1 text-[11px] text-emerald-400 hover:text-emerald-300"
					>
						View changelog
						<ExternalLink class="h-3 w-3" />
					</a>
				{/if}
				<div class="flex flex-wrap items-center gap-2 pt-1">
					<button
						class="rounded-lg bg-emerald-500 px-3 py-1.5 text-[11px] font-bold text-zinc-950 transition-colors hover:bg-emerald-400 disabled:opacity-50"
						onclick={startUpgrade}
						disabled={upgradeProgress.isOverlayActive}
					>
						{upgradeProgress.isOverlayActive ? 'Upgrading…' : `Upgrade to v${updaterStore.check.available}`}
					</button>
				</div>
				{#if updaterStore.installError}
					<p class="text-[11px] text-amber-400">
						{updaterStore.installError}
					</p>
				{/if}
			</div>
		{:else if updaterStore.check?.decision === 'too_old'}
			<p class="text-[12px] text-amber-400">
				Cannot upgrade directly from
				<span class="font-mono">v{updaterStore.check.current}</span>
				to
				<span class="font-mono">v{updaterStore.check.available}</span>.
				Upgrade to an intermediate release first (manifest requires
				<span class="font-mono">v{updaterStore.check.manifest?.min_upgrade_from}+</span>).
			</p>
		{:else if updaterStore.check?.decision === 'skipped'}
			<p class="text-[12px] text-zinc-500">
				The maintainer pulled <span class="font-mono">v{updaterStore.check.available}</span> after publishing it. Wait for the next release.
			</p>
		{:else if updaterStore.check?.decision === 'no_arch'}
			<p class="text-[12px] text-amber-400">
				<span class="font-mono">v{updaterStore.check.available}</span> does not ship a build for this architecture. The maintainer will publish a patch.
			</p>
		{:else}
			<p class="text-[12px] text-zinc-500">Click "Check now" to fetch the latest manifest.</p>
		{/if}
	</div>

	<!-- Highlights -->
	<div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
		<div class="rounded-2xl border border-white/[0.06] bg-white/[0.02] p-4">
			<div class="mb-3 flex h-9 w-9 items-center justify-center rounded-xl bg-emerald-500/[0.12]">
				<Zap class="h-4 w-4 text-emerald-400" strokeWidth={2} />
			</div>
			<h3 class="text-sm font-semibold text-white">Zero bloat</h3>
			<p class="mt-1 text-[12px] leading-relaxed text-zinc-500">SvelteKit SPA. No virtual DOM weight, no SSR runtime, sub-second renders.</p>
		</div>
		<div class="rounded-2xl border border-white/[0.06] bg-white/[0.02] p-4">
			<div class="mb-3 flex h-9 w-9 items-center justify-center rounded-xl bg-blue-500/[0.12]">
				<Container class="h-4 w-4 text-blue-400" strokeWidth={2} />
			</div>
			<h3 class="text-sm font-semibold text-white">Docker-native</h3>
			<p class="mt-1 text-[12px] leading-relaxed text-zinc-500">Compose builder, app store, log streaming, auto-port remap. All wired into your daemon.</p>
		</div>
		<div class="relative overflow-hidden rounded-2xl border border-blue-400/20 bg-gradient-to-br from-blue-500/[0.08] to-violet-500/[0.06] p-4">
			<div class="pointer-events-none absolute -right-6 -top-6 h-24 w-24 rounded-full bg-blue-500/[0.15] blur-2xl"></div>
			<div class="relative">
				<div class="mb-3 flex h-9 w-9 items-center justify-center rounded-xl bg-blue-500/[0.15]">
					<Sparkles class="h-4 w-4 text-blue-300" strokeWidth={2} />
				</div>
				<div class="flex items-center gap-1.5">
					<h3 class="text-sm font-semibold text-white">AI-ready</h3>
					<span class="rounded-md bg-blue-400/20 px-1.5 py-0.5 text-[9px] font-bold uppercase tracking-wider text-blue-200">Soon</span>
				</div>
				<p class="mt-1 text-[12px] leading-relaxed text-zinc-400">Run Ollama, Stable Diffusion, ChatGPT-Next-Web. GPU auto-detected. Models tab landing soon.</p>
			</div>
		</div>
		<div class="rounded-2xl border border-white/[0.06] bg-white/[0.02] p-4">
			<div class="mb-3 flex h-9 w-9 items-center justify-center rounded-xl bg-violet-500/[0.12]">
				<Heart class="h-4 w-4 text-violet-400" strokeWidth={2} />
			</div>
			<h3 class="text-sm font-semibold text-white">Self-hosted</h3>
			<p class="mt-1 text-[12px] leading-relaxed text-zinc-500">Your data, your box, your rules. Sign in with your OS credentials — no cloud account.</p>
		</div>
	</div>

	<!-- Built with -->
	<div>
		<h3 class="mb-3 text-[11px] font-semibold uppercase tracking-widest text-zinc-500">Built with</h3>
		<div class="flex flex-wrap gap-2">
			{#each ['SvelteKit', 'Svelte 5 Runes', 'TypeScript', 'Tailwind v4', 'Lucide', 'xterm.js', 'Go 1.21', 'Echo v4', 'Docker Compose', 'JWT + bcrypt', 'mDNS / Bonjour'] as tech}
				<span class="rounded-lg border border-white/[0.06] bg-white/[0.02] px-2.5 py-1 text-[11px] font-medium text-zinc-300">{tech}</span>
			{/each}
		</div>
	</div>

	<!-- Resources -->
	<div>
		<h3 class="mb-3 text-[11px] font-semibold uppercase tracking-widest text-zinc-500">Resources</h3>
		<div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
			<a
				href="https://github.com/neochaotic/powerlab"
				target="_blank"
				rel="noopener"
				class="group flex items-center justify-between gap-3 rounded-xl border border-white/[0.06] bg-white/[0.02] px-4 py-3 transition-all hover:border-white/10 hover:bg-white/[0.04]"
			>
				<div class="flex items-center gap-3">
					<div class="flex h-9 w-9 items-center justify-center rounded-lg bg-white/[0.04] text-zinc-300 transition-colors group-hover:text-white">
						<Code2 class="h-4 w-4" />
					</div>
					<div>
						<p class="text-sm font-medium text-white">Source code</p>
						<p class="text-[11px] text-zinc-500">github.com/neochaotic/powerlab</p>
					</div>
				</div>
				<ExternalLink class="h-3.5 w-3.5 text-zinc-600 transition-colors group-hover:text-zinc-300" />
			</a>
			<a
				href="https://github.com/neochaotic/powerlab/issues"
				target="_blank"
				rel="noopener"
				class="group flex items-center justify-between gap-3 rounded-xl border border-white/[0.06] bg-white/[0.02] px-4 py-3 transition-all hover:border-white/10 hover:bg-white/[0.04]"
			>
				<div class="flex items-center gap-3">
					<div class="flex h-9 w-9 items-center justify-center rounded-lg bg-white/[0.04] text-zinc-300 transition-colors group-hover:text-white">
						<Info class="h-4 w-4" />
					</div>
					<div>
						<p class="text-sm font-medium text-white">Report an issue</p>
						<p class="text-[11px] text-zinc-500">Bugs, requests, ideas</p>
					</div>
				</div>
				<ExternalLink class="h-3.5 w-3.5 text-zinc-600 transition-colors group-hover:text-zinc-300" />
			</a>
			<a
				href="https://github.com/neochaotic/powerlab/blob/main/LICENSE"
				target="_blank"
				rel="noopener"
				class="group flex items-center justify-between gap-3 rounded-xl border border-white/[0.06] bg-white/[0.02] px-4 py-3 transition-all hover:border-white/10 hover:bg-white/[0.04]"
			>
				<div class="flex items-center gap-3">
					<div class="flex h-9 w-9 items-center justify-center rounded-lg bg-white/[0.04] text-zinc-300 transition-colors group-hover:text-white">
						<Scale class="h-4 w-4" />
					</div>
					<div>
						<p class="text-sm font-medium text-white">License</p>
						<p class="text-[11px] text-zinc-500">GNU Affero General Public License v3.0</p>
					</div>
				</div>
				<ExternalLink class="h-3.5 w-3.5 text-zinc-600 transition-colors group-hover:text-zinc-300" />
			</a>
			<a
				href="https://github.com/neochaotic/powerlab"
				target="_blank"
				rel="noopener"
				class="group flex items-center justify-between gap-3 rounded-xl border border-white/[0.06] bg-white/[0.02] px-4 py-3 transition-all hover:border-white/10 hover:bg-white/[0.04]"
			>
				<div class="flex items-center gap-3">
					<div class="flex h-9 w-9 items-center justify-center rounded-lg bg-white/[0.04] text-zinc-300 transition-colors group-hover:text-white">
						<Boxes class="h-4 w-4" />
					</div>
					<div>
						<p class="text-sm font-medium text-white">PowerLab on GitHub</p>
						<p class="text-[11px] text-zinc-500">Source, releases, issues</p>
					</div>
				</div>
				<ExternalLink class="h-3.5 w-3.5 text-zinc-600 transition-colors group-hover:text-zinc-300" />
			</a>
			<div
				class="col-span-1 sm:col-span-2 group flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-4 rounded-xl border border-emerald-500/20 bg-emerald-500/[0.02] px-4 py-3 transition-all hover:border-emerald-500/30 hover:bg-emerald-500/[0.04]"
			>
				<div class="flex items-center gap-3">
					<div class="flex h-9 w-9 items-center justify-center rounded-lg bg-emerald-500/10 text-emerald-400 transition-colors group-hover:text-emerald-300">
						<Code2 class="h-4 w-4" />
					</div>
					<div>
						<p class="text-sm font-medium text-white">{t('settings.apiDocs')}</p>
						<p class="text-[11px] text-zinc-500">{t('settings.apiDocsDesc')}</p>
					</div>
				</div>
				<div class="flex gap-2">
					<a
						href="/docs#access_token={localStorage.getItem('powerlab_token')}"
						target="_blank"
						class="flex items-center justify-center gap-2 rounded-lg bg-emerald-500/10 px-4 py-1.5 text-[11px] font-bold text-emerald-400 transition-colors hover:bg-emerald-500/20"
					>
						Open API Portal
						<ExternalLink class="h-3 w-3" />
					</a>
				</div>
			</div>
		</div>
	</div>

	<!-- Footer -->
	<div class="flex items-center justify-between gap-4 border-t border-white/[0.04] pt-6 text-[11px] text-zinc-600">
		<div class="flex items-center gap-1.5">
			<span>Crafted with</span>
			<Heart class="h-3 w-3 text-rose-500/80" fill="currentColor" />
			<span>by</span>
			<a
				href="https://github.com/neochaotic"
				target="_blank"
				rel="noopener noreferrer"
				class="text-zinc-400 underline-offset-2 hover:text-emerald-400 hover:underline transition-colors"
			>
				neochaotic
			</a>
		</div>
		<span>© {new Date().getFullYear()} PowerLab</span>
	</div>
</div>
