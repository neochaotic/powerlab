<script lang="ts">
	import { onDestroy } from 'svelte';
	import { Activity, Pause, Play, Trash2 } from 'lucide-svelte';
	import { cn } from '$lib/utils';
	import { getAuthToken } from '$lib/api/client';

	// Per-service journald live stream viewer. Pairs with the backend
	// SSE endpoint at /v1/logs/services/{service}/stream which spawns
	// `journalctl -u powerlab-<service>.service -o json -f` and emits
	// each entry as { severity, message, ts_micro }.
	//
	// Behavior:
	//   - EventSource passes the JWT in the `?token=` query param
	//     (browsers don't let you set headers on EventSource).
	//   - Auto-scrolls when a new entry arrives, UNLESS the user has
	//     scrolled up — that's a paused state.
	//   - Capped at 1000 lines client-side; older entries are dropped
	//     from the head. Browsers freeze around 10k DOM nodes; 1000
	//     lines × 1 node each is comfortable.

	interface JournalEntry {
		severity: 'error' | 'warn' | 'info' | 'debug';
		message: string;
		ts_micro?: string;
	}

	// Known PowerLab unit names. The backend prepends `powerlab-` and
	// appends `.service` before invoking journalctl, so we list the
	// bare service portion here.
	const KNOWN_SERVICES = [
		'gateway',
		'app-management',
		'core',
		'message-bus',
		'user',
		'local-storage',
		'sync-catalog',
	] as const;

	const MAX_LINES = 1000;

	let selectedService = $state<string | null>(null);
	let entries = $state<JournalEntry[]>([]);
	let streaming = $state(false);
	let error = $state<string | null>(null);
	let autoScroll = $state(true);
	let scrollContainer: HTMLDivElement | null = $state(null);
	let eventSource: EventSource | null = null;

	function selectService(svc: string) {
		closeStream();
		selectedService = svc;
		entries = [];
		error = null;
		openStream(svc);
	}

	function openStream(svc: string) {
		const token = getAuthToken();
		if (!token) {
			error = 'Not authenticated — JWT token unavailable.';
			return;
		}
		const url = `/v1/logs/services/${encodeURIComponent(svc)}/stream?token=${encodeURIComponent(token)}`;
		const es = new EventSource(url);
		eventSource = es;
		streaming = true;
		error = null;

		es.onmessage = (e) => {
			try {
				const entry = JSON.parse(e.data) as JournalEntry;
				entries.push(entry);
				if (entries.length > MAX_LINES) {
					entries = entries.slice(-MAX_LINES);
				}
				if (autoScroll) scheduleScroll();
			} catch {
				// Skip non-JSON SSE payloads (the `: stream-open` comment
				// is filtered out by EventSource itself, so this only
				// guards against future malformed entries).
			}
		};

		// Backend emits `event: error` with a string payload on
		// subprocess-start failures. EventSource doesn't fire onmessage
		// for typed events, so listen explicitly.
		es.addEventListener('error', (e) => {
			const msgEvent = e as MessageEvent;
			if (msgEvent?.data) {
				try {
					error = JSON.parse(msgEvent.data) as string;
				} catch {
					error = String(msgEvent.data);
				}
				closeStream();
			}
		});

		es.onerror = () => {
			// EventSource's automatic-reconnect kicks in unless we close.
			// We let it try once; subsequent failures show as the
			// browser-reported readyState transitions.
			if (es.readyState === EventSource.CLOSED) {
				streaming = false;
			}
		};
	}

	function closeStream() {
		if (eventSource) {
			eventSource.close();
			eventSource = null;
		}
		streaming = false;
	}

	function pauseToggle() {
		if (streaming) {
			closeStream();
		} else if (selectedService) {
			openStream(selectedService);
		}
	}

	function clearEntries() {
		entries = [];
	}

	function scheduleScroll() {
		// Defer past the DOM update so we scroll AFTER the new line
		// has rendered. requestAnimationFrame matches the browser's
		// paint cycle and avoids the "scrolls to second-to-last"
		// bug from scrolling synchronously.
		requestAnimationFrame(() => {
			if (scrollContainer && autoScroll) {
				scrollContainer.scrollTop = scrollContainer.scrollHeight;
			}
		});
	}

	function onScrollHandler() {
		if (!scrollContainer) return;
		const atBottom =
			scrollContainer.scrollHeight - scrollContainer.scrollTop - scrollContainer.clientHeight < 8;
		autoScroll = atBottom;
	}

	function severityClass(s: JournalEntry['severity']): string {
		switch (s) {
			case 'error':
				return 'text-red-400';
			case 'warn':
				return 'text-yellow-300';
			case 'info':
				return 'text-zinc-300';
			case 'debug':
				return 'text-zinc-500';
		}
	}

	function formatTs(tsMicro?: string): string {
		if (!tsMicro) return '';
		const n = Number.parseInt(tsMicro, 10);
		if (!Number.isFinite(n)) return '';
		const d = new Date(n / 1000);
		return d.toLocaleTimeString();
	}

	onDestroy(closeStream);
