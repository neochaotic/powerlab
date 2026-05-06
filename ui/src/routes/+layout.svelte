<script lang="ts">
	import '../app.css';
	import Sidebar from '$lib/components/layout/Sidebar.svelte';
	import LoginScreen from '$lib/components/auth/LoginScreen.svelte';
	import SetupWizard from '$lib/components/auth/SetupWizard.svelte';
	import ToastContainer from '$lib/components/ui/Toast.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { ui } from '$lib/stores/ui.svelte';
	import { onMount } from 'svelte';
	import { fade, slide, scale } from 'svelte/transition';
	import { Button } from '$lib/components/ui/button';
	import { Download, X } from 'lucide-svelte';
	import { page } from '$app/stores';

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

		// Register Service Worker for PWA
		if ('serviceWorker' in navigator) {
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
		return () => window.removeEventListener('keydown', handleKeydown);
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
	<title>PowerLab</title>
	<meta name="description" content="High-performance headless OS management panel" />
	<link rel="preconnect" href="https://fonts.googleapis.com" />
	<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous" />
	<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&display=swap" rel="stylesheet" />
</svelte:head>

<ToastContainer />

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
					<h4 class="text-sm font-bold text-white leading-tight">Install PowerLab</h4>
					<p class="text-[11px] text-zinc-400 truncate">Add to your home screen for the full experience</p>
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
						Install
					</Button>
				</div>
			</div>
		</div>
	{/if}
</div>
