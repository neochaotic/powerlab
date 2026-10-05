<script lang="ts">
	import { onMount } from 'svelte';
	import { AlertTriangle, ExternalLink } from 'lucide-svelte';
	import { getDockerSystem } from '$lib/api/apps';
	import type { ApiError } from '$lib/api/client';
	import { t } from '$lib/i18n/index.svelte';
	import { cn } from '$lib/utils';

	/**
	 * Docker-missing banner (#63).
	 *
	 * Probes GET /v2/app_management/docker/system once on mount. That
	 * endpoint answers 503 when the Docker daemon is not installed or not
	 * running, so a 5xx means the App Store and Launchpad cannot work:
	 * show install hints instead of a silent empty grid. Auth errors,
	 * 404s (older backend) and network failures are someone else's
	 * banner, so they stay silent here.
	 */

	let { class: className = '' }: { class?: string } = $props();

	const DOCKER_INSTALL_DOCS = 'https://docs.docker.com/engine/install/';
	const POWERLAB_INSTALL_DOCS =
		'https://neochaotic.github.io/powerlab/getting-started/install/';

	let unavailable = $state(false);
	let reason = $state<string | null>(null);

	onMount(() => {
		getDockerSystem()
			.then(() => {
				unavailable = false;
			})
			.catch((e: unknown) => {
				const status = (e as Partial<ApiError>)?.status ?? 0;
				if (status >= 500) {
					unavailable = true;
					reason = (e as Partial<ApiError>)?.message ?? null;
				}
			});
	});
</script>

{#if unavailable}
	<div
		role="alert"
		data-testid="docker-unavailable-banner"
		class={cn(
			'flex items-start gap-3 rounded-2xl border border-amber-500/30 bg-amber-500/[0.06] p-4 text-sm',
			className
		)}
	>
		<AlertTriangle class="mt-0.5 h-4 w-4 shrink-0 text-amber-400" />
		<div class="min-w-0 space-y-2">
			<p class="font-semibold text-amber-200">{t('docker.unavailableTitle')}</p>
			<p class="text-xs leading-relaxed text-zinc-400">{t('docker.unavailableBody')}</p>
			<pre
				class="overflow-x-auto rounded-lg border border-white/[0.06] bg-black/40 p-2.5 font-mono text-[11px] text-zinc-300">curl -fsSL https://get.docker.com | sudo sh
sudo systemctl enable --now docker
sudo systemctl restart powerlab-app-management</pre>
			{#if reason}
				<p class="font-mono text-[11px] text-zinc-500" data-testid="docker-unavailable-reason">
					{reason}
				</p>
			{/if}
			<div class="flex flex-wrap gap-x-4 gap-y-1 text-[11px]">
				<a
					href={DOCKER_INSTALL_DOCS}
					target="_blank"
					rel="noopener"
					class="inline-flex items-center gap-1 font-medium text-amber-300 hover:text-amber-200"
				>
					{t('docker.installDocs')}
					<ExternalLink class="h-3 w-3" />
				</a>
				<a
					href={POWERLAB_INSTALL_DOCS}
					target="_blank"
					rel="noopener"
					class="inline-flex items-center gap-1 font-medium text-amber-300 hover:text-amber-200"
				>
					{t('docker.powerlabDocs')}
					<ExternalLink class="h-3 w-3" />
				</a>
			</div>
		</div>
	</div>
{/if}
