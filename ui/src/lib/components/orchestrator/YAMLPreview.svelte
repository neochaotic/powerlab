<script lang="ts">
	import yaml from 'js-yaml';
	import { Box, Server, Plug, HardDrive, AlertCircle } from 'lucide-svelte';

	/**
	 * Read-only preview derived from a compose YAML string.
	 *
	 * Shows a summary panel — service count, name, image, port mappings,
	 * volume count — so the user can verify the YAML they pasted looks
	 * right BEFORE clicking Deploy. The page that owns the YAML state
	 * passes it in as a prop; we re-derive on every change via $derived.
	 *
	 * Handles bad / incomplete YAML gracefully — a parse error renders
	 * a small inline notice; the page can still attempt deploy and let
	 * the backend surface the real error.
	 */

	let { yaml: yamlText }: { yaml: string } = $props();

	interface PreviewInfo {
		projectName: string | null;
		services: Array<{
			name: string;
			image: string;
			ports: string[];
			volumes: number;
			environment: number;
		}>;
		volumeKeys: string[];
		networkKeys: string[];
		parseError: string | null;
	}

	const info = $derived.by<PreviewInfo>(() => {
		const out: PreviewInfo = {
			projectName: null,
			services: [],
			volumeKeys: [],
			networkKeys: [],
			parseError: null
		};
		if (!yamlText.trim()) {
			out.parseError = 'empty';
			return out;
		}
		try {
			const parsed = yaml.load(yamlText) as Record<string, unknown> | null;
			if (!parsed || typeof parsed !== 'object') {
				out.parseError = 'not an object';
				return out;
			}
			out.projectName = typeof parsed.name === 'string' ? parsed.name : null;
			const services = parsed.services as Record<string, Record<string, unknown>> | undefined;
			if (services && typeof services === 'object') {
				for (const [name, svc] of Object.entries(services)) {
					out.services.push({
						name,
						image: typeof svc.image === 'string' ? svc.image : '',
						ports: normalizePorts(svc.ports),
						volumes: normalizeCount(svc.volumes),
						environment: normalizeEnvCount(svc.environment)
					});
				}
			}
			const volumes = parsed.volumes as Record<string, unknown> | undefined;
			if (volumes && typeof volumes === 'object') {
				out.volumeKeys = Object.keys(volumes);
			}
			const networks = parsed.networks as Record<string, unknown> | undefined;
			if (networks && typeof networks === 'object') {
				out.networkKeys = Object.keys(networks);
			}
		} catch (e) {
			out.parseError = (e as Error).message;
		}
		return out;
	});

	function normalizePorts(p: unknown): string[] {
		if (!Array.isArray(p)) return [];
		return p.map((entry) => {
			if (typeof entry === 'string') return entry;
			if (entry && typeof entry === 'object') {
				const obj = entry as { published?: number | string; target?: number | string };
				if (obj.published !== undefined && obj.target !== undefined) {
					return `${obj.published}:${obj.target}`;
				}
			}
			return '';
		}).filter(Boolean);
	}

	function normalizeCount(v: unknown): number {
		if (Array.isArray(v)) return v.length;
		if (v && typeof v === 'object') return Object.keys(v).length;
		return 0;
	}

	function normalizeEnvCount(v: unknown): number {
		if (Array.isArray(v)) return v.length;
		if (v && typeof v === 'object') return Object.keys(v).length;
		return 0;
	}
</script>

<div class="flex flex-col gap-3 px-4 py-3 text-zinc-200" data-testid="yaml-preview">
	{#if info.parseError === 'empty'}
		<p class="text-[11px] text-zinc-500">Paste a compose YAML to see a summary here.</p>
	{:else if info.parseError}
		<div class="flex items-start gap-2 rounded-lg border border-amber-400/20 bg-amber-400/5 px-3 py-2 text-[11px] text-amber-300">
			<AlertCircle class="h-4 w-4 shrink-0" />
			<span data-testid="yaml-preview-parse-error">Could not parse YAML: {info.parseError}</span>
		</div>
	{:else}
		<header class="flex items-center justify-between border-b border-white/[0.05] pb-2">
			<div class="flex items-center gap-2 text-[11px] font-medium uppercase tracking-[0.18em] text-zinc-400">
				<Box class="h-3.5 w-3.5" />
				<span data-testid="yaml-preview-project-name">{info.projectName ?? '(unnamed project)'}</span>
			</div>
			<span class="text-[10px] text-zinc-500" data-testid="yaml-preview-service-count">
				{info.services.length} service{info.services.length === 1 ? '' : 's'}
			</span>
		</header>

		<ul class="space-y-2 text-[12px]">
			{#each info.services as svc (svc.name)}
				<li class="rounded-lg border border-white/[0.04] bg-white/[0.02] px-3 py-2">
					<div class="flex items-center gap-2 font-mono text-zinc-100">
						<Server class="h-3.5 w-3.5 text-zinc-400" />
						<span data-testid="yaml-preview-service-name">{svc.name}</span>
					</div>
					<div class="mt-1 ml-5 text-[11px] text-zinc-400" data-testid="yaml-preview-service-image">{svc.image || '(no image)'}</div>
					<div class="mt-2 ml-5 flex flex-wrap gap-3 text-[10px] text-zinc-500">
						{#if svc.ports.length > 0}
							<span class="flex items-center gap-1" data-testid="yaml-preview-service-ports">
								<Plug class="h-3 w-3" />
								{svc.ports.join(', ')}
							</span>
						{/if}
						{#if svc.volumes > 0}
							<span class="flex items-center gap-1" data-testid="yaml-preview-service-volumes">
								<HardDrive class="h-3 w-3" />
								{svc.volumes} volume{svc.volumes === 1 ? '' : 's'}
							</span>
						{/if}
						{#if svc.environment > 0}
							<span data-testid="yaml-preview-service-env">{svc.environment} env</span>
						{/if}
					</div>
				</li>
			{/each}
		</ul>

		{#if info.volumeKeys.length > 0}
			<p class="text-[10px] text-zinc-500" data-testid="yaml-preview-top-volumes">
				Named volumes: {info.volumeKeys.join(', ')}
			</p>
		{/if}
		{#if info.networkKeys.length > 0}
			<p class="text-[10px] text-zinc-500" data-testid="yaml-preview-top-networks">
				Networks: {info.networkKeys.join(', ')}
			</p>
		{/if}
	{/if}
</div>
