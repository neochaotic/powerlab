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
	import { updaterStore } from '$lib/stores/updater.svelte';
	import { getCurrentOS, type OS } from '$lib/utils/os';
	import { probePortReachable } from '$lib/utils/probe';
	import { setLocale, getLocale, availableLocales } from '$lib/i18n/index.svelte';
	import { Download, Languages } from 'lucide-svelte';

	const store = useSettingsStore();

	type Section = 'general' | 'network' | 'apps' | 'security' | 'about';
	let activeSection = $state<Section>('general');
	let copiedKey = $state<string | null>(null);

	// Security HTTPS Onboarding state
	let activeSecurityTab = $state<OS>('unknown');
	let isTestingConnection = $state(false);

	// Deep-link support: a URL hash like /settings#security or
	// /settings#network jumps straight to that tab. Used by HttpBanner
	// to bring the user here from anywhere in the app.
	const VALID_SECTIONS: Section[] = ['general', 'network', 'apps', 'security', 'about'];
	function applyHash() {
		const h = window.location.hash.replace(/^#/, '') as Section;
		if (VALID_SECTIONS.includes(h)) activeSection = h;
	}

	onMount(() => {
		applyHash();
		window.addEventListener('hashchange', applyHash);

		activeSecurityTab = getCurrentOS();
		if (activeSecurityTab === 'unknown' || activeSecurityTab === 'linux') {
			activeSecurityTab = 'windows'; // Default for Linux/Unknown to show manual instructions
		}
		return () => window.removeEventListener('hashchange', applyHash);
	});

	// 4-guard trust dance: never redirect to a URL that won't render
	// the SPA. The "Connection failed / Not Found" white screen we
	// saw in dev (gateway:8443 only serves APIs) must not be possible
	// in prod either if config drifts.
	//
	//   Guard 1 — TLS reachable: cross-origin no-cors fetch on the CA
	//     PEM. If the cert chain is broken, the browser rejects.
	//   Guard 2 — SPA-served on HTTPS: cors GET / on the HTTPS port,
	//     content-type must be text/html. Catches the case where the
	//     gateway is alive but only routing APIs (dev mode, mis-config,
	//     reverse proxy override).
	//   Guard 3 — HSTS arming response acked: POST /trust-confirmed
	//     must return 2xx. If the server refuses (HSTS gate file write
	//     failed, request not from non-localhost over HTTPS, etc) we
	//     don't lie to the user with a "Trust established" toast.
	//   Guard 4 — Only redirect when all three guards pass. Otherwise
	//     show the success toast WITHOUT redirecting, so the user
	//     stays on a page that exists.
	async function testHttpsConnection() {
		isTestingConnection = true;
		const httpsBase = `https://${window.location.hostname}:8443`;

		try {
			// Guard 1: TLS handshake must complete. no-cors lets us
			// skip the CORS preflight; we don't need to read the body.
			await fetch(`${httpsBase}/v1/sys/ca-certificate.crt`, {
				mode: 'no-cors',
				signal: AbortSignal.timeout(5000)
			});

			// Guard 2: confirm the HTTPS endpoint is actually serving
			// the SPA (HTML) — not just APIs (JSON 404 / 405). This is
			// what makes the "Not Found" screen impossible to reach.
			let canRedirect = false;
			try {
				const probe = await fetch(`${httpsBase}/`, {
					method: 'GET',
					mode: 'cors',
					signal: AbortSignal.timeout(3000)
				});
				const ct = probe.headers.get('content-type') || '';
				canRedirect = probe.ok && ct.includes('text/html');
			} catch {
				canRedirect = false;
			}

			// Guard 3: HSTS arm. The endpoint REJECTS localhost requests
			// by design (ADR 0006) — the whole point is to prove the
			// trust dance works from a real LAN client. So if the user
			// is testing from localhost we don't even attempt the arm;
			// we just confirm the cert chain works. Production users
			// reach the panel via LAN IP / mDNS, which the handler
			// accepts.
			const isLocalhost = ['localhost', '127.0.0.1', '::1'].includes(window.location.hostname);
			if (!isLocalhost) {
				const armResp = await fetch(`${httpsBase}/v1/sys/trust-confirmed`, {
					method: 'POST',
					mode: 'cors',
					signal: AbortSignal.timeout(5000)
				});
				if (!armResp.ok) {
					const detail = await armResp.text().catch(() => '');
					throw new Error(`HSTS arming refused (${armResp.status}): ${detail.slice(0, 120)}`);
				}
			}

			// Guard 4: only redirect when the destination will render.
			if (!canRedirect) {
				const note = isLocalhost
					? 'Trust verified. (HSTS arming is skipped on localhost — visit via your LAN IP to complete that step.)'
					: 'Trust established. The certificate is valid; visit the secure URL when ready.';
				toast.success(note);
				return;
			}

			toast.success('Trust established! Redirecting to secure connection…');
			setTimeout(() => {
				const secureUrl = new URL(window.location.href);
				secureUrl.protocol = 'https:';
				secureUrl.port = '8443';
				window.location.href = secureUrl.toString();
			}, 1500);

		} catch (e) {
			console.error('HTTPS test failed:', e);
			toast.error(`Connection test failed: ${(e as Error).message || 'unknown error'}. Confirm the certificate is installed and trusted.`);
		} finally {
			isTestingConnection = false;
		}
	}

	onMount(() => {
		// Initialize security tab from OS
		activeSecurityTab = getCurrentOS();
		if (activeSecurityTab === 'unknown' || activeSecurityTab === 'linux') {
			activeSecurityTab = 'windows'; 
		}

		store.fetchUtilization();
		store.fetchHardwareInfo();
		store.fetchTimezone();
		store.fetchNetworkInterfaces();
		fetchAppConfig();
		fetchCurrentPort();
		// Updater polls once on mount and then hourly. Settings is the
		// most frequent landing page for "is there a new version?" so
		// it's the right place to start the cycle. Sidebar pill (later)
		// can read from the same store without re-polling.
		updaterStore.startPolling();
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
		// Successful response; the gateway will rebind on the new
		// port within a ~1-2s grace window. Count down, probe the
		// new port, ONLY redirect if it's actually answering.
		// Otherwise we'd strand the user on a connection-refused page.
		countdownSeconds = 3;
		countdownTimer = setInterval(async () => {
			countdownSeconds--;
			if (countdownSeconds > 0) return;
			if (countdownTimer) clearInterval(countdownTimer);

			const newUrl = new URL(window.location.href);
			newUrl.port = String(portInput);

			// Retry the probe a few times — the rebind grace window
			// can stretch on slow hosts and the cleanup of the old
			// listener is asynchronous.
			let alive = false;
			for (let i = 0; i < 4; i++) {
				if (await probePortReachable(newUrl)) {
					alive = true;
					break;
				}
				await new Promise(r => setTimeout(r, 750));
			}

			if (!alive) {
				toast.error(`Port change acknowledged but ${portInput} is not responding. The change may have failed; refresh manually if you can reach it, or revert via the API.`);
				return;
			}

			window.location.href = newUrl.toString();
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

					<!-- Locale: language + timezone -->
					<section class="mb-8">
						<h3 class="mb-3 text-[10px] font-bold uppercase tracking-[0.2em] text-zinc-500">Locale</h3>
						<div class="rounded-2xl border border-white/5 bg-white/[0.02] divide-y divide-white/[0.04]">
							<!-- Language. The first time the user lands here, we
								 honor their browser locale; once they pick from
								 this dropdown the choice persists to localStorage
								 (powerlab_locale) and survives a refresh. The
								 picker also forces a re-render so the surrounding
								 text updates immediately, no reload required. -->
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
						<p class="mt-1 text-sm text-zinc-500">HTTPS infrastructure and session management.</p>
					</header>

					<!-- HTTPS Onboarding (Issue #43) -->
					<section class="mb-10">
						<h3 class="mb-4 text-[10px] font-bold uppercase tracking-[0.2em] text-zinc-500">Local HTTPS Establishment</h3>
						<div class="overflow-hidden rounded-2xl border border-white/5 bg-white/[0.02]">
							<!-- Tabs Header -->
							<div class="flex border-b border-white/5 bg-white/[0.02]">
								{#each ['ios', 'macos', 'android', 'windows'] as tab}
									<button
										class={cn(
											"flex-1 px-4 py-3 text-[11px] font-bold uppercase tracking-wider transition-colors",
											activeSecurityTab === tab ? "bg-white/5 text-white shadow-[inset_0_-2px_0_white]" : "text-zinc-500 hover:text-zinc-300"
										)}
										onclick={() => activeSecurityTab = tab as any}
									>
										{tab}
									</button>
								{/each}
							</div>

							<!-- Tab Content -->
							<div class="p-6">
								<div class="grid grid-cols-1 lg:grid-cols-2 gap-8">
									<div class="space-y-6">
										{#if activeSecurityTab === 'ios'}
											<div class="space-y-4" in:fade>
												<h4 class="text-lg font-semibold text-white">iOS Installation</h4>
												<ol class="space-y-3 list-decimal list-inside text-sm text-zinc-400">
													<li>Download the <a href="/v1/sys/ca-certificate.mobileconfig" class="text-emerald-400 hover:underline">Security Profile</a>.</li>
													<li>Go to <strong>Settings → Profile Downloaded</strong> and click <strong>Install</strong>.</li>
													<li>Go to <strong>Settings → General → About → Certificate Trust Settings</strong>.</li>
													<li>Enable full trust for <strong>PowerLab Root CA</strong>.</li>
												</ol>
												<Button class="w-full bg-white text-zinc-950 font-bold" onclick={() => window.location.href = '/v1/sys/ca-certificate.mobileconfig'}>
													<Download class="h-4 w-4 mr-2" />
													Download Profile
												</Button>
											</div>
										{:else if activeSecurityTab === 'macos'}
											<div class="space-y-4" in:fade>
												<h4 class="text-lg font-semibold text-white">macOS Installation</h4>
												<ol class="space-y-3 list-decimal list-inside text-sm text-zinc-400">
													<li>Download the <a href="/v1/sys/ca-certificate.mobileconfig" class="text-emerald-400 hover:underline">Security Profile</a>.</li>
													<li>Open the profile and click <strong>Install</strong> in System Settings.</li>
													<li>Alternatively, use the <a href="/v1/sys/ca-certificate.crt" class="text-emerald-400 hover:underline">CRT file</a> and set trust in Keychain Access.</li>
												</ol>
												<div class="flex gap-2">
													<Button variant="secondary" class="flex-1 font-bold" onclick={() => window.location.href = '/v1/sys/ca-certificate.mobileconfig'}>.mobileconfig</Button>
													<Button variant="secondary" class="flex-1 font-bold" onclick={() => window.location.href = '/v1/sys/ca-certificate.crt'}>.crt</Button>
												</div>
											</div>
										{:else if activeSecurityTab === 'android'}
											<div class="space-y-4" in:fade>
												<h4 class="text-lg font-semibold text-white">Android Installation</h4>
												<ol class="space-y-3 list-decimal list-inside text-sm text-zinc-400">
													<li>Download the <a href="/v1/sys/ca-certificate.crt" class="text-emerald-400 hover:underline">CA Certificate</a>.</li>
													<li>Settings → Security → Encryption → Install from storage.</li>
													<li>Select <strong>CA certificate</strong> and pick the file.</li>
												</ol>
												<Button class="w-full bg-white text-zinc-950 font-bold" onclick={() => window.location.href = '/v1/sys/ca-certificate.crt'}>
													<Download class="h-4 w-4 mr-2" />
													Download CRT
												</Button>
											</div>
										{:else if activeSecurityTab === 'windows'}
											<div class="space-y-4" in:fade>
												<h4 class="text-lg font-semibold text-white">Windows / Linux</h4>
												<ol class="space-y-3 list-decimal list-inside text-sm text-zinc-400">
													<li>Download the <a href="/v1/sys/ca-certificate.crt" class="text-emerald-400 hover:underline">CA Certificate</a>.</li>
													<li>Right-click → Install → Local Machine.</li>
													<li>Place in <strong>Trusted Root Certification Authorities</strong>.</li>
													<li>On Linux: Copy to <code>/usr/local/share/ca-certificates/</code> and run <code>update-ca-certificates</code>.</li>
												</ol>
												<Button class="w-full bg-white text-zinc-950 font-bold" onclick={() => window.location.href = '/v1/sys/ca-certificate.crt'}>
													<Download class="h-4 w-4 mr-2" />
													Download CRT
												</Button>
											</div>
										{/if}

										<div class="pt-6 border-t border-white/5">
											<div class="flex items-center justify-between gap-4 p-4 rounded-xl bg-emerald-500/5 border border-emerald-500/10">
												<div>
													<p class="text-sm font-bold text-white">Verification</p>
													<p class="text-xs text-zinc-500">Test if your device trusts PowerLab.</p>
												</div>
												<Button 
													size="sm" 
													class={cn("font-bold transition-all", isTestingConnection ? "bg-zinc-800 text-zinc-500" : "bg-emerald-500 text-zinc-950 hover:bg-emerald-400")}
													onclick={testHttpsConnection}
													disabled={isTestingConnection}
												>
													{#if isTestingConnection}
														<RefreshCw class="h-3.5 w-3.5 mr-2 animate-spin" />
														Testing…
													{:else}
														Test Connection
													{/if}
												</Button>
											</div>
										</div>
									</div>

									<!-- Visual Guide -->
									<div class="relative aspect-[4/3] overflow-hidden rounded-xl border border-white/5 bg-black/40">
										<img 
											src={`/docs/security/${activeSecurityTab}.png`} 
											alt={`${activeSecurityTab} installation guide`}
											class="h-full w-full object-cover opacity-80"
										/>
										<div class="absolute inset-0 bg-gradient-to-t from-zinc-950/60 to-transparent"></div>
										<p class="absolute bottom-4 left-4 text-[10px] font-bold uppercase tracking-widest text-white/40">Reference Mockup</p>
									</div>
								</div>
							</div>
						</div>
					</section>

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
									{#if updaterStore.check.release_summary}
										<span class="block mt-1 text-[12px] text-zinc-500">{updaterStore.check.release_summary}</span>
									{/if}
								</p>
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
										onclick={() => updaterStore.install()}
										disabled={updaterStore.installing}
									>
										{updaterStore.installing ? 'Upgrading…' : `Upgrade to v${updaterStore.check.available}`}
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
