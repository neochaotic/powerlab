<script lang="ts">
	import { Clock, Wifi, Languages, RefreshCw, Power } from 'lucide-svelte';
	import { setLocale, getLocale, availableLocales } from '$lib/i18n/index.svelte';

	interface Props {
		// Reactive store reading slot — passed as a getter so the parent
		// keeps reactivity wiring (utilization.os.hostname, timezone,
		// setTimezone). Component doesn't own store lifecycle.
		osHostname: string;
		timezone: string;
		onTimezoneChange: (value: string) => void;
		reachableUrl: string;
		currentPort: string;
		portInput: number;
		onPortInputChange: (value: number) => void;
		onRequestPortChange: () => void;
		timezones: readonly string[];
	}

	let {
		osHostname,
		timezone,
		onTimezoneChange,
		reachableUrl,
		currentPort,
		portInput,
		onPortInputChange,
		onRequestPortChange,
		timezones
	}: Props = $props();
</script>

<div>
	<header class="mb-8">
		<h1 class="text-2xl font-bold tracking-tight text-white">General</h1>
		<p class="mt-1 text-sm text-zinc-500">Basic identity and locale.</p>
	</header>

	<!-- Hostname -->
	<section class="mb-8">
		<h3 class="mb-3 text-[10px] font-bold uppercase tracking-[0.2em] text-zinc-500">Identity</h3>
		<div class="rounded-2xl border border-white/5 bg-white/[0.02] divide-y divide-white/5">
			<div class="flex items-center justify-between gap-4 px-5 py-4">
				<div class="min-w-0">
					<p class="text-sm font-medium text-white">PowerLab hostname</p>
					<p class="mt-0.5 text-xs text-zinc-500">Used as the mDNS name. Reachable at <span class="text-emerald-400">{reachableUrl}</span></p>
				</div>
				<div class="flex items-center gap-2">
					<input
						type="text"
						value="powerlab"
						disabled
						class="w-32 rounded-lg border border-white/5 bg-white/[0.03] px-2.5 py-1.5 text-xs text-zinc-400 outline-none disabled:cursor-not-allowed"
					/>
					<span class="text-xs text-zinc-600">.local</span>
				</div>
			</div>
			<div class="flex items-center justify-between gap-4 px-5 py-4">
				<div class="min-w-0">
					<p class="text-sm font-medium text-white">OS hostname</p>
					<p class="mt-0.5 text-xs text-zinc-500 truncate">{osHostname || 'Unknown'}</p>
				</div>
				<span class="text-[10px] font-medium uppercase tracking-wider text-zinc-600">read-only</span>
			</div>
		</div>
		<p class="mt-2 text-[11px] text-zinc-600">Hostname configuration via UI is planned. For now, edit at the OS level.</p>
	</section>

	<!-- Listen port (issue #18) -->
	<section class="mb-8">
		<h3 class="mb-3 text-[10px] font-bold uppercase tracking-[0.2em] text-zinc-500">Network</h3>
		<div class="rounded-2xl border border-white/5 bg-white/[0.02]">
			<div class="flex items-center justify-between gap-4 px-5 py-4">
				<div class="flex items-center gap-3">
					<Wifi class="h-4 w-4 text-zinc-500" />
					<div>
						<p class="text-sm font-medium text-white">Listen port</p>
						<p class="mt-0.5 text-xs text-zinc-500">
							The HTTP port the gateway binds. Currently <span class="text-emerald-400">{currentPort || '?'}</span>.
						</p>
					</div>
				</div>
				<div class="flex items-center gap-2">
					<input
						type="number"
						min="1"
						max="65535"
						value={portInput}
						oninput={(e) => onPortInputChange(parseInt(e.currentTarget.value, 10) || 0)}
						class="w-24 rounded-lg border border-white/8 bg-white/[0.03] px-3 py-1.5 text-xs text-white outline-none focus:border-emerald-500/40"
					/>
					<button
						class="rounded-lg bg-emerald-500/15 border border-emerald-500/30 px-3 py-1.5 text-xs font-medium text-emerald-300 transition-colors hover:bg-emerald-500/25 disabled:opacity-50 disabled:cursor-not-allowed"
						onclick={onRequestPortChange}
						disabled={String(portInput) === currentPort}
					>
						Change…
					</button>
				</div>
			</div>
		</div>
		<p class="mt-2 text-[11px] text-zinc-600">
			Changing the port disconnects this session for ~3 seconds while the gateway re-binds. We'll redirect you automatically.
		</p>
	</section>

	<!-- Locale: language + timezone -->
	<section class="mb-8">
		<h3 class="mb-3 text-[10px] font-bold uppercase tracking-[0.2em] text-zinc-500">Locale</h3>
		<div class="rounded-2xl border border-white/5 bg-white/[0.02] divide-y divide-white/[0.04]">
			<div class="flex items-center justify-between gap-4 px-5 py-4">
				<div class="flex items-center gap-3">
					<Languages class="h-4 w-4 text-zinc-500" />
					<div>
						<p class="text-sm font-medium text-white">Language</p>
						<p class="mt-0.5 text-xs text-zinc-500">Display language for the panel UI.</p>
					</div>
				</div>
				<select
					class="rounded-lg border border-white/8 bg-white/[0.03] px-3 py-1.5 text-xs text-white outline-none focus:border-emerald-500/40"
					value={getLocale()}
					onchange={(e) => setLocale(e.currentTarget.value)}
				>
					{#each availableLocales as opt}
						<option value={opt.id}>{opt.label}</option>
					{/each}
				</select>
			</div>
			<div class="flex items-center justify-between gap-4 px-5 py-4">
				<div class="flex items-center gap-3">
					<Clock class="h-4 w-4 text-zinc-500" />
					<div>
						<p class="text-sm font-medium text-white">Timezone</p>
						<p class="mt-0.5 text-xs text-zinc-500">Affects logs and scheduled tasks.</p>
					</div>
				</div>
				<select
					class="rounded-lg border border-white/8 bg-white/[0.03] px-3 py-1.5 text-xs text-white outline-none focus:border-emerald-500/40"
					value={timezone}
					onchange={(e) => onTimezoneChange(e.currentTarget.value)}
				>
					{#each timezones as tz}
						<option value={tz}>{tz}</option>
					{/each}
				</select>
			</div>
		</div>
	</section>

	<!-- Power -->
	<section>
		<h3 class="mb-3 text-[10px] font-bold uppercase tracking-[0.2em] text-zinc-500">Power</h3>
		<div class="grid grid-cols-2 gap-3">
			<button class="flex items-center gap-3 rounded-2xl border border-white/5 bg-white/[0.02] px-5 py-4 text-left transition-colors hover:bg-white/[0.04]">
				<RefreshCw class="h-4 w-4 text-zinc-500" />
				<div>
					<p class="text-sm font-medium text-white">Reboot</p>
					<p class="mt-0.5 text-xs text-zinc-500">Restart the server</p>
				</div>
			</button>
			<button class="flex items-center gap-3 rounded-2xl border border-white/5 bg-white/[0.02] px-5 py-4 text-left transition-colors hover:border-red-500/20 hover:bg-red-500/[0.04]">
				<Power class="h-4 w-4 text-zinc-500" />
				<div>
					<p class="text-sm font-medium text-white">Shut down</p>
					<p class="mt-0.5 text-xs text-zinc-500">Power off the server</p>
				</div>
			</button>
		</div>
	</section>
</div>
