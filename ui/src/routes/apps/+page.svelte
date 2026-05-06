<script lang="ts">
	import { onMount } from 'svelte';
	import Fuse from 'fuse.js';
	import yaml from 'js-yaml';
	import { useAppStore } from '$lib/stores/apps.svelte';
	import type { ComposeAppStoreInfo } from '$lib/api/apps';
	import { getStoreAppYaml, installComposeApp, uninstallComposeApp, getComposeAppDiskUsage, updateComposeApp, checkPorts } from '$lib/api/apps';
	import ContainerLogs from '$lib/components/apps/ContainerLogs.svelte';
	import AppMetrics from '$lib/components/apps/AppMetrics.svelte';
	import Markdown from '$lib/components/ui/Markdown.svelte';
	import { Button } from '$lib/components/ui/button';
	import {
		ArrowLeft, Search, X, Package, Pencil, ArrowUpCircle,
		Play, Square, Activity, ScrollText, Trash2, ChevronRight, RefreshCw, Plus, CheckCircle2, Loader2, AlertCircle, ArrowRight, Boxes
	} from 'lucide-svelte';
	import { cn } from '$lib/utils';
	import { fade, scale } from 'svelte/transition';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { ui } from '$lib/stores/ui.svelte';

	const store = useAppStore();

	$effect(() => {
		if (ui.searchTriggered) {
			currentTab = 'store';
			setTimeout(() => searchEl?.focus(), 50);
		}
	});

	// Default to 'store' — installed apps are already visible on the Launchpad
	let currentTab = $state<'store' | 'installed'>('store');
	let activeLogAppId = $state<string | null>(null);
	let confirmingUninstall = $state<string | null>(null);
	let deleteData = $state(false);
	let forkingAppId = $state<string | null>(null);
	let showLogsModal = $state(false);
	let selectedAppId = $state('');
	let showMetricsModal = $state(false);

	// Install flow state machine
	// 'installing' = HTTP POST in flight
	// 'starting'   = async Docker pull/start in progress, SSE logs streaming
	// 'success'    = app appeared in installed list
	// 'timeout'    = 10 min elapsed, user should check Launchpad
	// 'error'      = backend returned error or container failed to start
	type InstallPhase = 'idle' | 'confirm' | 'installing' | 'starting' | 'success' | 'timeout' | 'error';
	let installPhase = $state<InstallPhase>('idle');
	let detailApp = $state<ComposeAppStoreInfo | null>(null);
	let pendingInstallApp = $state<ComposeAppStoreInfo | null>(null);
	let installingApp = $state<ComposeAppStoreInfo | null>(null);
	let installError = $state<string | null>(null);
	let installedProjectId = $state<string | null>(null);
	let installLogs = $state('');
	let installLogEl = $state<HTMLPreElement | null>(null);
	let activeSSE = $state<EventSource | null>(null);
	let sseTimeoutId: ReturnType<typeof setTimeout> | null = null;
	let installPortNote = $state<string | null>(null); // "Running on port 8081 (was 8080)"
	let compatibilityWarnings = $state<string[]>([]);
	let isCheckingCompatibility = $state(false);
	let hasCriticalWarning = $derived(compatibilityWarnings.some(w => w.toLowerCase().includes('critical') || w.toLowerCase().includes('privileged')));
	let diskUsages = $state<Record<string, number>>({});
	let confirmingUpdate = $state<any | null>(null);

	// Search state — persisted in sessionStorage so navigating to the install
	// flow and back doesn't lose the user's search/filter context.
	let searchQuery = $state('');
	let searchEl = $state<HTMLInputElement | null>(null);
	let activeCategory = $state<string | null>(null);

	const STATE_KEY = 'powerlab_store_state';

	onMount(async () => {
		const tabParam = $page.url.searchParams.get('tab');
		if (tabParam === 'installed') currentTab = 'installed';

		// Restore previous search/filter state from sessionStorage.
		try {
			const saved = sessionStorage.getItem(STATE_KEY);
			if (saved) {
				const s = JSON.parse(saved) as { search?: string; category?: string | null };
				if (s.search) searchQuery = s.search;
				if (s.category !== undefined) activeCategory = s.category;
			}
		} catch { /* ignore parse errors */ }

		store.fetchAppStore();
		store.fetchInstalledApps();
	});

	function formatSize(bytes: number) {
		if (bytes === 0) return '0 B';
		const k = 1024;
		const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
		const i = Math.floor(Math.log(bytes) / Math.log(k));
		return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
	}

	// Persist whenever search or category changes.
	$effect(() => {
		try {
			sessionStorage.setItem(STATE_KEY, JSON.stringify({
				search: searchQuery,
				category: activeCategory
			}));
		} catch { /* ignore quota errors */ }
	});

	$effect(() => {
		if (currentTab === 'installed') store.fetchInstalledApps();
		else store.fetchAppStore();
	});

	// Keyboard: `/` focuses search (when not already typing); ESC clears + blurs.
	// Only active on the Discover tab so we don't hijack other contexts.
	function onGlobalKey(e: KeyboardEvent) {
		if (currentTab !== 'store') return;
		const t = e.target as HTMLElement | null;
		const isTyping = !!t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable);

		if (e.key === '/' && !isTyping) {
			e.preventDefault();
			searchEl?.focus();
			return;
		}
		if (e.key === 'Escape' && t === searchEl) {
			searchQuery = '';
			searchEl?.blur();
		}
	}

	// ── Fuse.js search engine ──────────────────────────────────────────────────
	// Keys weighted: title > developer > description > category.
	// threshold 0.35 = tolerates ~1–2 character typos on short words.
	const fuse = $derived.by(() => {
		if (store.appStoreCatalog.length === 0) return null;
		return new Fuse(store.appStoreCatalog, {
			keys: [
				{ name: 'title.en_us', weight: 0.45 },
				{ name: 'developer', weight: 0.2 },
				{ name: 'description.en_us', weight: 0.25 },
				{ name: 'category', weight: 0.1 }
			],
			threshold: 0.35,
			includeScore: true,
			ignoreLocation: true,
			minMatchCharLength: 2
		});
	});

	// Derived filtered + sorted catalog
	const filteredCatalog = $derived.by(() => {
		let items = store.appStoreCatalog;

		// Category filter
		if (activeCategory) {
			items = items.filter(a => a.category === activeCategory);
		}

		// Fuzzy search
		if (searchQuery.trim().length >= 2 && fuse) {
			return fuse.search(searchQuery.trim()).map(r => r.item);
		}

		// Default sort: alphabetical by title
		return [...items].sort((a, b) => {
			const ta = getTitle(a.title);
			const tb = getTitle(b.title);
			return ta.localeCompare(tb);
		});
	});

	// Unique sorted categories from the catalog
	const categories = $derived.by(() => {
		const seen = new Set<string>();
		store.appStoreCatalog.forEach(a => { if (a.category) seen.add(a.category); });
		return Array.from(seen).sort();
	});

	function getTitle(titleObj: Record<string, string> | undefined) {
		if (!titleObj) return 'Unknown';
		return titleObj['en_us'] || titleObj['en_US'] || Object.values(titleObj)[0] || 'Unknown';
	}

	function extractProjectName(yaml: string): string | null {
		const m = yaml.match(/^name:\s*(.+)$/m);
		return m ? m[1].trim() : null;
	}

	function extractPortMap(yaml: string): string | null {
		// Look for `port_map: "8080"` (or web:/port:) inside the
		// PowerLab/CasaOS extension block (x-powerlab, x-web, or x-casaos —
		// they all share the same property names).
		const m = yaml.match(/^\s+(?:port_map|web|port):\s*"?(\d+)"?$/m);
		return m ? m[1] : null;
	}

	// Translate raw backend / Docker errors into user-facing messages.
	// Falls back to the raw text if nothing matches.
	function humanizeError(raw: string): string {
		if (!raw) return 'Installation failed';
		const r = raw.toLowerCase();
		if (r.includes('mkdir') && r.includes('/dev/')) {
			return 'This app needs Linux-only kernel devices (e.g. /dev/net/tun for VPN apps) that aren\'t available on this system.';
		}
		if (r.includes('no matching manifest') || r.includes('no matching distribution')) {
			return 'This app is not available for your CPU architecture.';
		}
		if (r.includes('pull access denied') || r.includes('repository does not exist')) {
			return 'The Docker image could not be pulled. It may have been removed or is no longer public.';
		}
		if (r.includes('port is already allocated') || r.includes('bind: address already in use')) {
			return 'Another service is using a port this app needs. Try restarting and reinstalling.';
		}
		if (r.includes('extension') && r.includes('not found')) {
			return 'This Compose file is missing the x-powerlab (or x-casaos) manifest required by PowerLab.';
		}
		if (r.includes('compose app') && r.includes('is already being installed')) {
			return 'This app is already installing. Wait for it to finish before trying again.';
		}
		if (r.includes('network error')) {
			return 'Could not reach the backend. Check that the PowerLab service is running.';
		}
		// Truncate very long stack traces
		if (raw.length > 240) return raw.slice(0, 240) + '…';
		return raw;
	}

	// Each conflicting port gets an editable choice. The user can accept the
	// suggestion or type a different number; live validation gates Install.
	type PortChoice = {
		original: number;
		chosen: string;                                  // bound to the input
		status: 'free' | 'inuse' | 'invalid' | 'checking';
	};
	let portChoices = $state<PortChoice[]>([]);
	let portCheckTimer: ReturnType<typeof setTimeout> | null = null;

	// True only when every conflicting port has a free chosen value.
	const portsResolved = $derived(portChoices.every(p => p.status === 'free'));

	async function requestInstall(storeApp: ComposeAppStoreInfo) {
		pendingInstallApp = storeApp;
		installPhase = 'confirm';
		compatibilityWarnings = [];
		portChoices = [];
		isCheckingCompatibility = true;

		try {
			const yamlText = await getStoreAppYaml(storeApp.store_app_id);
			const parsed = yaml.load(yamlText) as any;
			const requestedPorts: number[] = [];

			if (parsed?.services) {
				const services = Object.values(parsed.services) as any[];
				for (const svc of services) {
					if (svc.network_mode === 'host') {
						compatibilityWarnings.push("This app needs network_mode: host which doesn't work on Docker Desktop (macOS/Windows)");
					}
					if (svc.privileged === true) {
						compatibilityWarnings.push("This app requires privileged mode (Critical Risk)");
					}
					if (svc.cap_add && Array.isArray(svc.cap_add)) {
						if (svc.cap_add.some((c: string) => c.includes('ADMIN') || c.includes('NET'))) {
							compatibilityWarnings.push("Needs Linux kernel capabilities (cap_add)");
						}
					}
					if (svc.volumes && Array.isArray(svc.volumes)) {
						if (svc.volumes.some((v: any) => {
							const path = typeof v === 'string' ? v : v.source;
							return path?.startsWith('/dev/') || path?.startsWith('/proc/') || path?.startsWith('/sys/');
						})) {
							compatibilityWarnings.push("Needs Linux kernel devices (/dev, /proc, /sys)");
						}
					}
					// Collect every published host port across all services (deduped).
					if (Array.isArray(svc.ports)) {
						for (const p of svc.ports) {
							const pub = typeof p === 'string' ? p.split(':')[0] : (p?.published ?? '');
							const n = parseInt(String(pub), 10);
							if (Number.isFinite(n) && n > 0 && !requestedPorts.includes(n)) {
								requestedPorts.push(n);
							}
						}
					}
				}
			}

			// Probe ports. For any in use, seed the choice with the suggestion
			// (or original+1) so the user has a sensible default to accept or edit.
			if (requestedPorts.length > 0) {
				try {
					const res = await checkPorts(requestedPorts);
					for (const port of requestedPorts) {
						const key = String(port);
						if (res.data?.[key] === false) {
							const suggested = res.suggestions?.[key] ?? port + 1;
							portChoices.push({
								original: port,
								chosen: String(suggested),
								status: 'free' // assume the suggestion is free; validatePortChoices() re-checks
							});
						}
					}
					if (portChoices.length > 0) await validatePortChoices();
				} catch { /* probe is best-effort */ }
			}
		} catch (e) {
			console.error('Failed to check compatibility:', e);
		} finally {
			isCheckingCompatibility = false;
		}
	}

	// Re-check every chosen port. Catches the case where the suggestion was
	// stale (the suggested port was grabbed by another process between
	// /ports/check and the user submitting).
	async function validatePortChoices() {
		const ports = portChoices
			.map(c => parseInt(c.chosen, 10))
			.filter(n => Number.isFinite(n) && n > 0 && n < 65536);

		// Mark invalid inputs immediately
		for (const c of portChoices) {
			const n = parseInt(c.chosen, 10);
			if (!Number.isFinite(n) || n < 1 || n > 65535) {
				c.status = 'invalid';
			} else {
				c.status = 'checking';
			}
		}

		if (ports.length === 0) return;

		try {
			const res = await checkPorts(ports);
			for (const c of portChoices) {
				if (c.status === 'invalid') continue;
				const ok = res.data?.[c.chosen];
				c.status = ok === true ? 'free' : ok === false ? 'inuse' : 'free';
			}
		} catch {
			// Network failure — assume the choices are fine; backend will resolve.
			for (const c of portChoices) if (c.status === 'checking') c.status = 'free';
		}
	}

	function onPortInput() {
		if (portCheckTimer) clearTimeout(portCheckTimer);
		portCheckTimer = setTimeout(() => void validatePortChoices(), 350);
	}

	// Auto-pick: walk forward until a free port is found for THIS choice.
	async function autoPickPort(choice: PortChoice) {
		choice.status = 'checking';
		const start = parseInt(choice.chosen, 10);
		const base = Number.isFinite(start) && start > 0 ? start : choice.original;
		// Cheap walk: probe in batches of 10 to avoid one round-trip per port.
		for (let batch = 0; batch < 10; batch++) {
			const tries = Array.from({ length: 10 }, (_, i) => base + batch * 10 + i + 1);
			try {
				const res = await checkPorts(tries);
				const free = tries.find(p => res.data?.[String(p)] === true);
				if (free) {
					choice.chosen = String(free);
					choice.status = 'free';
					return;
				}
			} catch {
				return; // network failure — leave as-is
			}
		}
		// Nothing free in 100 ports above — give up; user can type something else.
		choice.status = 'inuse';
	}

	// Rewrite the YAML so backend receives the user's chosen ports. Single regex
	// per port replacement keeps quoting and indentation untouched.
	function rewriteYamlPorts(yamlText: string, remap: Record<number, number>): string {
		let out = yamlText;
		for (const [oldPort, newPort] of Object.entries(remap)) {
			if (Number(oldPort) === newPort) continue;
			// Match `published: "8080"` / `published: 8080` (also handles single-quoted)
			const re = new RegExp(`(published:\\s*["']?)${oldPort}(["']?)`, 'g');
			out = out.replace(re, `$1${newPort}$2`);
			// Also rewrite the PowerLab/CasaOS extension's port_map / web / port
			// (regex matches keys regardless of which alias the YAML uses)
			const xcRe = new RegExp(`((?:port_map|web|port):\\s*["']?)${oldPort}(["']?)`, 'g');
			out = out.replace(xcRe, `$1${newPort}$2`);
		}
		return out;
	}

	// Original port_map captured from the store YAML before install — used to
	// detect auto-remap and tell the user when the actual port differs.
	let originalPortMap: string | null = null;

	async function executeInstall() {
		if (!pendingInstallApp) return;
		const app = pendingInstallApp;
		pendingInstallApp = null;
		installingApp = app;
		installPhase = 'installing';
		installError = null;
		installedProjectId = null;
		installLogs = '';
		installPortNote = null;
		originalPortMap = null;

		try {
			let yamlText = await getStoreAppYaml(app.store_app_id);
			installedProjectId = extractProjectName(yamlText) ?? app.store_app_id.toLowerCase().replace(/[^a-z0-9-]/g, '-');
			originalPortMap = extractPortMap(yamlText);

			// Apply the user's chosen ports (if any) to the YAML before sending.
			// Backend's autoRemap will still run as a safety net; the user's
			// values are just the starting point.
			if (portChoices.length > 0) {
				const remap: Record<number, number> = {};
				for (const c of portChoices) {
					const newPort = parseInt(c.chosen, 10);
					if (Number.isFinite(newPort) && newPort !== c.original) {
						remap[c.original] = newPort;
					}
				}
				if (Object.keys(remap).length > 0) {
					yamlText = rewriteYamlPorts(yamlText, remap);
				}
			}

			await installComposeApp(yamlText);
			// HTTP 200 = install started asynchronously. Stream SSE logs and poll for success.
			installPhase = 'starting';
			streamInstallLogs(installedProjectId);
		} catch (e) {
			installError = humanizeError((e as Error).message ?? 'Installation failed');
			installPhase = 'error';
		}
	}

	function streamInstallLogs(projectId: string) {
		const sse = new EventSource(`/v2/app_management/compose/task/${projectId}/logs`);
		activeSSE = sse;

		sse.onmessage = (e) => {
			installLogs += e.data + '\n';
			requestAnimationFrame(() => {
				if (installLogEl) installLogEl.scrollTop = installLogEl.scrollHeight;
			});
		};

		// Backend sends `event: end` when docker pull+start finished (success or failure)
		sse.addEventListener('end', () => {
			sse.close();
			activeSSE = null;
			clearSSETimeout();
			void checkInstallResult(projectId);
		});

		sse.onerror = () => {
			sse.close();
			activeSSE = null;
			clearSSETimeout();
			// SSE failed — fall back to polling once after a short delay
			setTimeout(() => checkInstallResult(projectId), 3000);
		};

		// Safety timeout: after 10 minutes assume something went wrong
		sseTimeoutId = setTimeout(() => {
			sse.close();
			activeSSE = null;
			installPhase = 'timeout';
		}, 600_000);
	}

	async function handleUpdate() {
		if (!confirmingUpdate) return;
		const app = confirmingUpdate;
		confirmingUpdate = null;
		
		installingApp = app.store_info;
		installPhase = 'installing';
		installError = null;
		installedProjectId = app.id;
		installLogs = '';
		installPortNote = null;
		
		try {
			await updateComposeApp(app.id);
			installPhase = 'starting';
			streamInstallLogs(app.id);
		} catch (e) {
			installError = humanizeError((e as Error).message ?? 'Update failed');
			installPhase = 'error';
		}
	}

	async function checkInstallResult(projectId: string) {
		await store.fetchInstalledApps();
		const found = store.installedApps.find(a => a.id === projectId);
		if (found) {
			// Pin the new app to the first slot of the Launchpad's custom order.
			// Launchpad reads `workload_order` from localStorage on mount.
			try {
				const raw = localStorage.getItem('workload_order');
				const arr: string[] = raw ? JSON.parse(raw) : [];
				const next = [projectId, ...arr.filter(id => id !== projectId)];
				localStorage.setItem('workload_order', JSON.stringify(next));
			} catch { /* ignore quota / parse errors */ }

			// If the backend auto-remapped the published port (because the requested
			// one was in use), show the user what port the app actually opened on.
			const actualPort = found.store_info?.port_map;
			if (actualPort && originalPortMap && actualPort !== originalPortMap) {
				installPortNote = `Running on port ${actualPort} — port ${originalPortMap} was already in use.`;
			} else if (actualPort) {
				installPortNote = `Running on port ${actualPort}.`;
			}

			installPhase = 'success';
		} else {
			// Pull the most useful line from the SSE log to show as the human error.
			const lastErrorLine = installLogs
				.split('\n')
				.reverse()
				.find(l => /error|fail|denied|not found|permitted/i.test(l));
			installError = humanizeError(lastErrorLine ?? 'App did not appear in the installed list. It may still be starting — check your Launchpad.');
			installPhase = 'error';
		}
	}

	function clearSSETimeout() {
		if (sseTimeoutId) { clearTimeout(sseTimeoutId); sseTimeoutId = null; }
	}

	function closeInstallOverlay() {
		activeSSE?.close();
		activeSSE = null;
		clearSSETimeout();
		installPhase = 'idle';
		installingApp = null;
		installedProjectId = null;
		installLogs = '';
	}

	async function handleUninstall(id: string) {
		try {
			await uninstallComposeApp(id, deleteData);
			confirmingUninstall = null;
			deleteData = false;
			await store.fetchInstalledApps();
		} catch (e) {
			installError = (e as Error).message ?? 'Uninstall failed';
		}
	}

	function handleEdit(appId: string, isPowerLab: boolean) {
		if (isPowerLab) {
			// PowerLab apps (internal) use the fork flow to become custom
			forkingAppId = appId;
		} else {
			// Custom/Container apps go to the editor
			goto(`/apps/new?id=${appId}`);
		}
	}

	function confirmFork() {
		if (!forkingAppId) return;
		const id = forkingAppId;
		forkingAppId = null;
		goto(`/apps/new?id=${id}&fork=1`);
	}

	function clearSearch() {
		searchQuery = '';
		searchEl?.focus();
	}

	function openLogs(id: string) {
		selectedAppId = id;
		showLogsModal = true;
	}

	function openMetrics(id: string) {
		selectedAppId = id;
		showMetricsModal = true;
	}
