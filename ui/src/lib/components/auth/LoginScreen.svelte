<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte';
	import { Lock, Eye, EyeOff, Loader2, ArrowRight } from 'lucide-svelte';
	import { fade, scale, slide } from 'svelte/transition';
	import { onMount, onDestroy } from 'svelte';

	let username = $state('');
	let password = $state('');
	let errorKind = $state<null | 'invalid' | 'offline'>(null);
	let loading = $state(false);
	let showPassword = $state(false);
	let time = $state('');
	let date = $state('');
	let interval: ReturnType<typeof setInterval>;

	function updateTime() {
		const now = new Date();
		time = now.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: false });
		date = now.toLocaleDateString('en-US', { weekday: 'long', month: 'long', day: 'numeric' });
	}

	onMount(() => {
		updateTime();
		// 30s is enough — minute-precision UI doesn't need second-level updates
		interval = setInterval(updateTime, 30_000);
	});
	onDestroy(() => clearInterval(interval));

	async function handleLogin() {
		if (loading || !username || !password) return;
		loading = true;
		errorKind = null;
		const result = await auth.login(username, password);
		if (result === 'ok') {
			// Layout will swap to the OS interface.
		} else {
			errorKind = result;
			if (result === 'invalid') password = '';
		}
		loading = false;
	}
</script>

<div class="relative h-full w-full overflow-hidden bg-[#08080a] font-sans antialiased">
	<!-- Subtle ambient lighting -->
	<div class="pointer-events-none absolute inset-0">
		<div class="absolute -top-[30vh] left-1/2 h-[80vh] w-[80vh] -translate-x-1/2 rounded-full bg-emerald-500/[0.06] blur-[140px]"></div>
		<div class="absolute bottom-[-20vh] right-[-10vh] h-[40vh] w-[40vh] rounded-full bg-blue-500/[0.04] blur-[120px]"></div>
	</div>

	<!-- Top: time + date -->
	<div class="absolute left-1/2 top-[14%] -translate-x-1/2 text-center" in:fade={{ delay: 200, duration: 800 }}>
		<h1 class="font-['Outfit'] text-[88px] font-extralight leading-none tracking-tight text-white/95 tabular-nums">
			{time}
		</h1>
		<p class="mt-3 text-[10px] font-bold uppercase tracking-[0.5em] text-zinc-600">
			{date}
		</p>
	</div>

	<!-- Center: login card -->
	<div class="relative z-10 flex h-full items-center justify-center px-6">
		<div class="w-full max-w-[360px]" in:scale={{ delay: 400, duration: 600, start: 0.96 }}>

			<!-- Brand mark -->
			<div class="mb-10 text-center">
				<div class="mx-auto mb-5 flex h-14 w-14 items-center justify-center rounded-2xl border border-white/[0.06] bg-white/[0.02] shadow-[inset_0_1px_0_0_rgba(255,255,255,0.04)]">
					<span class="font-['Outfit'] text-2xl font-black leading-none text-white">
						P<span class="text-emerald-500">.</span>
					</span>
				</div>
				<p class="text-[10px] font-bold uppercase tracking-[0.4em] text-zinc-500">PowerLab</p>
				<p class="mt-3 text-[11px] font-medium text-zinc-600">
					Sign in with your computer username and password
				</p>
			</div>

			<form
				onsubmit={(e) => { e.preventDefault(); handleLogin(); }}
				class="space-y-3"
			>
				<!-- svelte-ignore a11y_autofocus -->
				<input
					type="text"
					bind:value={username}
					placeholder="Username"
					autocomplete="username"
					autofocus
					class="h-12 w-full rounded-xl border border-white/[0.06] bg-white/[0.02] px-4 text-sm text-white placeholder:text-zinc-600 outline-none transition-all focus:border-emerald-500/30 focus:bg-white/[0.04]"
				/>

				<div class="relative">
					<Lock class="pointer-events-none absolute left-4 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-zinc-600" />
					<input
						type={showPassword ? 'text' : 'password'}
						bind:value={password}
						placeholder="Password"
						autocomplete="current-password"
						class="h-12 w-full rounded-xl border border-white/[0.06] bg-white/[0.02] pl-11 pr-11 text-sm text-white placeholder:text-zinc-600 outline-none transition-all focus:border-emerald-500/30 focus:bg-white/[0.04]"
					/>
					<button
						type="button"
						onclick={() => (showPassword = !showPassword)}
						aria-label={showPassword ? 'Hide password' : 'Show password'}
						class="absolute right-3 top-1/2 -translate-y-1/2 rounded-md p-1 text-zinc-600 transition-colors hover:text-zinc-300"
					>
						{#if showPassword}
							<EyeOff class="h-4 w-4" />
						{:else}
							<Eye class="h-4 w-4" />
						{/if}
					</button>
				</div>

				{#if errorKind === 'invalid'}
					<div in:slide={{ duration: 200 }} class="flex items-center justify-center gap-2 pt-1 text-[11px] font-medium text-red-400">
						<span class="h-1 w-1 rounded-full bg-red-500"></span>
						Invalid username or password
					</div>
				{:else if errorKind === 'offline'}
					<div in:slide={{ duration: 200 }} class="flex flex-col items-center gap-1 pt-1">
						<div class="flex items-center gap-2 text-[11px] font-medium text-amber-400">
							<span class="h-1 w-1 rounded-full bg-amber-500 animate-pulse"></span>
							Cannot reach the PowerLab backend
						</div>
						<p class="text-[10px] text-zinc-600">Check that the gateway is running, then try again</p>
					</div>
				{/if}

				<button
					type="submit"
					disabled={loading || !username || !password}
					class="group mt-3 flex h-12 w-full items-center justify-center gap-2 rounded-xl bg-white text-[12px] font-bold uppercase tracking-[0.15em] text-zinc-950 transition-all hover:bg-emerald-500 active:scale-[0.98] disabled:opacity-30 disabled:hover:bg-white"
				>
					{#if loading}
						<Loader2 class="h-4 w-4 animate-spin" />
					{:else}
						Sign in
						<ArrowRight class="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5" />
					{/if}
				</button>

				{#if import.meta.env.DEV}
					<button
						type="button"
						onclick={() => {
							const wasEnabled = localStorage.getItem('powerlab_dev_autologin') === 'true';
							if (wasEnabled) {
								localStorage.removeItem('powerlab_dev_autologin');
								localStorage.removeItem('powerlab_dev_creds');
								localStorage.removeItem('powerlab_token');
								localStorage.removeItem('powerlab_user');
							} else {
								localStorage.setItem('powerlab_dev_autologin', 'true');
							}
							window.location.reload();
						}}
						class="mt-6 w-full text-center text-[9px] font-bold uppercase tracking-[0.3em] text-emerald-500/40 transition-colors hover:text-emerald-500"
					>
						[ DEV ] {localStorage.getItem('powerlab_dev_autologin') === 'true' ? 'Disable auto-login' : 'Enable auto-login'}
					</button>
				{/if}
			</form>
		</div>
	</div>

	<!-- Footer -->
	<div class="absolute bottom-8 left-1/2 -translate-x-1/2 text-center" in:fade={{ delay: 800 }}>
		<p class="text-[9px] font-medium uppercase tracking-[0.4em] text-zinc-700">
			v0.1.0 · neochaotic
		</p>
	</div>
</div>

<style>
	:global(body) {
		overflow: hidden;
	}
</style>
