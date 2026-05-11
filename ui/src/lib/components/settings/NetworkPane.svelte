<script lang="ts">
	import { Check, Copy, Network } from 'lucide-svelte';
	import { cn } from '$lib/utils';

	interface NetworkInterface {
		name: string;
		state: string;
		type: string;
		ip?: string;
		mac?: string;
	}

	interface Props {
		mdnsHostname: string;
		reachableUrl: string;
		copiedKey: string | null;
		onCopy: (text: string, key: string) => void;
		networkInterfaces: NetworkInterface[];
	}

	let { mdnsHostname, reachableUrl, copiedKey, onCopy, networkInterfaces }: Props = $props();
</script>

<div>
	<header class="mb-8">
		<h1 class="text-2xl font-bold tracking-tight text-white">Network</h1>
		<p class="mt-1 text-sm text-zinc-500">How devices on your local network reach PowerLab.</p>
	</header>

	<!-- mDNS / Discovery -->
	<section class="mb-8">
		<h3 class="mb-3 text-[10px] font-bold uppercase tracking-[0.2em] text-zinc-500">Local discovery (mDNS)</h3>
		<div class="rounded-2xl border border-white/5 bg-white/[0.02] p-5">
			<div class="flex items-start justify-between gap-3">
				<div>
					<p class="text-sm font-medium text-white">{mdnsHostname}</p>
					<p class="mt-1 text-xs text-zinc-500">Announced via Bonjour/Avahi to all interfaces. macOS, iOS, modern Linux/Windows resolve this without configuration.</p>
				</div>
				<span class="inline-flex shrink-0 items-center gap-1.5 rounded-full bg-emerald-500/10 px-2.5 py-0.5 text-[11px] font-medium text-emerald-400">
					<span class="h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
					Active
				</span>
			</div>

			<div class="mt-4 flex items-center gap-2 rounded-lg bg-black/30 px-3 py-2 font-mono text-[12px] text-zinc-300">
				<span class="flex-1 truncate">{reachableUrl}</span>
				<button
					class="flex h-6 w-6 items-center justify-center rounded-md text-zinc-500 transition-colors hover:bg-white/5 hover:text-white"
					onclick={() => onCopy(reachableUrl, 'mdns')}
					aria-label="Copy URL"
				>
					{#if copiedKey === 'mdns'}
						<Check class="h-3.5 w-3.5 text-emerald-400" />
					{:else}
						<Copy class="h-3.5 w-3.5" />
					{/if}
				</button>
			</div>
			<p class="mt-3 text-[11px] text-zinc-600">
				HTTPS via self-signed cert is disabled by default. Browsers will warn about HTTP. For warning-free HTTPS, install a local CA via <span class="text-zinc-400">mkcert</span>.
			</p>
		</div>
	</section>

	<!-- Interfaces -->
	<section>
		<h3 class="mb-3 text-[10px] font-bold uppercase tracking-[0.2em] text-zinc-500">Network interfaces</h3>
		<div class="overflow-hidden rounded-2xl border border-white/5 bg-white/[0.02]">
			{#each networkInterfaces as iface, i}
				<div class={cn(
					"flex items-center justify-between gap-4 px-5 py-4",
					i > 0 && "border-t border-white/5"
				)}>
					<div class="min-w-0 flex-1">
						<div class="flex items-center gap-2">
							<span class="text-sm font-medium text-white">{iface.name}</span>
							<span class={cn(
								"rounded-full px-1.5 py-px text-[9px] font-bold uppercase tracking-wider",
								iface.state === 'up'
									? "bg-emerald-500/10 text-emerald-400"
									: "bg-zinc-500/10 text-zinc-500"
							)}>
								{iface.state}
							</span>
							<span class="text-[9px] uppercase tracking-wider text-zinc-600">{iface.type}</span>
						</div>
						<p class="mt-0.5 truncate font-mono text-[11px] text-zinc-500">
							{iface.ip || 'No IP'} · {iface.mac || 'No MAC'}
						</p>
					</div>
				</div>
			{:else}
				<div class="flex flex-col items-center justify-center gap-2 px-5 py-10 text-zinc-500">
					<Network class="h-6 w-6 opacity-40" strokeWidth={1.5} />
					<p class="text-xs font-medium">No network interfaces detected</p>
				</div>
			{/each}
		</div>
	</section>
</div>
