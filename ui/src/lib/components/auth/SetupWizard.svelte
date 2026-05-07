<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte';
	import { User, Lock, Rocket, ShieldCheck, ArrowRight, Loader2, Server } from 'lucide-svelte';
	import { t } from '$lib/i18n/index.svelte';
	import { fade, fly, scale } from 'svelte/transition';
	import { onMount } from 'svelte';

	let username = $state('admin');
	let password = $state('');
	let confirmPassword = $state('');
	let step = $state(1);
	let loading = $state(false);
	let error = $state('');

	async function handleSetup() {
		// Re-entry guard: a fast double-click or Enter-Enter could otherwise
		// submit twice. The 2nd register would 400 (user exists) and overwrite
		// the 1st success with a misleading "Falha ao inicializar" error.
		if (loading) return;

		if (password !== confirmPassword) {
			error = t('error.passMismatch');
			return;
		}
		if (password.length < 5) {
			error = t('error.passTooShort');
			return;
		}

		loading = true;
		error = '';

		const success = await auth.register(username, password);
		if (!success) {
			// Most common cause: register succeeded but a stale state in the auth
			// store caused login() to be called with wrong arguments. If we got
			// here AND auth.isAuthenticated is now true, the registration actually
			// worked and we just need to reload to pick up the session.
			if (auth.isAuthenticated) {
				window.location.reload();
				return;
			}
			error = t('error.setupFailed');
		}
		loading = false;
	}
</script>

