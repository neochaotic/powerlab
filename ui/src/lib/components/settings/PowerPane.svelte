<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import {
		Activity,
		RefreshCw,
		RotateCw,
		Power,
		AlertTriangle,
		ShieldAlert,
		CircleDot
	} from 'lucide-svelte';
	import { cn } from '$lib/utils';
	import {
		listPowerLabServices,
		restartPowerLabService,
		rebootHost,
		shutdownHost,
		type ServiceState
	} from '$lib/api/power';

	// Settings → Power pane (#260).
	//
	// Lists the 6 PowerLab systemd units with their current
	// active/sub state + offers per-service Restart + host-level
	// Reboot/Shutdown actions.
	//
	// Host-level ops gate behind a modal acknowledgement (matches the
	// backend's required {"confirm": true} body — defence in depth,
	// memory feedback_security_is_priority).
	//
	// Service list polls every 5s while the pane is mounted. Stops on
	// unmount; refcount-friendly thanks to the local interval handle.

	let services = $state<ServiceState[]>([]);
	let loading = $state(false);
	let error = $state<string | null>(null);
	let restartInFlight = $state<Record<string, boolean>>({});
	let pollHandle: ReturnType<typeof setInterval> | null = null;

	let showRebootModal = $state(false);
	let rebootAck = $state(false);
	let rebootInFlight = $state(false);

	let showShutdownModal = $state(false);
	let shutdownAck = $state(false);
	let shutdownInFlight = $state(false);

	async function load(): Promise<void> {
		loading = true;
		error = null;
		try {
			services = await listPowerLabServices();
		} catch (e) {
			const apiErr = e as { status?: number; message?: string };
			if (apiErr?.status !== 401) {
				error = apiErr?.message ?? String(e);
			}
		} finally {
			loading = false;
		}
	}

	async function doRestart(name: string): Promise<void> {
		restartInFlight = { ...restartInFlight, [name]: true };
		error = null;
		try {
			await restartPowerLabService(name);
			// Give systemd ~600ms to flip the state, then refresh.
			setTimeout(load, 600);
		} catch (e) {
			const apiErr = e as { message?: string };
			error = apiErr?.message ?? String(e);
		} finally {
			restartInFlight = { ...restartInFlight, [name]: false };
		}
	}

	function openRebootModal(): void {
		rebootAck = false;
		showRebootModal = true;
	}

	async function confirmReboot(): Promise<void> {
		if (!rebootAck) return;
		rebootInFlight = true;
		error = null;
		try {
			await rebootHost();
			showRebootModal = false;
			rebootAck = false;
		} catch (e) {
			const apiErr = e as { message?: string };
			error = apiErr?.message ?? String(e);
		} finally {
			rebootInFlight = false;
		}
	}

	function openShutdownModal(): void {
		shutdownAck = false;
		showShutdownModal = true;
	}

	async function confirmShutdown(): Promise<void> {
		if (!shutdownAck) return;
		shutdownInFlight = true;
		error = null;
		try {
			await shutdownHost();
			showShutdownModal = false;
			shutdownAck = false;
		} catch (e) {
			const apiErr = e as { message?: string };
			error = apiErr?.message ?? String(e);
		} finally {
			shutdownInFlight = false;
		}
	}

	function badgeClasses(state: string): string {
		switch (state) {
			case 'active':
				return 'border-emerald-500/30 bg-emerald-500/[0.08] text-emerald-300';
			case 'inactive':
				return 'border-zinc-500/30 bg-zinc-500/[0.08] text-zinc-400';
			case 'failed':
				return 'border-red-500/30 bg-red-500/[0.08] text-red-300';
			case 'activating':
			case 'deactivating':
				return 'border-yellow-500/30 bg-yellow-500/[0.08] text-yellow-300';
			default:
				return 'border-zinc-500/30 bg-zinc-500/[0.08] text-zinc-400';
		}
	}

	onMount(() => {
		void load();
		pollHandle = setInterval(load, 5000);
	});

	onDestroy(() => {
		if (pollHandle) clearInterval(pollHandle);
	});
</script>

