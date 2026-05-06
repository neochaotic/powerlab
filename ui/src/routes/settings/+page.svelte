<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { useSettingsStore } from '$lib/stores/settings.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import {
		SlidersHorizontal, Network, Boxes, Info, Globe, Clock, Hash,
		Power, RefreshCw, Copy, Check, ExternalLink, ShieldCheck, KeyRound,
		Code2, Scale, Heart, Sparkles, Container, Zap, Wifi, AlertTriangle
	} from 'lucide-svelte';
	import { Button } from '$lib/components/ui/button';
	import { cn } from '$lib/utils';
	import { fade } from 'svelte/transition';
	import { getAppManagementConfig, type AppManagementConfig } from '$lib/api/apps';
	import { getGatewayPort, setGatewayPort } from '$lib/api/gateway';
	import { toast } from '$lib/stores/toast.svelte';

	const store = useSettingsStore();

	type Section = 'general' | 'network' | 'apps' | 'security' | 'about';
	let activeSection = $state<Section>('general');
	let copiedKey = $state<string | null>(null);

	onMount(() => {
		store.fetchUtilization();
		store.fetchHardwareInfo();
		store.fetchTimezone();
		store.fetchNetworkInterfaces();
		fetchAppConfig();
		fetchCurrentPort();
	});

	let appConfig = $state<AppManagementConfig | null>(null);

	async function fetchAppConfig() {
		try {
			appConfig = await getAppManagementConfig();
		} catch (e) {
			console.error("Failed to fetch app config:", e);
		}
	}

	// ── Gateway port editor (issue #18) ─────────────────────────────────
	// Changing the port terminates the very HTTP connection serving the
	// UI, so the flow is: confirm modal → PUT /v1/gateway/port → wait
	// 3 s with countdown → window.location.href = host:newport.
	let currentPort = $state<string>('');
	let portInput = $state<number>(0);
	let confirmingPortChange = $state(false);
	let countdownSeconds = $state(0);
	let countdownTimer: ReturnType<typeof setInterval> | null = null;

	async function fetchCurrentPort() {
		try {
			const port = await getGatewayPort();
			currentPort = port;
			portInput = parseInt(port, 10) || 8765;
		} catch (e) {
			console.error('Failed to fetch gateway port:', e);
		}
	}

	function requestPortChange() {
		if (!Number.isInteger(portInput) || portInput < 1 || portInput > 65535) {
			toast.error('Port must be an integer between 1 and 65535');
			return;
		}
		if (String(portInput) === currentPort) {
			toast.info('That is already the current port');
			return;
		}
		confirmingPortChange = true;
	}

	async function executePortChange() {
		confirmingPortChange = false;
		try {
			await setGatewayPort(portInput);
		} catch (e) {
			toast.error(`Failed to change port: ${(e as Error).message}`);
			return;
		}
		// Successful response; the gateway is now listening on the new
		// port. Count down then redirect — keep the user in the loop
		// rather than throwing a blank screen at them.
		countdownSeconds = 3;
		countdownTimer = setInterval(() => {
			countdownSeconds--;
			if (countdownSeconds <= 0) {
				if (countdownTimer) clearInterval(countdownTimer);
				const url = new URL(window.location.href);
				url.port = String(portInput);
				window.location.href = url.toString();
			}
		}, 1000);
	}

	function cancelPortChange() {
		confirmingPortChange = false;
		// reset the input back to current so an accidental save doesn't
		// retain a stale value
		portInput = parseInt(currentPort, 10) || 8765;
	}

	// Common timezones — small curated list. Backend accepts any IANA name.
	const timezones = [
		'UTC',
		'America/Sao_Paulo',
		'America/New_York',
		'America/Los_Angeles',
		'America/Chicago',
		'Europe/London',
		'Europe/Berlin',
		'Europe/Paris',
		'Asia/Tokyo',
		'Asia/Shanghai',
		'Asia/Kolkata',
		'Australia/Sydney'
	];

	const sections: Array<{ id: Section; label: string; icon: typeof Globe; desc: string }> = [
		{ id: 'general',  label: 'General',  icon: SlidersHorizontal, desc: 'Hostname, timezone, language' },
		{ id: 'network',  label: 'Network',  icon: Network,           desc: 'mDNS, interfaces, DNS' },
		{ id: 'apps',     label: 'Apps',     icon: Boxes,             desc: 'Storage path, app sources' },
		{ id: 'security', label: 'Security', icon: ShieldCheck,       desc: 'Password, sessions' },
		{ id: 'about',    label: 'About',    icon: Info,              desc: 'Version, license, links' }
	];

	async function copy(text: string, key: string) {
		try {
			await navigator.clipboard.writeText(text);
			copiedKey = key;
			setTimeout(() => { if (copiedKey === key) copiedKey = null; }, 1500);
		} catch { /* clipboard unavailable on insecure contexts */ }
	}

	const mdnsHostname = 'powerlab.local';
	const reachableUrl = `http://${mdnsHostname}`;

	// Storage path is configured at backend startup.
	const storagePath = $derived(appConfig?.storage_path || '/DATA');
