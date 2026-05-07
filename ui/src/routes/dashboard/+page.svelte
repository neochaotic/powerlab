<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import {
		Cpu, MemoryStick, Activity, HardDrive, Thermometer,
		Server, RefreshCw, PowerOff, Wifi, WifiOff
	} from 'lucide-svelte';
	import { useSystemStore } from '$lib/stores/system.svelte';
	import { putSystemState } from '$lib/api/system';
	import { toast } from '$lib/stores/toast.svelte';
	import RadialGauge from '$lib/components/dashboard/RadialGauge.svelte';
	import Sparkline from '$lib/components/dashboard/Sparkline.svelte';
	import MiniProgress from '$lib/components/dashboard/MiniProgress.svelte';
	import { cn } from '$lib/utils';
	import AppHeader from '$lib/components/layout/AppHeader.svelte';
	import { t } from '$lib/i18n/index.svelte';

	const store = useSystemStore();

	let confirmAction = $state<'restart' | 'off' | null>(null);
	let isActioning = $state(false);

	onMount(() => store.startPolling(1000));
	onDestroy(() => store.stopPolling());

	const u = $derived(store.utilization);

	const cpuStatus = $derived.by<'normal' | 'warning' | 'critical'>(() => {
		if (!u) return 'normal';
		if (u.cpu.percent > 95) return 'critical';
		if (u.cpu.percent > 80) return 'warning';
		return 'normal';
	});

	const ramStatus = $derived.by<'normal' | 'warning' | 'critical'>(() => {
		if (!u) return 'normal';
		if (u.mem.usedPercent > 95) return 'critical';
		if (u.mem.usedPercent > 85) return 'warning';
		return 'normal';
	});

	const cpuColor = $derived(
		cpuStatus === 'critical' ? '#ef4444' : cpuStatus === 'warning' ? '#f59e0b' : '#3b82f6'
	);
	const ramColor = $derived(
		ramStatus === 'critical' ? '#ef4444' : ramStatus === 'warning' ? '#f59e0b' : '#a855f7'
	);

	function formatBytes(bytes: number): string {
		if (bytes === 0) return '0 B';
		const units = ['B', 'KB', 'MB', 'GB', 'TB'];
		const i = Math.floor(Math.log(bytes) / Math.log(1024));
		return `${(bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0)} ${units[i]}`;
	}

	function diskHealth(pct: number): string {
		if (pct > 90) return t('dashboard.critical');
		if (pct > 75) return t('dashboard.warning');
		return t('dashboard.healthy');
	}

	const diskHealthColor: Record<string, string> = {
		[t('dashboard.healthy')]: 'text-emerald-500',
		[t('dashboard.warning')]: 'text-amber-500',
		[t('dashboard.critical')]: 'text-red-500'
	};

	const diskBarColor: Record<string, string> = {
		[t('dashboard.healthy')]: 'bg-emerald-500',
		[t('dashboard.warning')]: 'bg-amber-500',
		[t('dashboard.critical')]: 'bg-red-500'
	};

	async function performAction(action: 'restart' | 'off') {
		isActioning = true;
		confirmAction = null;
		try {
			await putSystemState(action);
			toast.success(action === 'restart' ? t('dashboard.isRestarting') : t('dashboard.isShuttingDown'));
		} catch {
			toast.error(action === 'restart' ? t('dashboard.failedToRestart') : t('dashboard.failedToShutdown'));
		} finally {
			isActioning = false;
		}
	}
</script>

<svelte:head>
	<title>{t('dashboard.title')} — PowerLab</title>
</svelte:head>

