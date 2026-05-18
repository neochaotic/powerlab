<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { useSettingsStore } from '$lib/stores/settings.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import {
		SlidersHorizontal, Network, Boxes, Info, Globe, Clock, Hash,
		Power, RefreshCw, Copy, Check, ExternalLink, ShieldCheck, KeyRound,
		Code2, Scale, Heart, Sparkles, Container, Zap, Wifi, AlertTriangle,
		AlertCircle, ClipboardList, FileText, Store
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
	import { computeRedirectIntent } from '$lib/utils/trust-dance';
	import { t, setLocale, getLocale, availableLocales } from '$lib/i18n/index.svelte';
	import { Download, Languages } from 'lucide-svelte';
	import AppsPane from '$lib/components/settings/AppsPane.svelte';
	import GeneralPane from '$lib/components/settings/GeneralPane.svelte';
	import NetworkPane from '$lib/components/settings/NetworkPane.svelte';
	import SecurityPane from '$lib/components/settings/SecurityPane.svelte';
	import AboutPane from '$lib/components/settings/AboutPane.svelte';
	import AuditPane from '$lib/components/settings/AuditPane.svelte';
	import LogsPane from '$lib/components/settings/LogsPane.svelte';
	import CatalogPane from '$lib/components/settings/CatalogPane.svelte';

	const store = useSettingsStore();

	type Section = 'general' | 'network' | 'apps' | 'catalog' | 'security' | 'audit' | 'logs' | 'about';
	let activeSection = $state<Section>('general');
	let copiedKey = $state<string | null>(null);

	// Security HTTPS Onboarding state
	let activeSecurityTab = $state<OS>('unknown');
	let isTestingConnection = $state(false);

	// Deep-link support: a URL hash like /settings#security or
	// /settings#network jumps straight to that tab. Used by HttpBanner
	// to bring the user here from anywhere in the app.
	const VALID_SECTIONS: Section[] = ['general', 'network', 'apps', 'catalog', 'security', 'audit', 'logs', 'about'];
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

			// All guards passed: persist the server's CA fingerprint
			// so the TrustStateChecker can detect a future mismatch
			// (CA regen / rotation) and re-prompt re-install. Best
			// effort — never blocks the trust dance.
			try {
				const stateResp = await fetch(`${httpsBase}/v1/sys/trust-state`, {
					mode: 'cors',
					signal: AbortSignal.timeout(3000)
				});
				if (stateResp.ok) {
					const state = (await stateResp.json()) as { ca_fingerprint?: string };
					if (state.ca_fingerprint) {
						window.localStorage.setItem('powerlab_trusted_ca_fp', state.ca_fingerprint);
						window.localStorage.removeItem('powerlab_ca_mismatch_dismissed');
					}
				}
			} catch {
				/* fingerprint persistence is best-effort */
			}

			// Guard 4: only redirect when the destination will render.
			if (!canRedirect) {
				const note = isLocalhost
					? 'Trust verified. (HSTS arming is skipped on localhost — visit via your LAN IP to complete that step.)'
					: 'Trust established. The certificate is valid; visit the secure URL when ready.';
				toast.success(note);
				return;
			}

			// Distinguish redirect from already-secure no-op (#7).
			// In production the user typically opens the secure URL
			// directly, so window.location.href === target — naive
			// assignment was a silent no-op and the user reported
			// "Verify did nothing" even though the dance succeeded.
			const intent = computeRedirectIntent(window.location.href, '8443');
			if (intent.kind === 'already-secure') {
				toast.success(t('settings.alreadySecure'));
				return;
			}
			toast.success(t('settings.trustEstablishedRedirect'));
			setTimeout(() => {
				window.location.href = intent.targetUrl;
			}, 1500);

		} catch (e) {
			console.error('HTTPS test failed:', e);
			toast.error(`Connection test failed: ${(e as Error).message || 'unknown error'}. Confirm the certificate is installed and trusted.`);
		} finally {
			isTestingConnection = false;
		}
	}

	// downloadCA — fetch the CA file via XHR and trigger a programmatic
	// download. Replaces `window.location.href = '/v1/sys/...'` because
	// that pattern navigates the browser to the URL, and on any
	// non-2xx response the user sees a plain-text error page instead
	// of a download. JS-driven download:
	//   - pre-flight: failure → toast, page stays put
	//   - saves to the BROWSER's local Downloads, never the server
	//   - filename + content-type stay correct for the OS install flow
	//
	// Same pattern recommended in issue #50.
	async function downloadCA(format: 'mobileconfig' | 'crt' | 'cer') {
		const url = `/v1/sys/ca-certificate.${format}`;
		try {
			const r = await fetch(url);
			if (!r.ok) {
				// Surface enough info that the next bug-report has a
				// fingerprint to act on. Previous "Could not download"
				// message swallowed status code + body, leaving us
				// guessing during user testing (#118).
				let bodyHint = '';
				try {
					const text = await r.text();
					if (text && text.length < 200) bodyHint = ` — ${text}`;
				} catch { /* body unreadable; ignore */ }
				const detail = `HTTP ${r.status}${bodyHint}`;
				console.error('CA download failed', { url, status: r.status, detail });
				toast.error(`${t('settings.caDownloadFailed')} (${detail})`);
				return;
			}
			const blob = await r.blob();
			if (blob.size === 0) {
				console.error('CA download returned empty body', { url });
				toast.error(`${t('settings.caDownloadFailed')} (empty body)`);
				return;
			}
			const objectUrl = URL.createObjectURL(blob);
			const a = document.createElement('a');
			a.href = objectUrl;
			a.download = `powerlab-ca.${format}`;
			document.body.appendChild(a);
			a.click();
			a.remove();
			URL.revokeObjectURL(objectUrl);
		} catch (e) {
			const msg = (e as Error).message;
			console.error('CA download exception', { url, error: e });
			toast.error(`${t('settings.caDownloadFailed')}: ${msg}`);
		}
	}

	// Chrome (and Safari/Edge to a lesser extent) blocks .crt /
	// .mobileconfig / .config downloads when the originating page is
	// HTTPS-with-untrusted-cert — exactly the catch-22 the user is
	// in DURING the Trust Onboarding bootstrap. The remaining escape
	// hatch is `openHttpDownload`, which opens the download URL on
	// the HTTP variant of the panel: top-level HTTP navigation is
	// treated as Mixed Content (warned but allowed), and once on
	// HTTP the high-risk-file rule no longer fires.
	//
	// PowerLab does not surface the certificate as text. Past
	// iterations included a `.txt` rename trick and an inline
	// "Show as text + copy clipboard" view; both were removed
	// (per product policy: cert is a binary artifact, not text).
	function openHttpDownload(format: 'mobileconfig' | 'crt' | 'cer') {
		// Build the HTTP variant of the panel URL. Top-level navigation
		// to HTTP from an HTTPS page is treated as a Mixed Content
		// navigation, not a Mixed Content download — Chrome warns but
		// allows it. Once on HTTP, the .crt download is unblocked
		// because the high-risk-file rule only fires from HTTPS-with-
		// untrusted contexts.
		const port = location.port || '8765';
		// HTTPS port → HTTP port. The gateway listens on both. Default
		// pairing: 8443 ↔ 8765, 443 ↔ 80, 8080 stays.
		let httpPort = port;
		if (port === '8443') httpPort = '8765';
		else if (port === '443') httpPort = '80';
		const url = `http://${location.hostname}${httpPort === '80' ? '' : ':' + httpPort}/v1/sys/ca-certificate.${format}`;
		window.open(url, '_blank', 'noopener,noreferrer');
	}

	const isHttpsSelfSigned = $derived.by(() => {
		// We can't introspect the cert chain from JS — but if the page
		// loaded at all over HTTPS without the user dismissing a hard
		// block, Chrome decided to render. Show the workaround section
		// proactively whenever HTTPS is in use; it's tiny and avoids
		// the dead-end where the download silently fails.
		return typeof location !== 'undefined' && location.protocol === 'https:';
	});

	// resetTrust clears the HSTS gate on the server and the cached
	// CA fingerprint on this device. The CA itself is untouched —
	// the next visit re-runs the trust dance with the existing
	// installed CA. Use this when the dance got into a weird state
	// and you want to redo it cleanly.
	async function resetTrust() {
		const ok = confirm(
			'Reset trust? The certificate stays the same; you will be guided through the trust dance again. No action needed on your devices.'
		);
		if (!ok) return;
		try {
			const r = await fetch('/v1/sys/trust-confirmed', { method: 'DELETE' });
			if (!r.ok) throw new Error(`status ${r.status}`);
			window.localStorage.removeItem('powerlab_trusted_ca_fp');
			window.localStorage.removeItem('powerlab_ca_mismatch_dismissed');
			toast.success(t('settings.trustResetSuccess'));
		} catch (e) {
			toast.error(`Could not reset trust: ${(e as Error).message}`);
		}
	}

	// confirmRotateCA opens the destructive-confirmation modal. The
	// modal owns the type-to-confirm phrase + the explicit list of
	// consequences. The actual API call is in `executeRotateCA`,
	// only reachable from the modal's "Rotate now" button after the
	// user typed the right phrase.
	let isRotateModalOpen = $state(false);
	let rotateConfirmPhrase = $state('');
	let isRotating = $state(false);

	function confirmRotateCA() {
		rotateConfirmPhrase = '';
		isRotateModalOpen = true;
	}

	function cancelRotate() {
		isRotateModalOpen = false;
		rotateConfirmPhrase = '';
	}

	async function executeRotateCA() {
		if (rotateConfirmPhrase !== 'ROTATE') return;
		isRotating = true;
		try {
			const httpsBase = `https://${window.location.hostname}:8443`;
			const r = await fetch(
				`${httpsBase}/v1/sys/rotate-ca?confirm=ROTATE_CA`,
				{ method: 'POST', mode: 'cors' }
			);
			if (!r.ok) {
				const detail = await r.text().catch(() => '');
				throw new Error(`status ${r.status}: ${detail.slice(0, 120)}`);
			}
			window.localStorage.removeItem('powerlab_trusted_ca_fp');
			window.localStorage.removeItem('powerlab_ca_mismatch_dismissed');
			isRotateModalOpen = false;
			rotateConfirmPhrase = '';
			toast.success(t('settings.caRotatedSuccess'));
		} catch (e) {
			toast.error(`Rotation failed: ${(e as Error).message}`);
		} finally {
			isRotating = false;
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
			toast.error(t('settings.portRangeError'));
			return;
		}
		if (String(portInput) === currentPort) {
			toast.info(t('settings.alreadyCurrentPort'));
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
		{ id: 'catalog',  label: 'Catalog',  icon: Store,             desc: 'Sources for installable apps' },
		{ id: 'security', label: 'Security', icon: ShieldCheck,       desc: 'Password, sessions' },
		{ id: 'audit',    label: 'Audit',    icon: ClipboardList,     desc: 'API request log' },
		{ id: 'logs',     label: 'Logs',     icon: FileText,          desc: 'Service stdout files' },
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
					<GeneralPane
						osHostname={store.utilization?.os?.hostname || ''}
						timezone={store.timezone}
						onTimezoneChange={(v) => store.setTimezone(v)}
						{reachableUrl}
						{currentPort}
						{portInput}
						onPortInputChange={(v) => portInput = v}
						onRequestPortChange={requestPortChange}
						{timezones}
					/>
				</div>

			{:else if activeSection === 'network'}
				<div in:fade={{ duration: 150 }}>
					<NetworkPane
						{mdnsHostname}
						{reachableUrl}
						{copiedKey}
						onCopy={copy}
						networkInterfaces={store.networkInterfaces}
					/>
				</div>

			{:else if activeSection === 'apps'}
				<div in:fade={{ duration: 150 }}>
					<AppsPane {storagePath} {copiedKey} onCopy={copy} />
				</div>

			{:else if activeSection === 'catalog'}
				<div in:fade={{ duration: 150 }}>
					<CatalogPane />
				</div>

			{:else if activeSection === 'security'}
				<div in:fade={{ duration: 150 }}>
					<SecurityPane
						{activeSecurityTab}
						onTabChange={(tab) => activeSecurityTab = tab}
						{isTestingConnection}
						{isHttpsSelfSigned}
						onDownloadCA={downloadCA}
						onOpenHttpDownload={openHttpDownload}
						onTestHttpsConnection={testHttpsConnection}
						onResetTrust={resetTrust}
						onConfirmRotateCA={confirmRotateCA}
					/>
				</div>

			{:else if activeSection === 'audit'}
				<div in:fade={{ duration: 150 }}>
					<AuditPane />
				</div>

			{:else if activeSection === 'logs'}
				<div in:fade={{ duration: 150 }}>
					<LogsPane />
				</div>

			{:else if activeSection === 'about'}
				<div in:fade={{ duration: 150 }}>
					<AboutPane />
				</div>
			{/if}

		</div>
	</main>
</div>

<!-- Rotate-CA confirmation modal — destructive, type-to-confirm.
	 Voids trust on every installed device, so the prompt is
	 deliberately scary: rose accent, explicit list of consequences,
	 the user has to type ROTATE before the action button enables. -->
{#if isRotateModalOpen}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4"
		onclick={(e) => { if (!isRotating && e.target === e.currentTarget) cancelRotate(); }}
		transition:fade={{ duration: 200 }}
	>
		<div class="w-full max-w-md rounded-2xl border border-rose-500/30 bg-zinc-950 p-6 shadow-2xl">
			<div class="flex items-start gap-3 mb-4">
				<div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-rose-500/15 text-rose-400">
					<AlertTriangle class="h-5 w-5" />
				</div>
				<div>
					<h3 class="text-base font-bold text-white">Rotate Local Certificate Authority</h3>
					<p class="mt-1 text-[12px] text-zinc-400 leading-relaxed">
						This is a <span class="font-semibold text-rose-300">destructive, irreversible</span>
						action. Read carefully before continuing.
					</p>
				</div>
			</div>

			<div class="rounded-xl border border-rose-500/20 bg-rose-500/[0.03] p-4 mb-4">
				<p class="text-[12px] font-semibold text-rose-200 mb-2">What happens when you rotate</p>
				<ul class="space-y-1.5 text-[11px] text-zinc-300 leading-relaxed">
					<li class="flex gap-2">
						<span class="text-rose-400 shrink-0">·</span>
						<span>The current CA is replaced by a brand-new one.</span>
					</li>
					<li class="flex gap-2">
						<span class="text-rose-400 shrink-0">·</span>
						<span><strong class="text-white">Every device</strong> (phone, laptop, tablet) that previously trusted PowerLab will see "Not Secure" warnings until you re-install the new CA on each one.</span>
					</li>
					<li class="flex gap-2">
						<span class="text-rose-400 shrink-0">·</span>
						<span>HSTS is cleared so HTTP becomes reachable again as a recovery path.</span>
					</li>
					<li class="flex gap-2">
						<span class="text-rose-400 shrink-0">·</span>
						<span>The previous CA is preserved as <code class="text-amber-300">ca.crt.previous</code> on the server (audit trail; can be restored manually if rotation was a mistake).</span>
					</li>
				</ul>
			</div>

			<div class="rounded-xl border border-amber-500/20 bg-amber-500/[0.03] p-4 mb-4">
				<p class="text-[12px] font-semibold text-amber-200 mb-1">When should you rotate?</p>
				<p class="text-[11px] text-zinc-400 leading-relaxed">
					Only if your CA private key was exposed (e.g., backup leak, screen-share with key visible) or if you're handing the panel to a different operator. Routine maintenance does <span class="text-white">NOT</span> require rotation — leaf certs auto-renew under the same CA.
				</p>
			</div>

			<label class="block mb-4">
				<span class="text-[11px] font-semibold text-zinc-300 mb-1.5 block">
					Type <code class="text-rose-300 bg-rose-500/10 px-1.5 py-0.5 rounded">ROTATE</code> to confirm
				</span>
				<input
					type="text"
					bind:value={rotateConfirmPhrase}
					autocomplete="off"
					autocorrect="off"
					autocapitalize="characters"
					spellcheck="false"
					class="w-full rounded-lg border border-white/10 bg-white/[0.03] px-3 py-2 text-sm text-white placeholder-zinc-600 outline-none focus:border-rose-500/40 font-mono"
					placeholder="ROTATE"
					disabled={isRotating}
				/>
			</label>

			<div class="flex items-center justify-end gap-2">
				<button
					type="button"
					onclick={cancelRotate}
					disabled={isRotating}
					class="rounded-lg border border-white/10 bg-white/[0.03] px-4 py-2 text-[12px] font-bold text-zinc-300 hover:bg-white/[0.06] hover:text-white disabled:opacity-40"
				>
					Cancel
				</button>
				<button
					type="button"
					onclick={executeRotateCA}
					disabled={rotateConfirmPhrase !== 'ROTATE' || isRotating}
					class="rounded-lg bg-rose-500 px-4 py-2 text-[12px] font-bold text-zinc-950 hover:bg-rose-400 disabled:bg-zinc-800 disabled:text-zinc-500 disabled:cursor-not-allowed transition-colors"
				>
					{#if isRotating}
						<RefreshCw class="h-3 w-3 mr-1.5 animate-spin inline" />
						Rotating…
					{:else}
						Rotate now
					{/if}
				</button>
			</div>
		</div>
	</div>
{/if}

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
