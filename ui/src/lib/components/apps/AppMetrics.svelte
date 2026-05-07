<script lang="ts">
    import { getComposeAppStats, type ComposeAppStats } from '$lib/api/apps';
    import Sparkline from '$lib/components/dashboard/Sparkline.svelte';
    import { onMount, onDestroy } from 'svelte';
    import { Cpu, HardDrive, Network, X, Activity } from 'lucide-svelte';
    import { formatSize, formatPercent, cn } from '$lib/utils';
    import { fade, scale } from 'svelte/transition';
    import { t } from '$lib/i18n/index.svelte';

    interface Props {
        appId: string;
        onClose: () => void;
    }

    let { appId, onClose }: Props = $props();

    let stats = $state<ComposeAppStats | null>(null);
    let history = $state<{
        cpu: number[],
        mem: number[],
        net_rx: number[],
        net_tx: number[]
    }>({ cpu: [], mem: [], net_rx: [], net_tx: [] });

    let interval: any;

    async function update() {
        try {
            const res = await getComposeAppStats(appId);
            if (res.data) {
                stats = res.data;
                // Keep last 30 points
                history.cpu = [...history.cpu.slice(-29), res.data.cpu_percent];
                history.mem = [...history.mem.slice(-29), res.data.mem_used_bytes];
                history.net_rx = [...history.net_rx.slice(-29), res.data.net_rx];
                history.net_tx = [...history.net_tx.slice(-29), res.data.net_tx];
            }
        } catch (e) {
            console.error('Failed to fetch app stats:', e);
        }
    }

    onMount(() => {
        update();
        interval = setInterval(update, 2000);
    });

    onDestroy(() => clearInterval(interval));
</script>

<div 
    class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm"
    transition:fade={{ duration: 200 }}