<div class="space-y-6" data-testid="power-pane">
	<header class="flex items-start justify-between gap-4">
		<div>
			<h2 class="text-2xl font-bold text-white">Power</h2>
			<p class="mt-1 text-sm text-zinc-400">
				PowerLab service control and host power actions. Restart a single
				service if it's misbehaving, or reboot/shutdown the entire host
				when you need to take the box offline.
			</p>
		</div>
		<button
			onclick={load}
			disabled={loading}
			class="flex h-9 items-center gap-2 rounded-xl border border-white/10 bg-white/[0.03] px-3 text-xs text-zinc-300 transition-colors hover:border-white/20 hover:text-white disabled:opacity-50"
			data-testid="power-refresh"
		>
			<RefreshCw class={cn('h-3.5 w-3.5', loading && 'animate-spin')} />
			Refresh
		</button>
	</header>

	{#if error}
		<div class="rounded-2xl border border-red-500/20 bg-red-500/[0.05] p-4 text-sm text-red-400">
			{error}
		</div>
	{/if}

	<!-- Services list -->
	<section class="space-y-2">
		<h3 class="text-xs font-bold uppercase tracking-widest text-zinc-500">
			PowerLab Services
		</h3>
		{#if loading && services.length === 0}
			<div class="rounded-2xl border border-white/[0.06] bg-white/[0.02] p-6 text-center text-sm text-zinc-500">
				Loading…
			</div>
		{:else if services.length === 0}
			<div class="rounded-2xl border border-white/[0.06] bg-white/[0.02] p-6 text-center text-sm text-zinc-500">
				No services reported. Backend may be unreachable.
			</div>
		{:else}
			{#each services as svc (svc.name)}
				<div
					class="flex items-center justify-between gap-3 rounded-2xl border border-white/[0.06] bg-white/[0.02] p-4"
					data-testid="service-row-{svc.name}"
				>
					<div class="flex flex-1 items-center gap-3 min-w-0">
						<CircleDot
							class={cn(
								'h-3 w-3 shrink-0',
								svc.active_state === 'active'
									? 'text-emerald-400'
									: svc.active_state === 'failed'
									? 'text-red-400'
									: 'text-zinc-500'
							)}
						/>
						<div class="min-w-0 flex-1">
							<div class="flex items-center gap-2 flex-wrap">
								<span class="text-sm font-mono text-zinc-200">{svc.name}</span>
								<span
									class={cn(
										'rounded-full border px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider',
										badgeClasses(svc.active_state)
									)}
									data-testid="service-state-badge"
								>
									{svc.active_state}
								</span>
							</div>
							<div class="mt-1 flex items-center gap-3 text-[10px] text-zinc-500">
								{#if svc.sub_state}
									<span class="font-mono">{svc.sub_state}</span>
								{/if}
								{#if svc.pid}
									<span class="font-mono">pid {svc.pid}</span>
								{/if}
							</div>
						</div>
					</div>
					<button
						onclick={() => doRestart(svc.name)}
						disabled={restartInFlight[svc.name]}
						class="flex h-8 items-center gap-1.5 rounded-md border border-blue-500/20 bg-blue-500/[0.05] px-3 text-[11px] font-medium text-blue-300 transition-colors hover:border-blue-500/40 hover:bg-blue-500/[0.10] disabled:cursor-not-allowed disabled:opacity-50"
						data-testid="service-restart-btn"
					>
						<RotateCw class={cn('h-3 w-3', restartInFlight[svc.name] && 'animate-spin')} />
						Restart
					</button>
				</div>
			{/each}
		{/if}
	</section>

	<!-- Host actions -->
	<section class="space-y-2">
		<h3 class="text-xs font-bold uppercase tracking-widest text-zinc-500">
			Host
		</h3>
		<div class="flex flex-col gap-2 sm:flex-row">
			<button
				onclick={openRebootModal}
				class="flex flex-1 items-center justify-center gap-2 rounded-2xl border border-yellow-500/20 bg-yellow-500/[0.05] p-4 text-sm font-medium text-yellow-200 transition-colors hover:border-yellow-500/40 hover:bg-yellow-500/[0.10]"
				data-testid="host-reboot-btn"
			>
				<Activity class="h-4 w-4" />
				Reboot host
			</button>
			<button
				onclick={openShutdownModal}
				class="flex flex-1 items-center justify-center gap-2 rounded-2xl border border-red-500/20 bg-red-500/[0.05] p-4 text-sm font-medium text-red-300 transition-colors hover:border-red-500/40 hover:bg-red-500/[0.10]"
				data-testid="host-shutdown-btn"
			>
				<Power class="h-4 w-4" />
				Shut down host
			</button>
		</div>
	</section>
</div>

<!-- Reboot modal -->
{#if showRebootModal}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
		role="dialog"
		aria-modal="true"
		aria-labelledby="reboot-modal-title"
	>
		<div
			class="w-full max-w-lg rounded-2xl border border-yellow-500/30 bg-zinc-950 p-6 shadow-2xl"
			data-testid="host-reboot-modal"
		>
			<div class="flex items-start gap-3">
				<AlertTriangle class="mt-0.5 h-6 w-6 shrink-0 text-yellow-400" />
				<div class="flex-1">
					<h3 id="reboot-modal-title" class="text-lg font-semibold text-white">
						Reboot host?
					</h3>
					<p class="mt-3 text-sm leading-relaxed text-zinc-300">
						The host will run <code class="text-zinc-400">systemctl reboot</code>.
						All running PowerLab services + every Docker container managed by
						PowerLab will stop and restart with the box. Expect ~30-60s of
						downtime before the UI becomes reachable again.
					</p>

					<label class="mt-5 flex items-start gap-2 cursor-pointer">
						<input
							type="checkbox"
							bind:checked={rebootAck}
							class="mt-0.5 h-4 w-4"
							data-testid="host-reboot-ack"
						/>
						<span class="text-xs leading-relaxed text-zinc-400">
							I understand this will reboot the entire host, not just
							PowerLab.
						</span>
					</label>
				</div>
			</div>

			<div class="mt-6 flex justify-end gap-2">
				<button
					onclick={() => (showRebootModal = false)}
					class="rounded-xl border border-white/10 bg-white/[0.03] px-4 py-2 text-sm text-zinc-300 transition-colors hover:border-white/20 hover:text-white"
					data-testid="host-reboot-cancel"
				>
					Cancel
				</button>
				<button
					onclick={confirmReboot}
					disabled={!rebootAck || rebootInFlight}
					class="rounded-xl border border-yellow-500/40 bg-yellow-500/20 px-4 py-2 text-sm font-medium text-yellow-200 transition-colors hover:border-yellow-500/60 hover:bg-yellow-500/30 disabled:cursor-not-allowed disabled:opacity-50"
					data-testid="host-reboot-confirm"
				>
					{rebootInFlight ? 'Rebooting…' : 'Reboot host'}
				</button>
			</div>
		</div>
	</div>
{/if}

<!-- Shutdown modal -->
{#if showShutdownModal}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
		role="dialog"
		aria-modal="true"
		aria-labelledby="shutdown-modal-title"
	>
		<div
			class="w-full max-w-lg rounded-2xl border border-red-500/30 bg-zinc-950 p-6 shadow-2xl"
			data-testid="host-shutdown-modal"
		>
			<div class="flex items-start gap-3">
				<ShieldAlert class="mt-0.5 h-6 w-6 shrink-0 text-red-400" />
				<div class="flex-1">
					<h3 id="shutdown-modal-title" class="text-lg font-semibold text-white">
						Shut down host?
					</h3>
					<p class="mt-3 text-sm leading-relaxed text-zinc-300">
						The host will run <code class="text-zinc-400">systemctl poweroff</code>.
						The box will fully power off — bringing it back online requires
						physical or out-of-band (IPMI/iLO/Wake-on-LAN) access.
					</p>

					<label class="mt-5 flex items-start gap-2 cursor-pointer">
						<input
							type="checkbox"
							bind:checked={shutdownAck}
							class="mt-0.5 h-4 w-4"
							data-testid="host-shutdown-ack"
						/>
						<span class="text-xs leading-relaxed text-zinc-400">
							I understand I will need physical or out-of-band access to
							bring this host back online.
						</span>
					</label>
				</div>
			</div>

			<div class="mt-6 flex justify-end gap-2">
				<button
					onclick={() => (showShutdownModal = false)}
					class="rounded-xl border border-white/10 bg-white/[0.03] px-4 py-2 text-sm text-zinc-300 transition-colors hover:border-white/20 hover:text-white"
					data-testid="host-shutdown-cancel"
				>
					Cancel
				</button>
				<button
					onclick={confirmShutdown}
					disabled={!shutdownAck || shutdownInFlight}
					class="rounded-xl border border-red-500/40 bg-red-500/20 px-4 py-2 text-sm font-medium text-red-300 transition-colors hover:border-red-500/60 hover:bg-red-500/30 disabled:cursor-not-allowed disabled:opacity-50"
					data-testid="host-shutdown-confirm"
				>
					{shutdownInFlight ? 'Shutting down…' : 'Shut down host'}
				</button>
			</div>
		</div>
	</div>
{/if}