</script>

<div class="grid grid-cols-1 gap-4 md:grid-cols-[200px_1fr]" data-testid="logs-services-tab">
	<!-- Service list -->
	<div class="overflow-hidden rounded-2xl border border-white/[0.06] bg-white/[0.02]">
		<div
			class="flex items-center gap-2 border-b border-white/[0.04] px-4 py-3 text-sm font-medium text-zinc-200"
		>
			<Activity class="h-4 w-4 text-zinc-400" />
			Services
		</div>
		<ul class="divide-y divide-white/[0.03]">
			{#each KNOWN_SERVICES as svc (svc)}
				<li>
					<button
						onclick={() => selectService(svc)}
						data-testid="logs-service-{svc}"
						class={cn(
							'w-full px-4 py-2.5 text-left text-xs transition-colors hover:bg-white/[0.03]',
							selectedService === svc && 'bg-white/[0.06] text-white'
						)}
					>
						<div class="font-mono text-zinc-200">{svc}</div>
					</button>
				</li>
			{/each}
		</ul>
	</div>

	<!-- Live stream viewer -->
	<div class="overflow-hidden rounded-2xl border border-white/[0.06] bg-white/[0.02]" data-testid="logs-stream">
		<div class="flex items-center justify-between border-b border-white/[0.04] px-4 py-3">
			<div class="flex items-center gap-2">
				<span class="font-mono text-xs text-zinc-300">
					{selectedService ?? 'Select a service →'}
				</span>
				{#if streaming}
					<span
						class="flex h-2 w-2 animate-pulse rounded-full bg-emerald-400"
						title="Streaming"
					></span>
				{/if}
			</div>
			{#if selectedService}
				<div class="flex items-center gap-2">
					<button
						onclick={pauseToggle}
						class="flex h-7 items-center gap-1.5 rounded-md border border-white/10 bg-white/[0.03] px-2 text-[10px] text-zinc-300 transition-colors hover:border-white/20 hover:text-white"
						data-testid="logs-stream-toggle"
					>
						{#if streaming}
							<Pause class="h-3 w-3" />
							Pause
						{:else}
							<Play class="h-3 w-3" />
							Resume
						{/if}
					</button>
					<button
						onclick={clearEntries}
						class="flex h-7 items-center gap-1.5 rounded-md border border-white/10 bg-white/[0.03] px-2 text-[10px] text-zinc-300 transition-colors hover:border-white/20 hover:text-white"
						data-testid="logs-stream-clear"
					>
						<Trash2 class="h-3 w-3" />
						Clear
					</button>
				</div>
			{/if}
		</div>

		{#if error}
			<div
				class="border-b border-red-500/20 bg-red-500/[0.05] px-4 py-2 text-xs text-red-400"
				data-testid="logs-stream-error"
			>
				{error}
			</div>
		{/if}

		<div
			class="max-h-[60vh] overflow-y-auto custom-scrollbar"
			bind:this={scrollContainer}
			onscroll={onScrollHandler}
		>
			{#if !selectedService}
				<div class="px-4 py-8 text-center text-sm text-zinc-500">
					Pick a service from the left to start streaming its journald output.
				</div>
			{:else if entries.length === 0 && !error}
				<div class="px-4 py-8 text-center text-sm text-zinc-500">
					{streaming ? 'Waiting for entries…' : 'Stream paused.'}
				</div>
			{:else}
				<ul class="divide-y divide-white/[0.02] font-mono text-[11px] leading-relaxed">
					{#each entries as entry, i (i)}
						<li class="flex items-start gap-3 px-4 py-1.5">
							<span class="shrink-0 text-zinc-600">{formatTs(entry.ts_micro)}</span>
							<span
								class={cn(
									'shrink-0 w-12 text-[10px] uppercase tracking-wide',
									severityClass(entry.severity)
								)}>{entry.severity}</span
							>
							<span class={cn('whitespace-pre-wrap break-all', severityClass(entry.severity))}
								>{entry.message}</span
							>
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	</div>
</div>