</script>

<svelte:head>
	<title>Store — PowerLab</title>
</svelte:head>

<svelte:window onkeydown={onGlobalKey} />

<div class="flex h-full flex-col">

	<!-- ── Top bar ─────────────────────────────────────────────────────────── -->
	<div class="flex items-center justify-between border-b border-white/5 px-8 py-5">
		<div class="flex items-center gap-3">
			<a
				href="/"
				aria-label="Back to Launchpad"
				title="Back to Launchpad"
				class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl border border-white/[0.06] bg-white/[0.02] text-zinc-400 transition-all hover:-translate-x-0.5 hover:border-white/10 hover:bg-white/[0.05] hover:text-white"
			>
				<ArrowLeft class="h-4 w-4" />
			</a>
			<div>
				<h1 class="text-xl font-bold tracking-tight text-white">App Store</h1>
				<p class="text-xs text-zinc-500">
					{store.appStoreCatalog.length > 0 ? `${store.appStoreCatalog.length} apps available` : 'Loading catalog…'}
				</p>
			</div>
		</div>
		<a
			href="/apps/new"
			class="flex h-8 items-center gap-1.5 rounded-full border border-white/10 bg-white/5 px-4 text-xs font-semibold text-zinc-300 transition-colors hover:border-white/20 hover:text-white"
		>
			<Pencil class="h-3 w-3" /> Custom App
		</a>
	</div>

	<!-- ── Tab bar ─────────────────────────────────────────────────────────── -->
	<div class="flex gap-0 border-b border-white/5 px-8">
		<button
			class={cn(
				"relative pb-3 pt-3 text-xs font-semibold tracking-wide transition-colors mr-6",
				currentTab === 'store' ? 'text-white' : 'text-zinc-500 hover:text-zinc-300'
			)}
			onclick={() => { currentTab = 'store'; }}
		>
			Discover
			{#if currentTab === 'store'}
				<span class="absolute bottom-0 left-0 right-0 h-px bg-white"></span>
			{/if}
		</button>
		<button
			class={cn(
				"relative pb-3 pt-3 text-xs font-semibold tracking-wide transition-colors",
				currentTab === 'installed' ? 'text-white' : 'text-zinc-500 hover:text-zinc-300'
			)}
			onclick={() => { currentTab = 'installed'; }}
		>
			Installed
			{#if store.installedApps.length > 0}
				<span class="ml-1.5 rounded-full bg-white/10 px-1.5 py-0.5 text-[10px] font-bold text-zinc-400">
					{store.installedApps.length}
				</span>
			{/if}
			{#if currentTab === 'installed'}
				<span class="absolute bottom-0 left-0 right-0 h-px bg-white"></span>
			{/if}
		</button>
	</div>

	<!-- ── Store tab ───────────────────────────────────────────────────────── -->
	{#if currentTab === 'store'}
		<!-- Search bar -->
		<div class="px-8 pt-5 pb-4">
			<div class="relative">
				<Search class="absolute left-3.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-zinc-500" />
				<input
					bind:this={searchEl}
					bind:value={searchQuery}
					type="search"
					placeholder="Search apps…"
					class="h-9 w-full rounded-xl border border-white/8 bg-white/5 pl-9 pr-12 text-sm text-white placeholder:text-zinc-500 focus:border-white/20 focus:outline-none focus:ring-0 transition-colors"
				/>
				{#if searchQuery}
					<button
						class="absolute right-3 top-1/2 -translate-y-1/2 text-zinc-500 hover:text-white"
						onclick={clearSearch}
						aria-label="Clear search"
					>
						<X class="h-3.5 w-3.5" />
					</button>
				{:else}
					<!-- Hint: press `/` to focus this input from anywhere -->
					<kbd class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 rounded border border-white/10 bg-white/[0.04] px-1.5 py-0.5 font-mono text-[10px] font-semibold text-zinc-500">/</kbd>
				{/if}
			</div>
		</div>

		<!-- Category chips — hide when searching -->
		{#if !searchQuery && categories.length > 0}
			<div class="flex gap-2 overflow-x-auto px-8 pb-4 scrollbar-none" style="scrollbar-width: none">
				<button
					class={cn(
						"shrink-0 rounded-full px-3 py-1 text-xs font-semibold transition-colors",
						!activeCategory
							? 'bg-white text-black'
							: 'bg-white/5 text-zinc-400 hover:bg-white/10 hover:text-white'
					)}
					onclick={() => { activeCategory = null; }}
				>All</button>
				{#each categories as cat}
					<button
						class={cn(
							"shrink-0 rounded-full px-3 py-1 text-xs font-semibold transition-colors",
							activeCategory === cat
								? 'bg-white text-black'
								: 'bg-white/5 text-zinc-400 hover:bg-white/10 hover:text-white'
						)}
						onclick={() => { activeCategory = activeCategory === cat ? null : cat; }}
					>{cat}</button>
				{/each}
			</div>
		{/if}

		<!-- Results count when searching -->
		{#if searchQuery}
			<p class="px-8 pb-3 text-xs text-zinc-500">
				{filteredCatalog.length} result{filteredCatalog.length !== 1 ? 's' : ''} for
				<span class="text-zinc-300">"{searchQuery}"</span>
			</p>
		{/if}

		<!-- Error -->
		{#if store.error || installError}
			<div class="mx-8 mb-4 rounded-xl bg-red-500/10 px-4 py-2.5 text-xs text-red-400">
				{store.error ?? installError}
				<button class="ml-2 underline opacity-60 hover:opacity-100" onclick={() => { installError = null; }}>Dismiss</button>
			</div>
		{/if}

		<!-- App grid -->
		<div class="flex-1 overflow-y-auto px-8 pb-8">
			{#if !store.catalogLoaded}
				<!-- Skeleton — card grid (matches the default browse view) -->
				<div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
					{#each Array(8) as _}
						<div class="flex flex-col gap-3 rounded-2xl border border-white/[0.04] bg-white/[0.02] p-4">
							<div class="flex items-start gap-3">
								<div class="h-14 w-14 shrink-0 animate-pulse rounded-2xl bg-white/[0.04]"></div>
								<div class="flex-1 space-y-1.5 pt-1">
									<div class="h-3 w-3/4 animate-pulse rounded bg-white/[0.04]"></div>
									<div class="h-2.5 w-1/2 animate-pulse rounded bg-white/[0.04]"></div>
								</div>
							</div>
							<div class="h-2.5 w-full animate-pulse rounded bg-white/[0.04]"></div>
							<div class="h-2.5 w-2/3 animate-pulse rounded bg-white/[0.04]"></div>
						</div>
					{/each}
				</div>
			{:else if filteredCatalog.length === 0}
				<div class="flex flex-col items-center justify-center py-24 text-zinc-500">
					<Search class="mb-3 h-8 w-8 opacity-30" />
					<p class="text-sm">No apps found{searchQuery ? ` for "${searchQuery}"` : ''}.</p>
					{#if searchQuery}
						<button class="mt-2 text-xs text-zinc-400 hover:text-white" onclick={clearSearch}>
							Clear search
						</button>
					{/if}
				</div>
			{:else if searchQuery}
				<!-- LIST: when searching, dense rows are best for scanning matches -->
				<div class="divide-y divide-white/[0.04]">
					{#each filteredCatalog as app (app.store_app_id)}
						{@const appTitle = getTitle(app.title)}
						{@const appTagline = getTitle(app.tagline)}
						<!-- svelte-ignore a11y_click_events_have_key_events -->
						<!-- svelte-ignore a11y_no_static_element_interactions -->
						<div 
							class="group flex cursor-pointer items-center gap-3.5 py-3 pr-1"
							onclick={() => detailApp = app}
						>
							<div class="relative h-12 w-12 shrink-0">
								{#if app.icon}
									<img
										src={app.icon}
										alt={appTitle}
										class="h-12 w-12 rounded-xl object-contain bg-white/[0.03]"
										onerror={(e) => { (e.currentTarget as HTMLImageElement).style.display='none'; }}
									/>
								{:else}
									<div class="flex h-12 w-12 items-center justify-center rounded-xl bg-white/[0.05]">
										<Package class="h-5 w-5 text-zinc-500" />
									</div>
								{/if}
							</div>

							<div class="min-w-0 flex-1">
								<p class="truncate text-sm font-semibold text-white">{appTitle}</p>
								<p class="truncate text-xs text-zinc-500">{app.developer || app.author || ''}</p>
								{#if appTagline && appTagline !== 'Unknown'}
									<p class="mt-0.5 line-clamp-1 text-[11px] text-zinc-600">{appTagline}</p>
								{/if}
							</div>

							<button
								class="shrink-0 rounded-full border border-white/10 bg-white/5 px-3.5 py-1 text-xs font-semibold text-white transition-all hover:border-white/25 hover:bg-white/10 active:scale-95"
								onclick={(e) => { e.stopPropagation(); requestInstall(app); }}
							>
								Get
							</button>
						</div>
					{/each}
				</div>
			{:else}
				<!-- CARDS: hero icon + tagline + Get inline. Used for browsing
					 (no active search). Apple App Store-style: icon is the hero,
					 the Get button is right next to the title — never far. -->
				<div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
					{#each filteredCatalog as app (app.store_app_id)}
						{@const appTitle = getTitle(app.title)}
						{@const appTagline = getTitle(app.tagline)}
						<!-- svelte-ignore a11y_click_events_have_key_events -->
						<!-- svelte-ignore a11y_no_static_element_interactions -->
						<div 
							class="group relative flex cursor-pointer flex-col gap-3 rounded-2xl border border-white/[0.04] bg-white/[0.02] p-4 transition-colors hover:bg-white/[0.04]"
							onclick={() => detailApp = app}
						>
							<div class="flex items-start gap-3">
								<!-- Icon -->
								<div class="h-14 w-14 shrink-0">
									{#if app.icon}
										<img
											src={app.icon}
											alt={appTitle}
											class="h-14 w-14 rounded-2xl object-contain bg-white/[0.04]"
											onerror={(e) => { (e.currentTarget as HTMLImageElement).style.display='none'; }}
										/>
									{:else}
										<div class="flex h-14 w-14 items-center justify-center rounded-2xl bg-white/[0.06]">
											<Package class="h-6 w-6 text-zinc-500" />
										</div>
									{/if}
								</div>

								<!-- Title + dev + Get on same baseline as title -->
								<div class="min-w-0 flex-1">
									<div class="flex items-start justify-between gap-2">
										<p class="truncate text-sm font-semibold leading-tight text-white">{appTitle}</p>
										<button
											class="shrink-0 rounded-full border border-white/10 bg-white/5 px-3 py-0.5 text-[11px] font-bold text-white transition-all hover:border-emerald-500/40 hover:bg-emerald-500/10 hover:text-emerald-400 active:scale-95"
											onclick={(e) => { e.stopPropagation(); requestInstall(app); }}
										>
											Get
										</button>
									</div>
									<p class="mt-0.5 truncate text-[11px] text-zinc-500">{app.developer || app.author || ''}</p>
								</div>
							</div>

							{#if appTagline && appTagline !== 'Unknown'}
								<p class="line-clamp-2 text-[11px] leading-relaxed text-zinc-500">{appTagline}</p>
							{:else}
								<!-- Spacer so cards stay equal height -->
								<div class="h-[2.4em]"></div>
							{/if}

							{#if app.category}
								<div class="flex items-center gap-1.5">
									<span class="rounded-full bg-white/[0.04] px-2 py-px text-[9px] font-bold uppercase tracking-wider text-zinc-500">{app.category}</span>
								</div>
							{/if}
						</div>
					{/each}
				</div>
			{/if}
		</div>

	<!-- ── Installed tab ───────────────────────────────────────────────────── -->
	{:else}
		<div class="flex-1 overflow-y-auto px-8 pb-8 pt-5">
			{#if store.installedApps.length === 0}
				<div class="flex flex-col items-center justify-center py-32 text-center group">
					<div class="relative h-20 w-20 flex items-center justify-center rounded-3xl bg-white/[0.02] border border-white/5 shadow-inner transition-transform duration-500 group-hover:scale-110 mb-6">
						<Boxes class="h-10 w-10 text-zinc-500 group-hover:text-emerald-500 transition-colors duration-500" strokeWidth={1} />
					</div>
					<h3 class="text-lg font-black text-white tracking-tight mb-2">Build your Lab</h3>
					<p class="text-sm text-zinc-500 max-w-[240px] leading-relaxed mb-8">
						Your server is empty. Explore the App Store to install your first container.
					</p>
					<button
						class="flex items-center gap-2 rounded-xl bg-emerald-500 px-6 py-2.5 text-xs font-black uppercase tracking-widest text-zinc-950 transition-all hover:bg-emerald-400 hover:shadow-[0_0_20px_rgba(16,185,129,0.3)] active:scale-95"
						onclick={() => { currentTab = 'store'; }}
					>
						Browse the Store
						<ArrowRight class="h-4 w-4" />
					</button>
				</div>
			{:else}
				<div class="divide-y divide-white/[0.04]">
					{#each store.installedApps as app (app.store_info.store_app_id)}
						{@const info = app.store_info}
						{@const appTitle = getTitle(info.title)}
						{@const isRunning = app.status === 'running'}
						{@const ispl = store.isPowerLabApp(app)}
						<div class="group flex items-center gap-3.5 py-3">
							<!-- Icon -->
							<div class="h-11 w-11 shrink-0">
								{#if info.icon}
									<img src={info.icon} alt={appTitle} class="h-11 w-11 rounded-xl object-contain bg-white/[0.03]"
										onerror={(e) => { (e.currentTarget as HTMLImageElement).style.display='none'; }} />
								{:else}
									<div class="flex h-11 w-11 items-center justify-center rounded-xl bg-white/[0.05]">
										<Package class="h-5 w-5 text-zinc-500" />
									</div>
								{/if}
							</div>

							<!-- Info -->
							<div class="min-w-0 flex-1">
								<div class="flex items-center gap-2">
									<p class="truncate text-sm font-semibold text-white">{appTitle}</p>
									<!-- Type badge -->
									{#if ispl}
										<span class="shrink-0 rounded-full border border-emerald-500/20 bg-emerald-500/10 px-1.5 py-px text-[9px] font-bold uppercase tracking-widest text-emerald-400">PowerLab</span>
									{:else}
										<span class="shrink-0 rounded-full border border-amber-500/20 bg-amber-500/10 px-1.5 py-px text-[9px] font-bold uppercase tracking-widest text-amber-400">Custom</span>
									{/if}

									{#if app.update_available}
										<button 
											class="shrink-0 rounded-full bg-emerald-500/20 px-1.5 py-px text-[9px] font-black uppercase tracking-wider text-emerald-500 border border-emerald-500/30 hover:bg-emerald-500 hover:text-zinc-950 transition-all shadow-[0_0_10px_rgba(16,185,129,0.1)] active:scale-95"
											onclick={() => confirmingUpdate = app}
											title="Show update details"
										>
											Update Available
										</button>
									{/if}
								</div>
								<p class="text-xs text-zinc-500">
									<span class={cn("mr-1.5 inline-block h-1.5 w-1.5 rounded-full align-middle", isRunning ? 'bg-emerald-500' : 'bg-zinc-600')}></span>
									{isRunning ? 'Running' : 'Stopped'}
									{#if info.port_map}· port {info.port_map}{/if}
									<span class="ml-2 rounded-md bg-white/[0.03] px-1.5 py-0.5 text-[9px] font-bold text-zinc-500 uppercase tracking-widest">{formatSize(store.getDiskUsage(app.id) || 0)}</span>
								</p>
							</div>

							<!-- Actions (appear on hover) -->
							<div class="flex items-center gap-1 opacity-0 transition-opacity group-hover:opacity-100">
								<button
									class="flex h-7 w-7 items-center justify-center rounded-lg text-zinc-500 transition-colors hover:bg-white/5 hover:text-white"
									title={isRunning ? 'Stop' : 'Start'}
									onclick={() => store.toggleAppStatus(app.id, app.status)}
								>
									{#if isRunning}
										<Square class="h-3.5 w-3.5" />
									{:else}
										<Play class="h-3.5 w-3.5" />
									{/if}
								</button>
								<button
									class="flex h-7 w-7 items-center justify-center rounded-lg text-zinc-500 transition-colors hover:bg-white/5 hover:text-white"
									title="Metrics"
									onclick={() => openMetrics(info.store_app_id)}
								>
									<Activity class="h-3.5 w-3.5" />
								</button>
								<button
									class="flex h-7 w-7 items-center justify-center rounded-lg text-zinc-500 transition-colors hover:bg-white/5 hover:text-white"
									title="Logs"
									onclick={() => openLogs(info.store_app_id)}
								>
									<ScrollText class="h-3.5 w-3.5" />
								</button>
								<button
									class="flex h-7 w-7 items-center justify-center rounded-lg text-zinc-500 transition-colors hover:bg-white/5 hover:text-white"
									title={ispl ? 'Fork as Custom App' : 'Edit'}
									onclick={() => handleEdit(info.store_app_id, ispl)}
								>
									{#if ispl}
										<ChevronRight class="h-3.5 w-3.5" />
									{:else}
										<Pencil class="h-3.5 w-3.5" />
									{/if}
								</button>
								<button
									class="flex h-7 w-7 items-center justify-center rounded-lg text-zinc-500 transition-colors hover:bg-red-500/10 hover:text-red-400"
									title="Uninstall"
									onclick={() => { confirmingUninstall = info.store_app_id; }}
								>
									<Trash2 class="h-3.5 w-3.5" />
								</button>
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</div>
	{/if}
</div>

<!-- ── Modals ───────────────────────────────────────────────────────────────── -->

{#if showLogsModal}
	<ContainerLogs appId={selectedAppId} onClose={() => showLogsModal = false} />
{/if}

{#if showMetricsModal}
	<AppMetrics appId={selectedAppId} onClose={() => showMetricsModal = false} />
{/if}

{#if installPhase === 'confirm' && pendingInstallApp}
	<div class="fixed inset-0 z-50 flex items-end justify-center bg-black/50 backdrop-blur-sm sm:items-center">
		<div class="w-full max-w-sm rounded-t-[2rem] border border-white/8 bg-zinc-900 p-6 sm:rounded-2xl">
			<div class="mb-4 flex items-center gap-3">
				{#if pendingInstallApp.icon}
					<img src={pendingInstallApp.icon} alt="" class="h-12 w-12 rounded-xl" onerror={(e) => { (e.target as HTMLImageElement).style.display='none'; }} />
				{:else}
					<div class="flex h-12 w-12 items-center justify-center rounded-xl bg-zinc-800"><Package class="h-6 w-6 text-zinc-500" /></div>
				{/if}
				<div>
					<p class="font-semibold text-white">{getTitle(pendingInstallApp.title)}</p>
					<p class="text-xs text-zinc-500">{pendingInstallApp.developer || pendingInstallApp.author}</p>
				</div>
			</div>
			<p class="mb-4 text-sm text-zinc-400">
				This will pull the Docker image and start the container.
			</p>

			{#if isCheckingCompatibility}
				<div class="mb-5 flex items-center gap-2 rounded-xl bg-white/5 p-3 text-xs text-zinc-500">
					<Loader2 class="h-3 w-3 animate-spin" />
					Checking compatibility…
				</div>
			{:else}
				{#if compatibilityWarnings.length > 0}
					<div class="mb-3 space-y-2">
						<p class="text-[10px] font-bold uppercase tracking-widest text-zinc-500">Compatibility Warnings</p>
						<div class="space-y-1.5">
							{#each compatibilityWarnings as warning}
								<div class="flex items-start gap-2 rounded-xl bg-amber-500/10 p-2.5 text-[11px] leading-tight text-amber-200/80 border border-amber-500/10">
									<AlertCircle class="h-3 w-3 shrink-0 mt-0.5 text-amber-500" />
									<span>{warning}</span>
								</div>
							{/each}
						</div>
					</div>
				{/if}

				{#if portChoices.length > 0}
					<div class="mb-3 space-y-2">
						<p class="text-[10px] font-bold uppercase tracking-widest text-zinc-500">Port Conflicts</p>
						<div class="space-y-2">
							{#each portChoices as choice}
								<div class="flex items-center gap-2.5 rounded-xl border border-white/[0.06] bg-white/[0.02] px-3 py-2.5">
									<span class="font-mono text-xs text-zinc-500 line-through">{choice.original}</span>
									<span class="text-zinc-700">→</span>
									<input
										type="number"
										min="1"
										max="65535"
										bind:value={choice.chosen}
										oninput={onPortInput}
										class={cn(
											'w-20 rounded-md border bg-white/[0.03] px-2 py-1 text-center font-mono text-xs outline-none transition-colors',
											choice.status === 'free' && 'border-emerald-500/30 text-emerald-300 focus:border-emerald-500/60',
											choice.status === 'inuse' && 'border-red-500/40 text-red-300 focus:border-red-500/70',
											choice.status === 'invalid' && 'border-red-500/40 text-red-300',
											choice.status === 'checking' && 'border-white/10 text-zinc-400'
										)}
									/>
									{#if choice.status === 'checking'}
										<Loader2 class="h-3.5 w-3.5 animate-spin text-zinc-500" />
									{:else if choice.status === 'free'}
										<CheckCircle2 class="h-3.5 w-3.5 text-emerald-400" />
									{:else}
										<AlertCircle class="h-3.5 w-3.5 text-red-400" />
									{/if}
									{#if choice.status === 'inuse' || choice.status === 'invalid'}
										<button
											type="button"
											onclick={() => autoPickPort(choice)}
											class="ml-auto rounded-md bg-white/[0.04] px-2 py-1 text-[10px] font-bold uppercase tracking-wider text-zinc-300 transition-colors hover:bg-white/[0.08]"
										>
											Auto
										</button>
									{/if}
								</div>
							{/each}
						</div>
						<p class="px-1 text-[10px] text-zinc-600">
							{portsResolved
								? 'All chosen ports are free. Install will use these.'
								: 'Edit the highlighted ports or click Auto to pick the next free one.'}
						</p>
					</div>
				{/if}
			{/if}

			<div class="flex gap-2">
				<Button variant="ghost" class="flex-1 rounded-xl" onclick={() => { installPhase = 'idle'; pendingInstallApp = null; }}>Cancel</Button>
				<Button
					class={cn(
						'flex-1 rounded-xl font-bold',
						hasCriticalWarning ? 'bg-red-600 text-white hover:bg-red-500' : ''
					)}
					disabled={isCheckingCompatibility || !portsResolved}
					onclick={executeInstall}
				>
					{hasCriticalWarning ? 'Install Anyway' : 'Install'}
				</Button>
			</div>
		</div>
	</div>
{/if}

{#if forkingAppId}
	<div class="fixed inset-0 z-50 flex items-end justify-center bg-black/50 backdrop-blur-sm sm:items-center">
		<div class="w-full max-w-sm rounded-t-[2rem] border border-white/8 bg-zinc-900 p-6 sm:rounded-2xl">
			<div class="mb-2 flex items-center gap-2">
				<Pencil class="h-4 w-4 text-amber-400" />
				<p class="font-semibold text-white">Fork as Custom App</p>
			</div>
			<p class="mb-1 text-sm text-zinc-400">
				Your edits will be saved as a <span class="text-amber-400">Custom App</span>. The original PowerLab version stays unchanged.
			</p>
			<p class="mb-5 text-xs text-zinc-600">Give it a unique name in the editor before deploying.</p>
			<div class="flex gap-2">
				<Button variant="ghost" class="flex-1 rounded-xl" onclick={() => { forkingAppId = null; }}>Cancel</Button>
				<Button class="flex-1 rounded-xl bg-amber-500 text-black hover:bg-amber-400 font-bold" onclick={confirmFork}>Open Editor</Button>
			</div>
		</div>
	</div>
{/if}

{#if confirmingUninstall}
	<div class="fixed inset-0 z-50 flex items-end justify-center bg-black/50 backdrop-blur-sm sm:items-center">
		<div class="w-full max-w-sm rounded-t-[2rem] border border-white/8 bg-zinc-900 p-6 sm:rounded-2xl">
			<p class="mb-1 font-semibold text-white">Uninstall App</p>
			<p class="mb-4 text-sm text-zinc-400">Remove this app and its containers?</p>
			
			<label class="mb-6 flex cursor-pointer items-center gap-3 rounded-xl border border-white/5 bg-white/[0.02] p-4 transition-colors hover:bg-white/[0.04]">
				<input 
					type="checkbox" 
					bind:checked={deleteData}
					class="h-4 w-4 rounded border-zinc-700 bg-zinc-800 text-red-600 focus:ring-red-600 focus:ring-offset-zinc-900" 
				/>
				<div class="flex flex-col">
					<span class="text-xs font-bold text-white uppercase tracking-wider">Delete app data</span>
					<span class="text-[10px] text-zinc-500">This will permanently remove all configuration and files.</span>
				</div>
			</label>

			<div class="flex gap-2">
				<Button variant="ghost" class="flex-1 rounded-xl" onclick={() => { confirmingUninstall = null; deleteData = false; }}>Cancel</Button>
				<Button class="flex-1 rounded-xl bg-red-600 text-white hover:bg-red-500 font-bold" onclick={() => handleUninstall(confirmingUninstall!)}>
					Uninstall
				</Button>
			</div>
		</div>
	</div>
{/if}

{#if installPhase !== 'idle' && installPhase !== 'confirm'}
	<!-- Install progress overlay -->
	<div class="fixed inset-0 z-50 flex flex-col bg-zinc-950/95 backdrop-blur-xl">
		<div class="flex flex-1 flex-col items-center justify-center gap-5 px-6 text-center">

			<!-- App icon with state ring/badge -->
			<div class="relative shrink-0">
				{#if installingApp?.icon}
					<img
						src={installingApp.icon}
						alt=""
						class="h-20 w-20 rounded-[20px] shadow-2xl"
						onerror={(e) => { (e.target as HTMLImageElement).style.display='none'; }}
					/>
				{:else}
					<div class="flex h-20 w-20 items-center justify-center rounded-[20px] bg-zinc-800">
						<Package class="h-10 w-10 text-zinc-500" />
					</div>
				{/if}

				{#if installPhase === 'installing' || installPhase === 'starting'}
					<div class="absolute inset-0 animate-ping rounded-[20px] border-2 border-emerald-500/50" style="animation-duration:1.8s"></div>
				{:else if installPhase === 'success'}
					<div class="absolute -bottom-2 -right-2 flex h-8 w-8 items-center justify-center rounded-full bg-emerald-500 shadow-lg ring-4 ring-zinc-950">
						<CheckCircle2 class="h-5 w-5 text-white" />
					</div>
				{:else if installPhase === 'error' || installPhase === 'timeout'}
					<div class="absolute -bottom-2 -right-2 flex h-8 w-8 items-center justify-center rounded-full bg-red-600 shadow-lg ring-4 ring-zinc-950">
						<X class="h-5 w-5 text-white" />
					</div>
				{/if}
			</div>

			<!-- Status text -->
			<div class="shrink-0">
				<p class="text-lg font-bold text-white">
					{#if installPhase === 'success'}
						{getTitle(installingApp?.title ?? {})} is running
					{:else if installPhase === 'error'}
						Installation failed
					{:else if installPhase === 'timeout'}
						Taking longer than expected
					{:else}
						Installing {getTitle(installingApp?.title ?? {})}…
					{/if}
				</p>
				{#if installPhase === 'installing'}
					<p class="mt-1 text-sm text-zinc-500">Preparing installation…</p>
				{:else if installPhase === 'starting'}
					<p class="mt-1 text-sm text-zinc-500">Pulling Docker image and starting containers</p>
				{:else if installPhase === 'success'}
					<p class="mt-1 text-sm text-zinc-500">App is running and available on your Launchpad.</p>
					{#if installPortNote}
						<p class="mt-2 inline-flex items-center gap-1.5 rounded-full bg-emerald-500/10 px-2.5 py-0.5 text-[11px] font-medium text-emerald-400">
							{installPortNote}
						</p>
					{/if}
				{:else if installPhase === 'timeout'}
					<p class="mt-1 text-sm text-zinc-400">The install is still running in the background. Check your Launchpad in a few minutes.</p>
				{:else if installPhase === 'error'}
					<p class="mt-2 max-w-xs rounded-xl bg-red-950/50 px-4 py-2 text-xs leading-relaxed text-red-400">{installError}</p>
				{/if}
			</div>

			<!-- Live install log — shown during 'starting' and on error/timeout if logs available -->
			{#if (installPhase === 'starting' || installPhase === 'error' || installPhase === 'timeout') && installLogs}
				<div class="w-full max-w-lg shrink overflow-hidden rounded-2xl border border-white/[0.06] bg-black/40">
					<div class="flex items-center gap-2 border-b border-white/[0.06] px-3 py-2">
						<div class="h-1.5 w-1.5 animate-pulse rounded-full bg-emerald-500"></div>
						<span class="font-mono text-[10px] text-zinc-500">install log</span>
					</div>
					<pre
						bind:this={installLogEl}
						class="h-40 overflow-y-auto p-3 font-mono text-[11px] leading-relaxed text-zinc-400 scrollbar-none"
						style="scrollbar-width:none"
					>{installLogs}</pre>
				</div>
			{:else if installPhase === 'installing'}
				<div class="flex items-center gap-1.5">
					{#each [0, 1, 2] as i}
						<div class="h-2 w-2 animate-bounce rounded-full bg-emerald-500" style="animation-delay:{i * 150}ms"></div>
					{/each}
				</div>
			{/if}

			<!-- Action buttons -->
			<div class="flex w-full max-w-xs shrink-0 gap-2">
				{#if installPhase === 'success'}
					<div class="mt-8 flex gap-3">
						<Button class="flex-1 rounded-xl bg-emerald-500 text-black font-bold hover:bg-emerald-400" onclick={() => { installPhase = 'idle'; }}>
							View Launchpad
						</Button>
						<Button variant="outline" class="flex-1 rounded-xl border-white/10" onclick={() => { installPhase = 'idle'; goto('/'); }}>
							Go Home
						</Button>
					</div>
				{:else if installPhase === 'error'}
					<Button variant="ghost" class="flex-1 rounded-xl" onclick={closeInstallOverlay}>Dismiss</Button>
					<Button class="flex-1 rounded-xl" onclick={() => { pendingInstallApp = installingApp; closeInstallOverlay(); setTimeout(() => installPhase = 'confirm', 50); }}>
						Retry
					</Button>
				{:else if installPhase === 'timeout'}
					<Button class="flex-1 rounded-xl" onclick={closeInstallOverlay}>Check Launchpad</Button>
				{:else}
					<Button variant="ghost" class="flex-1 rounded-xl text-zinc-600" onclick={closeInstallOverlay}>Cancel</Button>
				{/if}
			</div>
		</div>
	</div>
{/if}
<!-- App Detail Modal -->
{#if detailApp}
	{@const appTitle = getTitle(detailApp.title)}
	{@const appTagline = getTitle(detailApp.tagline)}
	{@const appDesc = getTitle(detailApp.description)}
	
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div 
		class="fixed inset-0 z-[70] flex items-center justify-center p-4 bg-zinc-950/80 backdrop-blur-md"
		onclick={() => detailApp = null}
		transition:fade={{ duration: 200 }}
	>
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div 
			class="relative w-full max-w-2xl max-h-[90vh] flex flex-col overflow-hidden rounded-[2.5rem] border border-white/[0.08] bg-zinc-900 shadow-[0_32px_64px_rgba(0,0,0,0.5)]"
			onclick={(e) => e.stopPropagation()}
		>
			<!-- Close button -->
			<button 
				class="absolute right-6 top-6 z-10 flex h-10 w-10 items-center justify-center rounded-full bg-white/5 text-zinc-400 backdrop-blur-md transition-all hover:bg-white/10 hover:text-white"
				onclick={() => detailApp = null}
			>
				<X class="h-5 w-5" />
			</button>

			<div class="flex-1 overflow-y-auto p-8 pt-10 scrollbar-none" style="scrollbar-width: none">
				<!-- Header Section -->
				<div class="flex flex-col items-center text-center sm:flex-row sm:text-left sm:items-start gap-6">
					<div class="h-24 w-24 shrink-0 shadow-2xl">
						{#if detailApp.icon}
							<img src={detailApp.icon} alt={appTitle} class="h-24 w-24 rounded-[2rem] object-contain bg-white/[0.05] border border-white/10" />
						{:else}
							<div class="flex h-24 w-24 items-center justify-center rounded-[2rem] bg-white/[0.05] border border-white/10">
								<Package class="h-10 w-10 text-zinc-500" />
							</div>
						{/if}
					</div>
					<div class="flex-1 pt-1">
						<h1 class="text-2xl font-black tracking-tight text-white">{appTitle}</h1>
						<p class="text-sm font-medium text-emerald-500">{detailApp.developer || detailApp.author || 'Independent Developer'}</p>
						{#if appTagline && appTagline !== 'Unknown'}
							<p class="mt-2 text-sm leading-relaxed text-zinc-400">{appTagline}</p>
						{/if}
						<div class="mt-4 flex flex-wrap justify-center sm:justify-start gap-2">
							{#if detailApp.category}
								<span class="rounded-full bg-white/5 border border-white/10 px-3 py-1 text-[10px] font-bold uppercase tracking-wider text-zinc-400">{detailApp.category}</span>
							{/if}
						</div>
					</div>
				</div>

				<!-- Screenshots Carrousel -->
				{#if detailApp.screenshot_link && detailApp.screenshot_link.length > 0}
					<div class="mt-10 overflow-hidden">
						<h3 class="mb-4 text-xs font-bold uppercase tracking-widest text-zinc-500">Preview</h3>
						<div class="flex gap-4 overflow-x-auto pb-2 scrollbar-none" style="scrollbar-width: none">
							{#each detailApp.screenshot_link as shot}
								<img src={shot} alt="Screenshot" class="h-48 rounded-2xl border border-white/10 bg-white/[0.02] shadow-lg" />
							{/each}
						</div>
					</div>
				{/if}

				<!-- Description Section -->
				<div class="mt-10">
					<h3 class="mb-4 text-xs font-bold uppercase tracking-widest text-zinc-500">About this App</h3>
					<Markdown content={appDesc} />
				</div>
			</div>

			<!-- Footer Action -->
			<div class="border-t border-white/[0.08] bg-white/[0.02] p-6 backdrop-blur-xl">
				<div class="flex items-center justify-between gap-4">
					<div class="hidden sm:block">
						<p class="text-[10px] font-bold uppercase tracking-widest text-zinc-500">Open Source</p>
						<p class="text-xs text-zinc-400">Verified and secure installation</p>
					</div>
					<Button 
						class="h-12 w-full sm:w-40 rounded-2xl bg-white text-zinc-950 font-bold hover:bg-emerald-500 hover:text-zinc-950 transition-all shadow-[0_8px_24px_rgba(255,255,255,0.15)] active:scale-95"
						onclick={() => { 
							const app = detailApp;
							detailApp = null; 
							if (app) requestInstall(app); 
						}}
					>
						Get App
					</Button>
				</div>
			</div>
		</div>
	</div>
{/if}

{#if confirmingUpdate}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4" transition:fade={{ duration: 200 }}>
		<div class="relative w-full max-w-md rounded-3xl border border-white/10 bg-zinc-950 p-8 shadow-2xl overflow-hidden" transition:scale={{ duration: 300, start: 0.95 }}>
			<div class="absolute inset-0 bg-gradient-to-b from-emerald-500/5 to-transparent pointer-events-none"></div>
			
			<div class="relative flex flex-col items-center text-center">
				<div class="mb-6 flex h-20 w-20 items-center justify-center rounded-3xl bg-emerald-500 text-zinc-950 shadow-[0_0_30px_rgba(16,185,129,0.3)]">
					<ArrowUpCircle class="h-10 w-10" />
				</div>

				<h2 class="mb-2 text-2xl font-black tracking-tight text-white">Update Available</h2>
				<p class="mb-8 text-sm font-medium text-zinc-400">
					A new version of <span class="text-white font-bold">{getTitle(confirmingUpdate.store_info.title)}</span> is ready to be installed.
				</p>

				<div class="mb-10 w-full space-y-3 rounded-2xl bg-white/[0.03] border border-white/5 p-4">
					<div class="flex items-center justify-between">
						<span class="text-[10px] font-bold text-zinc-500 uppercase tracking-widest">Type</span>
						<span class="text-xs font-bold text-emerald-400">Rolling Update</span>
					</div>
					<div class="h-px bg-white/5"></div>
					<div class="flex items-center justify-between">
						<span class="text-[10px] font-bold text-zinc-600 uppercase tracking-[0.2em]">{formatSize(store.getDiskUsage(confirmingUpdate.id) || 0)}</span>
						<span class="text-xs font-mono text-zinc-400 truncate max-w-[200px]">{confirmingUpdate.store_info.image?.en_us || confirmingUpdate.store_info.thumbnail || 'latest'}</span>
					</div>
				</div>

				<div class="flex w-full gap-3">
					<Button 
						variant="ghost" 
						class="flex-1 rounded-2xl h-12 text-zinc-500 hover:text-white hover:bg-white/5 font-bold" 
						onclick={() => confirmingUpdate = null}
					>
						Cancel
					</Button>
					<Button 
						class="flex-1 rounded-2xl h-12 bg-emerald-500 text-zinc-950 hover:bg-emerald-400 font-black shadow-[0_0_20px_rgba(16,185,129,0.2)]" 
						onclick={handleUpdate}
					>
						Update Now
					</Button>
				</div>
			</div>
		</div>
	</div>
{/if}