>
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div 
        class="absolute inset-0" 
        onclick={onClose}
    ></div>

    <div 
        class="relative w-full max-w-2xl rounded-3xl border border-white/10 bg-zinc-950 p-6 shadow-2xl overflow-hidden"
        transition:scale={{ duration: 300, start: 0.95 }}
    >
        <!-- Background Glow -->
        <div class="absolute -right-20 -top-20 h-64 w-64 rounded-full bg-emerald-500/10 blur-[100px]"></div>
        <div class="absolute -left-20 -bottom-20 h-64 w-64 rounded-full bg-blue-500/10 blur-[100px]"></div>

        <div class="flex items-center justify-between mb-8 relative">
            <div class="flex items-center gap-3">
                <div class="flex h-12 w-12 items-center justify-center rounded-2xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-500 shadow-[0_0_20px_rgba(16,185,129,0.1)]">
                    <Activity class="h-6 w-6" />
                </div>
                <div>
                    <h2 class="text-xl font-black tracking-tight text-white">{appId}</h2>
                    <p class="text-[10px] font-bold uppercase tracking-widest text-zinc-500">{t('metrics.realtimePerformance')}</p>
                </div>
            </div>
            <button 
                onclick={onClose}
                class="flex h-10 w-10 items-center justify-center rounded-xl bg-white/5 text-zinc-400 transition-all hover:bg-white/10 hover:text-white"
            >
                <X class="h-5 w-5" />
            </button>
        </div>

        {#if !stats}
            <div class="flex h-64 items-center justify-center">
                <div class="flex flex-col items-center gap-3">
                    <div class="h-8 w-8 animate-spin rounded-full border-2 border-emerald-500 border-t-transparent"></div>
                    <span class="text-[10px] font-bold uppercase tracking-widest text-zinc-600">{t('metrics.connectingDocker')}</span>
                </div>
            </div>
        {:else}
            <div class="grid grid-cols-1 md:grid-cols-2 gap-4 relative">
                <!-- CPU -->
                <div class="group rounded-2xl border border-white/5 bg-white/[0.02] p-5 transition-all hover:bg-white/[0.04]">
                    <div class="flex items-center justify-between mb-4">
                        <div class="flex items-center gap-3">
                            <Cpu class="h-4 w-4 text-blue-500" />
                            <span class="text-[11px] font-black uppercase tracking-widest text-zinc-400">{t('metrics.processor')}</span>
                        </div>
                        <span class="font-mono text-lg font-bold text-white">{formatPercent(stats.cpu_percent)}</span>
                    </div>
                    <div class="h-16 w-full overflow-hidden rounded-xl bg-zinc-950/50">
                        <Sparkline 
                            values={history.cpu} 
                            color="#3b82f6" 
                            height={64} 
                        />
                    </div>
                </div>

                <!-- Memory -->
                <div class="group rounded-2xl border border-white/5 bg-white/[0.02] p-5 transition-all hover:bg-white/[0.04]">
                    <div class="flex items-center justify-between mb-4">
                        <div class="flex items-center gap-3">
                            <HardDrive class="h-4 w-4 text-purple-500" />
                            <span class="text-[11px] font-black uppercase tracking-widest text-zinc-400">{t('metrics.memory')}</span>
                        </div>
                        <span class="font-mono text-lg font-bold text-white">{formatSize(stats.mem_used_bytes)}</span>
                    </div>
                    <div class="h-16 w-full overflow-hidden rounded-xl bg-zinc-950/50">
                        <Sparkline 
                            values={history.mem} 
                            color="#a855f7" 
                            height={64} 
                        />
                    </div>
                    <div class="mt-2 flex justify-between text-[9px] font-bold uppercase tracking-widest text-zinc-600">
                        <span>{t('metrics.used')}</span>
                        <span>{t('metrics.limit')}: {formatSize(stats.mem_limit_bytes)}</span>
                    </div>
                </div>

                <!-- Network Rx -->
                <div class="group rounded-2xl border border-white/5 bg-white/[0.02] p-5 transition-all hover:bg-white/[0.04] md:col-span-1">
                    <div class="flex items-center justify-between mb-4">
                        <div class="flex items-center gap-3">
                            <Network class="h-4 w-4 text-cyan-500" />
                            <span class="text-[11px] font-black uppercase tracking-widest text-zinc-400">{t('metrics.inbound')}</span>
                        </div>
                        <span class="font-mono text-lg font-bold text-white">{formatSize(stats.net_rx)}</span>
                    </div>
                    <div class="h-16 w-full overflow-hidden rounded-xl bg-zinc-950/50">
                        <Sparkline 
                            values={history.net_rx} 
                            color="#06b6d4" 
                            height={64} 
                        />
                    </div>
                </div>

                <!-- Network Tx -->
                <div class="group rounded-2xl border border-white/5 bg-white/[0.02] p-5 transition-all hover:bg-white/[0.04] md:col-span-1">
                    <div class="flex items-center justify-between mb-4">
                        <div class="flex items-center gap-3">
                            <Network class="h-4 w-4 text-amber-500" />
                            <span class="text-[11px] font-black uppercase tracking-widest text-zinc-400">{t('metrics.outbound')}</span>
                        </div>
                        <span class="font-mono text-lg font-bold text-white">{formatSize(stats.net_tx)}</span>
                    </div>
                    <div class="h-16 w-full overflow-hidden rounded-xl bg-zinc-950/50">
                        <Sparkline 
                            values={history.net_tx} 
                            color="#f59e0b" 
                            height={64} 
                        />
                    </div>
                </div>
            </div>

            <div class="mt-8 flex items-center justify-between rounded-2xl border border-emerald-500/10 bg-emerald-500/[0.02] px-4 py-3">
                <div class="flex items-center gap-3">
                    <div class="h-2 w-2 rounded-full bg-emerald-500 shadow-[0_0_10px_rgba(16,185,129,0.8)] animate-pulse"></div>
                    <span class="text-[10px] font-black uppercase tracking-widest text-emerald-500/80">{t('metrics.liveConnectionActive')}</span>
                </div>
                <span class="text-[10px] font-bold text-zinc-600">{t('metrics.updatedEvery')}</span>
            </div>
        {/if}
    </div>
</div>