</script>

<svelte:head>
	<title>Settings — PowerLab</title>
</svelte:head>

<div class="flex h-full text-zinc-50 font-sans antialiased overflow-hidden">
	<!-- Sidebar -->
	<aside class="w-64 shrink-0 border-r border-white/5 bg-zinc-900/30 backdrop-blur-xl flex flex-col">
		<div class="px-6 pt-8 pb-4">
			<h2 class="text-xl font-bold tracking-tight text-white">Settings</h2>
		</div>

		<nav class="flex-1 px-3 space-y-0.5">
			{#each sections as section}
				{@const Icon = section.icon}
				<button
					onclick={() => activeSection = section.id}
					class={cn(
						"w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors",
						activeSection === section.id
							? "bg-white/10 text-white"
							: "text-zinc-400 hover:text-white hover:bg-white/5"
					)}
				>
					<Icon class={cn("h-4 w-4 shrink-0", activeSection === section.id ? "text-white" : "text-zinc-500")} />
					<span class="font-medium">{section.label}</span>
				</button>
			{/each}
		</nav>

		<div class="p-4 border-t border-white/5">
			<button
				onclick={() => { auth.logout(); goto('/'); }}
				class="w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium text-red-400/80 hover:bg-red-500/10 hover:text-red-400 transition-colors"
			>
				<Power class="h-4 w-4" />
				Sign out
			</button>
		</div>
	</aside>

	<!-- Content -->
	<main class="flex-1 overflow-y-auto">
		<div class="max-w-2xl mx-auto px-12 py-10">

			{#if activeSection === 'general'}
				<div in:fade={{ duration: 150 }}>
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
									<p class="mt-0.5 text-xs text-zinc-500 truncate">{store.utilization?.os?.hostname || 'Unknown'}</p>
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
										bind:value={portInput}
										class="w-24 rounded-lg border border-white/8 bg-white/[0.03] px-3 py-1.5 text-xs text-white outline-none focus:border-emerald-500/40"
									/>
									<button
										class="rounded-lg bg-emerald-500/15 border border-emerald-500/30 px-3 py-1.5 text-xs font-medium text-emerald-300 transition-colors hover:bg-emerald-500/25 disabled:opacity-50 disabled:cursor-not-allowed"
										onclick={requestPortChange}
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

					<!-- Timezone -->
					<section class="mb-8">
						<h3 class="mb-3 text-[10px] font-bold uppercase tracking-[0.2em] text-zinc-500">Locale</h3>
						<div class="rounded-2xl border border-white/5 bg-white/[0.02]">
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
									value={store.timezone}
									onchange={(e) => store.setTimezone(e.currentTarget.value)}
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

			{:else if activeSection === 'network'}
				<div in:fade={{ duration: 150 }}>
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
									onclick={() => copy(reachableUrl, 'mdns')}
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
							{#each store.networkInterfaces as iface, i}
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

			{:else if activeSection === 'apps'}
				<div in:fade={{ duration: 150 }}>
					<header class="mb-8">
						<h1 class="text-2xl font-bold tracking-tight text-white">Apps</h1>
						<p class="mt-1 text-sm text-zinc-500">Where app data lives and where store apps come from.</p>
					</header>

					<!-- Storage path -->
					<section class="mb-8">
						<h3 class="mb-3 text-[10px] font-bold uppercase tracking-[0.2em] text-zinc-500">Storage</h3>
						<div class="rounded-2xl border border-white/5 bg-white/[0.02] p-5">
							<p class="text-sm font-medium text-white">App data directory</p>
							<p class="mt-0.5 text-xs text-zinc-500">Bind-mount root for installed apps. Volume sources prefixed with <code class="rounded bg-white/5 px-1 py-0.5 font-mono text-[10px] text-zinc-300">/DATA</code> are remapped here.</p>
							<div class="mt-3 flex items-center gap-2 rounded-lg bg-black/30 px-3 py-2 font-mono text-[12px] text-zinc-300">
								<span class="flex-1 truncate">{storagePath}</span>
								<button
									class="flex h-6 w-6 items-center justify-center rounded-md text-zinc-500 transition-colors hover:bg-white/5 hover:text-white"
									onclick={() => copy(storagePath, 'storage')}
									aria-label="Copy path"
								>
									{#if copiedKey === 'storage'}
										<Check class="h-3.5 w-3.5 text-emerald-400" />
									{:else}
										<Copy class="h-3.5 w-3.5" />
									{/if}
								</button>
							</div>
							<p class="mt-3 text-[11px] text-zinc-600">Configured at startup via <span class="text-zinc-400">app-management.conf</span>. UI editing planned.</p>
						</div>
					</section>

					<!-- App sources -->
					<section>
						<h3 class="mb-3 text-[10px] font-bold uppercase tracking-[0.2em] text-zinc-500">App store sources</h3>
						<div class="overflow-hidden rounded-2xl border border-white/5 bg-white/[0.02] divide-y divide-white/5">
							<div class="px-5 py-4">
								<p class="text-sm font-medium text-white">Local store</p>
								<p class="mt-0.5 truncate font-mono text-[11px] text-zinc-500">/Users/.../powerlab/store</p>
							</div>
							<div class="px-5 py-4">
								<p class="text-sm font-medium text-white">CasaOS catalog</p>
								<p class="mt-0.5 truncate font-mono text-[11px] text-zinc-500">cdn.jsdelivr.net/.../CasaOS-AppStore@gh-pages</p>
							</div>
							<div class="px-5 py-4">
								<p class="text-sm font-medium text-white">Big-Bear catalog</p>
								<p class="mt-0.5 truncate font-mono text-[11px] text-zinc-500">github.com/bigbeartechworld/big-bear-casaos</p>
							</div>
						</div>
						<p class="mt-2 text-[11px] text-zinc-600">Edit sources via <span class="text-zinc-400">app-management.conf</span>. UI editing planned.</p>
					</section>
				</div>

			{:else if activeSection === 'security'}
				<div in:fade={{ duration: 150 }}>
					<header class="mb-8">
						<h1 class="text-2xl font-bold tracking-tight text-white">Security</h1>
						<p class="mt-1 text-sm text-zinc-500">Authentication and session management.</p>
					</header>

					<section class="mb-8">
						<h3 class="mb-3 text-[10px] font-bold uppercase tracking-[0.2em] text-zinc-500">Account</h3>
						<div class="rounded-2xl border border-white/5 bg-white/[0.02] divide-y divide-white/5">
							<button class="flex w-full items-center justify-between gap-4 px-5 py-4 text-left transition-colors hover:bg-white/[0.02]">
								<div class="flex items-center gap-3">
									<KeyRound class="h-4 w-4 text-zinc-500" />
									<div>
										<p class="text-sm font-medium text-white">Change password</p>
										<p class="mt-0.5 text-xs text-zinc-500">Update your PowerLab login password</p>
									</div>
								</div>
								<span class="text-zinc-500">›</span>
							</button>
							<div class="flex items-center justify-between gap-4 px-5 py-4">
								<div class="flex items-center gap-3">
									<Hash class="h-4 w-4 text-zinc-500" />
									<div>
										<p class="text-sm font-medium text-white">Session timeout</p>
										<p class="mt-0.5 text-xs text-zinc-500">How long until automatic sign-out</p>
									</div>
								</div>
								<select class="rounded-lg border border-white/8 bg-white/[0.03] px-3 py-1.5 text-xs text-white outline-none focus:border-emerald-500/40">
									<option>30 minutes</option>
									<option>2 hours</option>
									<option selected>24 hours</option>
									<option>Never</option>
								</select>
							</div>
						</div>
					</section>

					<p class="text-[11px] text-zinc-600">PowerLab uses a bcrypt-hashed local user. OS-level authentication (PAM/dscl) is on the roadmap.</p>
				</div>

			{:else if activeSection === 'about'}
				<div in:fade={{ duration: 150 }} class="space-y-8">
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
								<span class="rounded-lg border border-white/[0.08] bg-white/[0.03] px-2.5 py-1 font-mono text-[11px] text-zinc-300">v0.1.0-dev</span>
								<span class="rounded-lg border border-white/[0.08] bg-white/[0.03] px-2.5 py-1 text-[11px] text-zinc-400">AGPL-3.0</span>
							</div>
						</div>
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
								href="https://www.casaos.io"
								target="_blank"
								rel="noopener"
								class="group flex items-center justify-between gap-3 rounded-xl border border-white/[0.06] bg-white/[0.02] px-4 py-3 transition-all hover:border-white/10 hover:bg-white/[0.04]"
							>
								<div class="flex items-center gap-3">
									<div class="flex h-9 w-9 items-center justify-center rounded-lg bg-white/[0.04] text-zinc-300 transition-colors group-hover:text-white">
										<Boxes class="h-4 w-4" />
									</div>
									<div>
										<p class="text-sm font-medium text-white">Powered by CasaOS</p>
										<p class="text-[11px] text-zinc-500">Open-source Docker engine</p>
									</div>
								</div>
								<ExternalLink class="h-3.5 w-3.5 text-zinc-600 transition-colors group-hover:text-zinc-300" />
							</a>
						</div>
					</div>

					<!-- Footer -->
					<div class="flex items-center justify-between gap-4 border-t border-white/[0.04] pt-6 text-[11px] text-zinc-600">
						<div class="flex items-center gap-1.5">
							Crafted with <Heart class="h-3 w-3 text-rose-500/80" fill="currentColor" /> by <span class="text-zinc-400">neochaotic</span>
						</div>
						<span>© {new Date().getFullYear()} PowerLab</span>
					</div>
				</div>
			{/if}

		</div>
	</main>
</div>

<!-- Port-change confirmation + countdown modal -->
{#if confirmingPortChange || countdownSeconds > 0}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
		onclick={(e) => { if (countdownSeconds === 0 && e.target === e.currentTarget) cancelPortChange(); }}
	>
		<div
			class="w-[28rem] rounded-2xl border border-white/[0.08] bg-zinc-950/95 p-6 text-left shadow-[0_32px_64px_-12px_rgba(0,0,0,0.7)] backdrop-blur-xl"
			role="dialog"
			aria-label="Confirm port change"
		>
			{#if countdownSeconds === 0}
				<div class="mb-4 flex items-center gap-3">
					<div class="flex h-10 w-10 items-center justify-center rounded-xl bg-amber-500/[0.12]">
						<AlertTriangle class="h-5 w-5 text-amber-400" strokeWidth={1.75} />
					</div>
					<h3 class="text-base font-semibold text-white">Change listen port?</h3>
				</div>
				<p class="text-sm leading-relaxed text-zinc-400">
					The gateway will move from
					<span class="font-mono text-emerald-400">{currentPort}</span>
					to
					<span class="font-mono text-emerald-400">{portInput}</span>.
					This session will disconnect for about 3 seconds while it re-binds.
					We'll redirect you to <span class="font-mono text-zinc-300">{window.location.hostname}:{portInput}</span> automatically.
				</p>
				<p class="mt-3 text-[11px] text-zinc-600">
					If the new port is unreachable from your network, you'll need shell access to revert
					(<span class="font-mono">sudo sed -i 's/^Port = .*/Port = {currentPort}/' /etc/powerlab/gateway.ini && sudo systemctl restart powerlab-gateway</span>).
				</p>
				<div class="mt-5 flex justify-end gap-2">
					<button
						class="rounded-lg px-3 py-1.5 text-sm text-zinc-400 transition-colors hover:bg-white/[0.04] hover:text-white"
						onclick={cancelPortChange}
					>Cancel</button>
					<button
						class="rounded-lg bg-amber-500 px-3 py-1.5 text-sm font-medium text-zinc-950 transition-colors hover:bg-amber-400"
						onclick={executePortChange}
					>Yes, change port</button>
				</div>
			{:else}
				<div class="mb-4 flex items-center gap-3">
					<div class="flex h-10 w-10 items-center justify-center rounded-xl bg-emerald-500/[0.12]">
						<RefreshCw class="h-5 w-5 text-emerald-400 animate-spin" strokeWidth={1.75} />
					</div>
					<h3 class="text-base font-semibold text-white">Redirecting…</h3>
				</div>
				<p class="text-sm leading-relaxed text-zinc-400">
					Port changed to
					<span class="font-mono text-emerald-400">{portInput}</span>.
					Reconnecting in
					<span class="font-bold text-white">{countdownSeconds}</span>…
				</p>
			{/if}
		</div>
	</div>
{/if}