<div class="h-full overflow-y-auto p-6 md:p-8">
	<AppHeader title={t('dashboard.systemDashboard')} subtitle={u?.os ? `${u.os.hostname} · ${t('dashboard.uptime')} ${u.os.uptime_str}` : t('dashboard.realtimeTelemetry')}>
		{#snippet actions()}
			{#if confirmAction === 'restart'}
				<div class="flex items-center gap-2 rounded-xl border border-amber-500/30 bg-amber-500/10 px-3 py-1.5 transition-all">
					<span class="text-[10px] font-bold uppercase tracking-widest text-amber-500">{t('dashboard.confirmAction')}</span>
					<button
						onclick={() => performAction('restart')}
						class="rounded-lg bg-amber-500/20 px-2.5 py-1 text-[11px] font-bold text-amber-400 hover:bg-amber-500/30"
					>{t('action.confirm')}</button>
					<button
						onclick={() => confirmAction = null}
						class="text-[11px] text-zinc-500 hover:text-zinc-300"
					>{t('action.cancel')}</button>
				</div>
			{:else if confirmAction === 'off'}
				<div class="flex items-center gap-2 rounded-xl border border-red-500/30 bg-red-500/10 px-3 py-1.5 transition-all">
					<span class="text-[10px] font-bold uppercase tracking-widest text-red-500">{t('dashboard.confirmAction')}</span>
					<button
						onclick={() => performAction('off')}
						class="rounded-lg bg-red-500/20 px-2.5 py-1 text-[11px] font-bold text-red-400 hover:bg-red-500/30"
					>{t('action.confirm')}</button>
					<button
						onclick={() => confirmAction = null}
						class="text-[11px] text-zinc-500 hover:text-zinc-300"
					>{t('action.cancel')}</button>
				</div>
			{:else}
				<button
					onclick={() => { confirmAction = 'restart'; }}
					class="flex h-10 w-10 items-center justify-center rounded-xl bg-white/[0.03] border border-white/5 text-zinc-400 transition-all hover:bg-white/[0.06] hover:text-amber-500"
					title={t('dashboard.restartServer')}
				>
					<RefreshCw class="h-4 w-4" />
				</button>
				<button
					onclick={() => { confirmAction = 'off'; }}
					class="flex h-10 w-10 items-center justify-center rounded-xl bg-red-500/10 border border-red-500/20 text-red-400 transition-all hover:bg-red-500/20"
					title={t('dashboard.shutdownServer')}
				>
					<PowerOff class="h-4 w-4" />
				</button>
			{/if}
		{/snippet}
	</AppHeader>

	<div class="space-y-6">

		{#if store.loading && !u}
			<!-- Skeleton — match the real layout so the page doesn't reflow when data arrives -->
			<div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
				{#each Array(4) as _}
					<div class="h-[280px] animate-pulse rounded-3xl border border-white/[0.04] bg-white/[0.015]"></div>
				{/each}
			</div>
			<div class="grid gap-4 lg:grid-cols-2">
				{#each Array(2) as _}
					<div class="h-64 animate-pulse rounded-3xl border border-white/[0.04] bg-white/[0.015]"></div>
				{/each}
			</div>
		{:else if u}

			<!-- ── Gauges Row ─────────────────────────────────────────── -->
			<div class="grid gap-4 grid-cols-2 lg:grid-cols-4">

				<!-- CPU Card -->
				<div class="flex flex-col items-center gap-4 rounded-3xl border border-white/[0.06] bg-white/[0.02] p-6 transition-all duration-200 hover:-translate-y-0.5 hover:border-white/10 hover:bg-white/[0.04] hover:shadow-[0_18px_40px_-12px_rgba(0,0,0,0.4)]">
					<RadialGauge value={u.cpu.percent} label={t('dashboard.cpu')} color={cpuColor} size={140} />
					<div class="w-full space-y-2">
						<div class="h-10 w-full overflow-hidden rounded-xl bg-zinc-900/60">
							<Sparkline value={u.cpu.percent} color={cpuColor} height={40} />
						</div>
						<div class="flex justify-between text-[10px] font-bold uppercase tracking-widest text-zinc-600">
							<span class="flex items-center gap-1"><Cpu class="h-3 w-3" /> {u.cpu.num} Cores</span>
							{#if u.cpu.temperature > 0}
								<span class={cn("flex items-center gap-1", u.cpu.temperature > 80 ? 'text-red-500' : u.cpu.temperature > 70 ? 'text-amber-500' : '')}>
									<Thermometer class="h-3 w-3" />{u.cpu.temperature}°C
								</span>
							{/if}
						</div>
					</div>
				</div>

				<!-- RAM Card -->
				<div class="flex flex-col items-center gap-4 rounded-3xl border border-white/[0.06] bg-white/[0.02] p-6 transition-all duration-200 hover:-translate-y-0.5 hover:border-white/10 hover:bg-white/[0.04] hover:shadow-[0_18px_40px_-12px_rgba(0,0,0,0.4)]">
					<RadialGauge value={u.mem.usedPercent} label={t('dashboard.memory')} color={ramColor} size={140} />
					<div class="w-full space-y-2">
						<div class="h-10 w-full overflow-hidden rounded-xl bg-zinc-900/60">
							<Sparkline value={u.mem.usedPercent} color={ramColor} height={40} />
						</div>
						<div class="flex justify-between text-[10px] font-bold uppercase tracking-widest text-zinc-600">
							<span class="flex items-center gap-1"><MemoryStick class="h-3 w-3" />{formatBytes(u.mem.used)}</span>
							<span>{formatBytes(u.mem.total)}</span>
						</div>
					</div>
				</div>

				<!-- GPU Card (conditional) -->
				{#if u.gpu}
					<div class="flex flex-col items-center gap-4 rounded-3xl border border-white/[0.06] bg-white/[0.02] p-6 transition-all duration-200 hover:-translate-y-0.5 hover:border-white/10 hover:bg-white/[0.04] hover:shadow-[0_18px_40px_-12px_rgba(0,0,0,0.4)]">
						<RadialGauge
							value={u.gpu.percent}
							label={t('dashboard.gpu')}
							color={u.gpu.percent > 90 ? '#ef4444' : u.gpu.percent > 75 ? '#f59e0b' : '#14b8a6'}
							size={140}
						/>
						<div class="w-full space-y-2">
							<div class="h-10 w-full overflow-hidden rounded-xl bg-zinc-900/60">
								<Sparkline value={u.gpu.percent} color="#14b8a6" height={40} />
							</div>
							<div class="flex justify-between text-[10px] font-bold uppercase tracking-widest text-zinc-600">
								<span class="truncate max-w-[90px]">{u.gpu.model}</span>
								<span>{formatBytes(u.gpu.memoryUsed)} {t('dashboard.vram')}</span>
							</div>
						</div>
					</div>
				{/if}

				<!-- Network Card -->
				<div class="flex flex-col gap-4 rounded-3xl border border-white/[0.06] bg-white/[0.02] p-6 transition-all duration-200 hover:-translate-y-0.5 hover:border-white/10 hover:bg-white/[0.04] hover:shadow-[0_18px_40px_-12px_rgba(0,0,0,0.4)]">
					<div class="flex items-center gap-2">
						<Activity class="h-4 w-4 text-zinc-600" />
						<span class="text-[10px] font-bold uppercase tracking-widest text-zinc-500">{t('dashboard.network')}</span>
					</div>

					<!-- Dual sparkline -->
					<div class="relative h-24 w-full overflow-hidden rounded-xl bg-zinc-900/60">
						<div class="absolute inset-0 opacity-70">
							<Sparkline value={u.netInRate || 0} color="#06b6d4" height={96} />
						</div>
						<div class="absolute inset-0 opacity-70">
							<Sparkline value={u.netOutRate || 0} color="#f59e0b" height={96} />
						</div>
					</div>

					<div class="grid grid-cols-2 gap-3">
						<div class="rounded-xl bg-cyan-500/10 p-3 text-center">
							<p class="text-[9px] font-bold uppercase tracking-widest text-cyan-600 mb-1">{t('dashboard.download')}</p>
							<p class="text-sm font-bold tabular-nums text-cyan-400">{formatBytes(u.netInRate || 0)}/s</p>
						</div>
						<div class="rounded-xl bg-amber-500/10 p-3 text-center">
							<p class="text-[9px] font-bold uppercase tracking-widest text-amber-600 mb-1">{t('dashboard.upload')}</p>
							<p class="text-sm font-bold tabular-nums text-amber-400">{formatBytes(u.netOutRate || 0)}/s</p>
						</div>
					</div>

					<!-- Interface list -->
					{#if u.net?.length}
						<div class="mt-auto space-y-1.5">
							{#each u.net.slice(0, 3) as iface}
								<div class="flex items-center gap-2 text-[10px] text-zinc-600">
									{#if iface.state === 'up'}
										<Wifi class="h-3 w-3 text-emerald-600" />
									{:else}
										<WifiOff class="h-3 w-3" />
									{/if}
									<span class="font-mono">{iface.name}</span>
								</div>
							{/each}
						</div>
					{/if}
				</div>

			</div>

			<!-- ── Storage + OS Row ───────────────────────────────────── -->
			<div class="grid gap-4 lg:grid-cols-2">

				<!-- Disk Storage Card -->
				<div class="rounded-3xl border border-white/[0.06] bg-white/[0.02] p-6 transition-all duration-200 hover:border-white/10 hover:bg-white/[0.03]">
					<div class="mb-4 flex items-center gap-2">
						<HardDrive class="h-4 w-4 text-zinc-600" />
						<span class="text-[10px] font-bold uppercase tracking-widest text-zinc-500">{t('dashboard.storage')}</span>
					</div>

					{#if store.disks.length === 0}
						<div class="flex h-32 flex-col items-center justify-center gap-2 text-zinc-500">
							<HardDrive class="h-6 w-6 opacity-40" strokeWidth={1.5} />
							<p class="text-xs font-medium">{t('dashboard.noDiskData')}</p>
						</div>
					{:else}
						<div class="space-y-5">
							{#each store.disks as disk}
								{@const health = diskHealth(disk.usedPercent)}
								<div class="space-y-2">
									<div class="flex items-center justify-between">
										<div>
											<span class="text-xs font-medium text-zinc-300">{disk.path}</span>
											<span class="ml-2 text-[10px] text-zinc-600">{disk.fstype}</span>
										</div>
										<span class={cn("text-[10px] font-bold uppercase tracking-wide", diskHealthColor[health])}>
											{health}
										</span>
									</div>
									<div class="h-2 w-full overflow-hidden rounded-full bg-zinc-900">
										<div
											class={cn("h-full rounded-full transition-[width] duration-300 ease-out", diskBarColor[health])}
											style="width: {disk.usedPercent.toFixed(1)}%"
										></div>
									</div>
									<div class="flex justify-between text-[10px] text-zinc-600">
										<span>{formatBytes(disk.used)} {t('dashboard.used')}</span>
										<span>{formatBytes(disk.total)} {t('dashboard.total')} · {disk.usedPercent.toFixed(1)}%</span>
									</div>
								</div>
							{/each}
						</div>
					{/if}
				</div>

				<!-- OS Info Card -->
				{#if u.os}
					<div class="rounded-3xl border border-white/[0.06] bg-white/[0.02] p-6 transition-all duration-200 hover:border-white/10 hover:bg-white/[0.03]">
						<div class="mb-4 flex items-center gap-2">
							<Server class="h-4 w-4 text-zinc-600" />
							<span class="text-[10px] font-bold uppercase tracking-widest text-zinc-500">{t('dashboard.hostInfo')}</span>
						</div>
						<dl class="space-y-4">
							<div class="flex items-start justify-between gap-4 border-b border-white/[0.04] pb-4">
								<dt class="text-xs text-zinc-500 shrink-0">{t('dashboard.hostname')}</dt>
								<dd class="text-xs font-semibold text-zinc-200 text-right truncate">{u.os.hostname}</dd>
							</div>
							<div class="flex items-start justify-between gap-4 border-b border-white/[0.04] pb-4">
								<dt class="text-xs text-zinc-500 shrink-0">{t('dashboard.kernel')}</dt>
								<dd class="text-xs font-mono text-zinc-300 text-right truncate">{u.os.kernel}</dd>
							</div>
							<div class="flex items-start justify-between gap-4 border-b border-white/[0.04] pb-4">
								<dt class="text-xs text-zinc-500 shrink-0">{t('dashboard.cpuModel')}</dt>
								<dd class="text-xs text-zinc-300 text-right truncate max-w-[200px]">{u.cpu.model || '—'}</dd>
							</div>
							<div class="flex items-start justify-between gap-4">
								<dt class="text-xs text-zinc-500 shrink-0">{t('dashboard.uptime')}</dt>
								<dd class="text-xs font-medium tabular-nums text-zinc-300">{u.os.uptime_str}</dd>
							</div>
						</dl>
					</div>
				{/if}

			</div>

		{:else if store.error}
			<div class="flex h-64 flex-col items-center justify-center gap-3 rounded-3xl border border-red-500/10 bg-red-500/5">
				<p class="text-sm text-red-400">{store.error}</p>
				<p class="text-xs text-zinc-600">{t('dashboard.checkBackend')}</p>
			</div>
		{/if}

	</div>
</div>
