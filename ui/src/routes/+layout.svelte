<script lang="ts">
	import '../app.css';
	import Sidebar from '$lib/components/layout/Sidebar.svelte';
	import LoginScreen from '$lib/components/auth/LoginScreen.svelte';
	import SetupWizard from '$lib/components/auth/SetupWizard.svelte';
	import ToastContainer from '$lib/components/ui/Toast.svelte';
	import TrustStateChecker from '$lib/components/security/TrustStateChecker.svelte';
	import UpgradeProgressOverlay from '$lib/components/system/UpgradeProgressOverlay.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { ui } from '$lib/stores/ui.svelte';
	import { versionHandshake } from '$lib/stores/versionHandshake.svelte';
	import { api } from '$lib/api/client';
	import { captureFrontendError, createErrorReporter } from '$lib/utils/error-reporter';
	import { onMount } from 'svelte';
	import { fade, slide, scale } from 'svelte/transition';
	import { Button } from '$lib/components/ui/button';
	import { Download, X, RefreshCw, AlertTriangle } from 'lucide-svelte';
	import { page } from '$app/stores';
	import { t } from '$lib/i18n/index.svelte';

	let { children } = $props();

	let deferredPrompt = $state<any>(null);
	let showInstallPrompt = $state(false);

	onMount(() => {
		// Probe whether the host has any registered user. The SetupWizard
		// is shown only on the very first run (no user in the DB yet) on
		// platforms without working OS auth (Linux today, until PAM lands).
		// On macOS the first dscl-validated login also returns initialized=true.
		auth.checkStatus();
		auth.checkSession();

		// Version handshake — fail fast if the JS bundle in this browser
		// is older than the backend that just answered. Without this,
		// stale UIs silently send the wrong shape of request and the
		// user just sees features "broken" with no diagnostic.
		versionHandshake.check();

		// Service Worker — register only on production builds (PROD), no
		// fetch interception. The SW (ui/static/sw.js) just satisfies
		// PWA install criteria so the browser offers "Add to Home
		// Screen". Skipped under vite dev because the SW + dev runtime
		// race condition surfaces "Failed to fetch" warnings without
		// any benefit — the install prompt only matters on production
		// anyway. See docs/decisions/0005-pwa-scaffolding-no-cache-yet.md.
		if ('serviceWorker' in navigator && import.meta.env.PROD) {
			navigator.serviceWorker.register('/sw.js').catch(console.error);
		}

		window.addEventListener('beforeinstallprompt', (e) => {
			e.preventDefault();
			deferredPrompt = e;
			showInstallPrompt = true;
		});

		window.addEventListener('appinstalled', () => {
			showInstallPrompt = false;
			deferredPrompt = null;
		});

		const handleKeydown = (e: KeyboardEvent) => {
			const isMac = navigator.platform.toUpperCase().indexOf('MAC') >= 0;
			const cmd = isMac ? e.metaKey : e.ctrlKey;

			if (cmd && e.key.toLowerCase() === 'k') {
				e.preventDefault();
				ui.openSearch();
			}

			if (cmd && e.key.toLowerCase() === 'g') {
				e.preventDefault();
				ui.isTerminalOpen = true;
			}

			if (cmd && e.key.toLowerCase() === 'q') {
				e.preventDefault();
				auth.logout();
			}
		};

		window.addEventListener('keydown', handleKeydown);

		// Frontend error capture → /v1/audit/frontend-error. The
		// reporter handles noise filter + dedupe + DoS cap so the
		// listener stays trivial. Audit failures swallow inside
		// reporter.report so a broken endpoint can never re-throw
		// into a user-visible "double crash" loop.
		const errorReporter = createErrorReporter(async (body) => {
			return api.post('/v1/audit/frontend-error', body);
		});
		const onErr = (e: ErrorEvent) => {
			void captureFrontendError(e, errorReporter);
		};
		const onRej = (e: PromiseRejectionEvent) => {
			void captureFrontendError(e, errorReporter);
		};
		window.addEventListener('error', onErr);
		window.addEventListener('unhandledrejection', onRej);

		return () => {
			window.removeEventListener('keydown', handleKeydown);
			window.removeEventListener('error', onErr);
			window.removeEventListener('unhandledrejection', onRej);
		};
	});

	async function installApp() {
		if (!deferredPrompt) return;
		deferredPrompt.prompt();
		const { outcome } = await deferredPrompt.userChoice;
		if (outcome === 'accepted') {
			showInstallPrompt = false;
		}
		deferredPrompt = null;
	}
</script>

<svelte:head>
	<title>{t('app.name')}</title>
	<meta name="description" content={t('app.tagline')} />
	<link rel="preconnect" href="https://fonts.googleapis.com" />
	<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous" />
	<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&display=swap" rel="stylesheet" />
</svelte:head>

<ToastContainer />
<UpgradeProgressOverlay />
<TrustStateChecker />

<!-- Root Container — same ambient wallpaper as the login screen.
	 Sidebar + Launchpad are translucent so the glow shows through.
	 Native app routes (Dashboard, Files, Apps, Settings, etc.) keep their
	 own opaque backgrounds — content first, atmosphere only at the OS chrome. -->