<div class="relative h-full w-full bg-[#050505] flex items-center justify-center overflow-hidden font-sans antialiased">
	<!-- Background Effects -->
	<div class="absolute inset-0 pointer-events-none">
		<div class="absolute top-[-20%] right-[-10%] w-[70%] h-[70%] rounded-full bg-emerald-500/5 blur-[120px] animate-pulse"></div>
		<div class="absolute bottom-[-20%] left-[-10%] w-[60%] h-[60%] rounded-full bg-blue-600/5 blur-[100px]"></div>
		<!-- Grid Pattern -->
		<div class="absolute inset-0 bg-[url('https://grainy-gradients.vercel.app/noise.svg')] opacity-20 mix-blend-overlay"></div>
	</div>

	<div class="z-10 w-full max-w-[440px] px-6">
		{#if step === 1}
			<div class="text-center" in:fly={{ y: 20, duration: 800 }}>
				<div class="mb-10 inline-flex p-4 rounded-3xl bg-zinc-900 border border-white/5 shadow-2xl relative group">
					<div class="absolute inset-0 bg-emerald-500/20 blur-2xl group-hover:bg-emerald-500/30 transition-all duration-700 rounded-full"></div>
					<Server class="h-12 w-12 text-emerald-500 relative z-10" strokeWidth={1.5} />
				</div>
				
				<h1 class="text-4xl font-bold tracking-tight text-white mb-4">{t('setup.welcomeTitle')}</h1>
				<p class="text-zinc-400 text-lg mb-10 leading-relaxed">
					{t('setup.welcomeDesc')}
				</p>

				<button 
					onclick={() => step = 2}
					class="group relative w-full h-16 rounded-2xl bg-white text-black font-bold text-lg hover:scale-[1.02] active:scale-[0.98] transition-all flex items-center justify-center gap-3"
				>
					{t('setup.startBtn')}
					<ArrowRight class="h-5 w-5 group-hover:translate-x-1 transition-transform" />
				</button>
			</div>
		{:else if step === 2}
			<div class="space-y-8" in:fly={{ x: 20, duration: 600 }}>
				<div class="text-center mb-8">
					<div class="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-emerald-500/10 border border-emerald-500/20 text-[10px] font-bold text-emerald-500 uppercase tracking-widest mb-4">
						<ShieldCheck class="h-3 w-3" />
						{t('setup.securityConfig')}
					</div>
					<h2 class="text-2xl font-bold text-white tracking-tight">{t('setup.createAdmin')}</h2>
				</div>

				<form onsubmit={(e) => { e.preventDefault(); handleSetup(); }} class="space-y-5">
					<div class="space-y-2">
						<label class="text-[10px] font-bold text-zinc-500 uppercase tracking-widest ml-1" for="username">{t('setup.systemUser')}</label>
						<div class="relative group">
							<User class="absolute left-4 top-1/2 -translate-y-1/2 h-4 w-4 text-zinc-600 group-focus-within:text-emerald-500 transition-colors" />
							<input 
								id="username"
								type="text" 
								bind:value={username}
								class="w-full h-14 bg-zinc-900/50 border border-white/5 rounded-2xl pl-12 pr-4 text-sm text-white outline-none focus:border-emerald-500/50 focus:bg-zinc-900 transition-all"
								placeholder="admin"
							/>
						</div>
					</div>

					<div class="space-y-2">
						<label class="text-[10px] font-bold text-zinc-500 uppercase tracking-widest ml-1" for="password">{t('setup.masterPassword')}</label>
						<div class="relative group">
							<Lock class="absolute left-4 top-1/2 -translate-y-1/2 h-4 w-4 text-zinc-600 group-focus-within:text-emerald-500 transition-colors" />
							<input 
								id="password"
								type="password" 
								bind:value={password}
								class="w-full h-14 bg-zinc-900/50 border border-white/5 rounded-2xl pl-12 pr-4 text-sm text-white outline-none focus:border-emerald-500/50 focus:bg-zinc-900 transition-all"
								placeholder="••••••••"
							/>
						</div>
					</div>

					<div class="space-y-2">
						<label class="text-[10px] font-bold text-zinc-500 uppercase tracking-widest ml-1" for="confirm">{t('setup.confirmPassword')}</label>
						<div class="relative group">
							<Lock class="absolute left-4 top-1/2 -translate-y-1/2 h-4 w-4 text-zinc-600 group-focus-within:text-emerald-500 transition-colors" />
							<input 
								id="confirm"
								type="password" 
								bind:value={confirmPassword}
								class="w-full h-14 bg-zinc-900/50 border border-white/5 rounded-2xl pl-12 pr-4 text-sm text-white outline-none focus:border-emerald-500/50 focus:bg-zinc-900 transition-all"
								placeholder="••••••••"
							/>
						</div>
					</div>

					{#if error}
						<div class="p-4 rounded-xl bg-red-500/10 border border-red-500/20 text-red-500 text-xs font-medium animate-shake" in:fade>
							{error}
						</div>
					{/if}

					<button 
						type="submit"
						disabled={loading || !password || !confirmPassword}
						class="w-full h-16 rounded-2xl bg-emerald-500 text-black font-bold text-lg hover:bg-emerald-400 active:scale-[0.98] transition-all disabled:opacity-50 disabled:bg-zinc-800 disabled:text-zinc-600 flex items-center justify-center gap-2 mt-8 shadow-[0_0_30px_rgba(16,185,129,0.15)]"
					>
						{#if loading}
							<Loader2 class="h-5 w-5 animate-spin" /> {t('setup.initializing')}
						{:else}
							<Rocket class="h-5 w-5" /> {t('setup.finishBtn')}
						{/if}
					</button>

					<p class="text-center text-[10px] text-zinc-600 font-medium leading-relaxed px-4 pt-4">
						{t('setup.note')}
					</p>
				</form>
			</div>
		{/if}
	</div>

	<!-- Bottom Brand -->
	<div class="absolute bottom-10 left-10 flex items-center gap-3" in:fade={{ delay: 1000 }}>
		<div class="h-8 w-8 rounded-xl bg-zinc-900 border border-white/5 flex items-center justify-center">
			<div class="h-2 w-2 bg-emerald-500 rounded-full shadow-[0_0_10px_#10b981]"></div>
		</div>
		<div>
			<p class="text-[10px] font-bold text-white uppercase tracking-[0.2em]">PowerLab Engine</p>
			<p class="text-[9px] text-zinc-600 font-medium">Build 2026.05.v1</p>
		</div>
	</div>
</div>

<style>
	.animate-shake {
		animation: shake 0.4s cubic-bezier(.36,.07,.19,.97) both;
	}

	@keyframes shake {
		10%, 90% { transform: translate3d(-1px, 0, 0); }
		20%, 80% { transform: translate3d(2px, 0, 0); }
		30%, 50%, 70% { transform: translate3d(-4px, 0, 0); }
		40%, 60% { transform: translate3d(4px, 0, 0); }
	}
</style>