<div class="relative h-screen w-full overflow-hidden bg-[#08080a] text-zinc-50 font-sans antialiased selection:bg-zinc-800 selection:text-white">
	<!-- Wallpaper: soft Apple-style ambient gradient. Emerald -> mint -> teal at
		 the top, a subtle teal pull at bottom-right. Visible through the glass
		 sidebar and through every page (pages are transparent over this). -->
	<div class="pointer-events-none fixed inset-0 z-0">
		<div class="absolute -top-[35vh] left-1/2 h-[90vh] w-[90vh] -translate-x-1/2 rounded-full bg-gradient-to-br from-emerald-300/[0.10] via-emerald-400/[0.06] to-teal-400/[0.04] blur-[160px]"></div>
		<div class="absolute bottom-[-20vh] right-[-10vh] h-[50vh] w-[50vh] rounded-full bg-gradient-to-tl from-teal-400/[0.06] to-emerald-400/[0.02] blur-[140px]"></div>
		<div class="absolute top-[40vh] left-[-20vh] h-[40vh] w-[40vh] rounded-full bg-cyan-400/[0.03] blur-[120px]"></div>
	</div>
	<!--
		Auth gating, first-run aware:
		  · No user registered yet → SetupWizard (one-shot bcrypt registration)
		  · User exists, not signed in → LoginScreen (OS-auth on macOS;
		    bcrypt fallback on Linux until PAM lands)
		  · Signed in → main OS interface
		The SetupWizard is the safety net for hosts where native OS auth
		is not yet implemented (Linux). On macOS it is rarely seen because
		the first successful dscl login auto-registers the DB record.
	-->
	<div class="relative z-10 h-full w-full">
		{#if !auth.isInitialized && $page.url.pathname !== '/product'}
			<div class="h-full w-full" in:fade={{ duration: 400 }}>
				<SetupWizard />
			</div>
		{:else if !auth.isAuthenticated && $page.url.pathname !== '/product'}
			<div class="h-full w-full" in:fade={{ duration: 400 }}>
				<LoginScreen />
			</div>
		{:else}
			<!-- Main OS Interface -->
			<div class="flex h-full w-full overflow-hidden" in:fade={{ duration: 600 }}>
				<!-- Glassmorphism Sidebar -->
				<Sidebar />

				<!-- Main Content Area -->
				<main class="flex-1 overflow-hidden relative">
					{@render children()}
				</main>
			</div>
		{/if}
	</div>

	{#if versionHandshake.mismatch && !versionHandshake.dismissed}
		<!-- Dismissable banner. The JS bundle in this browser does not
			 match the backend. First attempts: offer a cache-busted
			 force-reload (?powerlab_v=<ts>) since plain reload often
			 hits a cached bundle. After 2 attempts that didn't fix it,
			 the message changes to "the deployed bundle on the server
			 is wrong" — at that point reloading more won't help. -->
		<div
			data-testid="version-mismatch-banner"
			class="fixed top-0 left-0 right-0 z-[200] border-b border-amber-500/30 bg-amber-500/95 px-6 py-3 text-zinc-950 shadow-lg backdrop-blur-xl"
			transition:slide={{ axis: 'y' }}
		>
			<div class="mx-auto flex max-w-4xl items-center justify-between gap-4">
				<div class="flex items-center gap-3">
					<AlertTriangle class="h-5 w-5 shrink-0" />
					<div class="text-sm font-semibold leading-tight">
						{#if versionHandshake.persistentFailure}
							<div>{t('app.stalebundle.persistent.title')}</div>
							<div class="text-[11px] font-medium opacity-80">
								{t('app.stalebundle.persistent.help')}
							</div>
						{:else}
							<div>{t('app.stalebundle.title')}</div>
							<div class="text-[11px] font-medium opacity-80">
								{t('app.uiVersion')}: v{versionHandshake.uiVersion} · {t('app.serverVersion')}: v{versionHandshake.backendVersion}
							</div>
						{/if}
					</div>
				</div>
				<div class="flex items-center gap-2">
					{#if !versionHandshake.persistentFailure}
						<button
							data-testid="version-mismatch-force-reload"
							class="flex items-center gap-2 rounded-xl bg-zinc-950 px-4 py-2 text-xs font-bold text-amber-300 hover:bg-zinc-800 transition-colors"
							onclick={() => versionHandshake.forceReload()}
						>
							<RefreshCw class="h-3.5 w-3.5" />
							{t('app.stalebundle.action.forceReload')}
						</button>
					{/if}
					<button
						data-testid="version-mismatch-dismiss"
						aria-label={t('action.close')}
						title={t('action.close')}
						class="flex h-8 w-8 items-center justify-center rounded-lg text-zinc-950/70 hover:bg-zinc-950/10 hover:text-zinc-950 transition-colors"
						onclick={() => versionHandshake.dismiss()}
					>
						<X class="h-4 w-4" />
					</button>
				</div>
			</div>
		</div>
	{/if}

	{#if showInstallPrompt}
		<div 
			class="fixed bottom-6 left-1/2 z-[100] -translate-x-1/2 w-[calc(100%-3rem)] max-w-md"
			transition:slide={{ axis: 'y' }}
		>
			<div class="flex items-center gap-4 rounded-2xl border border-white/10 bg-zinc-900/90 p-4 shadow-2xl backdrop-blur-xl">
				<div class="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-emerald-500 text-zinc-950 shadow-[0_0_20px_rgba(16,185,129,0.3)]">
					<Download class="h-6 w-6" />
				</div>
				<div class="flex-1 min-w-0">
					<h4 class="text-sm font-bold text-white leading-tight">{t('pwa.installTitle')}</h4>
					<p class="text-[11px] text-zinc-400 truncate">{t('pwa.installDesc')}</p>
				</div>
				<div class="flex items-center gap-2">
					<button 
						class="rounded-lg p-2 text-zinc-500 hover:bg-white/5 hover:text-white transition-colors"
						onclick={() => showInstallPrompt = false}
					>
						<X class="h-4 w-4" />
					</button>
					<Button 
						size="sm" 
						class="rounded-xl bg-white text-zinc-950 font-bold hover:bg-emerald-500 hover:text-zinc-950 transition-all"
						onclick={installApp}
					>
						{t('action.install')}
					</Button>
				</div>
			</div>
		</div>
	{/if}
</div>
