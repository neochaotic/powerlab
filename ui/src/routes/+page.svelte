<script lang="ts">
	import { useAppStore } from "$lib/stores/apps.svelte";
	import { useSystemStore } from "$lib/stores/system.svelte";
	import { onMount, onDestroy, tick } from "svelte";
	import { goto } from "$app/navigation";
	import {
		Brain,
		Folder,
		Gauge,
		ShoppingBag,
		SquareCode,
		Pencil,
		Boxes,
		ExternalLink,
		Square,
		Play,
		ScrollText,
		Trash2,
		Settings2,
		X,
	} from "lucide-svelte";
	import { cn } from "$lib/utils";
	import { t } from "$lib/i18n/index.svelte";
	import ContainerLogs from "$lib/components/apps/ContainerLogs.svelte";
	import { Button } from "$lib/components/ui/button";

	const appStore = useAppStore();
	const systemStore = useSystemStore();

	// ── Sort + custom order ────────────────────────────────────────────────────
	let sortMode = $state<"custom" | "alpha" | "status">("custom");
	let customOrder = $state<string[]>([]);

	// ── Modals / popovers ──────────────────────────────────────────────────────
	let activeMenuId = $state<string | null>(null);
	let menuPos = $state<{ x: number; y: number }>({ x: 0, y: 0 });
	let confirmingUninstall = $state<string | null>(null);
	let activeLogAppId = $state<string | null>(null);
	let forkingAppId = $state<string | null>(null);

	// ── Pointer interaction state ──────────────────────────────────────────────
	// One of: 'idle' | 'pressing' | 'dragging'
	// pressing = pointer is down but no drag has started yet (long-press timer alive)
	// dragging = movement threshold exceeded, pointer captured, ghost following
	type Phase = "idle" | "pressing" | "dragging";
	let phase = $state<Phase>("idle");
	let pressId = $state<string | null>(null);
	let pressStartX = 0;
	let pressStartY = 0;
	let pressTileEl: HTMLElement | null = null;
	let longPressTimer: ReturnType<typeof setTimeout> | null = null;
	const DRAG_THRESHOLD = 6; // px of movement before turning press into drag
	const LONG_PRESS_MS = 550; // ms of stillness before opening context menu

	// ── Drag visualization (FLIP) ──────────────────────────────────────────────
	let dragId = $state<string | null>(null);
	let ghostX = $state(0);
	let ghostY = $state(0);
	let ghostDx = $state(0);
	let ghostDy = $state(0);
	let liveOrder = $state<string[]>([]);
	const tileRefs = new Map<string, HTMLElement>();
	let flipPending = false;

	const u = $derived(systemStore.utilization);

	onMount(() => {
		const saved = localStorage.getItem("workload_order");
		if (saved) {
			try {
				customOrder = JSON.parse(saved);
			} catch {
				/* ignore */
			}
		}
		appStore.fetchInstalledApps();
		appStore.fetchAppStore();
		systemStore.startPolling(2000);
	});

	onDestroy(() => {
		systemStore.stopPolling();
		clearLongPress();
	});

	// ── Ordered list ───────────────────────────────────────────────────────────
	const orderedApps = $derived.by(() => {
		const apps = [...appStore.installedApps];
		if (sortMode === "alpha") {
			return apps.sort((a, b) => getTitle(a).localeCompare(getTitle(b)));
		}
		if (sortMode === "status") {
			return apps.sort((a, b) => {
				const ar = a.status === "running" ? 0 : 1;
				const br = b.status === "running" ? 0 : 1;
				if (ar !== br) return ar - br;
				return getTitle(a).localeCompare(getTitle(b));
			});
		}
		if (customOrder.length > 0) {
			const orderMap = new Map(customOrder.map((id, i) => [id, i]));
			return apps.sort((a, b) => {
				const ia = orderMap.get(a.id) ?? 9999;
				const ib = orderMap.get(b.id) ?? 9999;
				if (ia === ib) return getTitle(a).localeCompare(getTitle(b));
				return ia - ib;
			});
		}
		return apps;
	});

	const displayOrder = $derived.by(() => {
		const apps = [...appStore.installedApps];
		if (dragId && liveOrder.length > 0) {
			const idx = new Map(liveOrder.map((id, i) => [id, i]));
			return apps.sort(
				(a, b) => (idx.get(a.id) ?? 9999) - (idx.get(b.id) ?? 9999),
			);
		}
		return orderedApps;
	});

	function registerTile(el: HTMLElement, id: string) {
		tileRefs.set(id, el);
		return {
			update(newId: string) {
				tileRefs.delete(id);
				id = newId;
				tileRefs.set(id, el);
			},
			destroy() {
				tileRefs.delete(id);
			},
		};
	}

	function getTitle(app: {
		id: string;
		store_info: { title?: Record<string, string> };
	}): string {
		const t = app.store_info?.title;
		if (!t) return app.id;
		return t["en_us"] || t["en_US"] || Object.values(t)[0] || app.id;
	}

	// ── Pointer model ──────────────────────────────────────────────────────────
	// All gestures originate at the icon button. No "edit mode" — the same press
	// can become a click (open), a drag (reorder), or a long-press (context menu)
	// depending on what happens next.

	function onTilePointerDown(
		appId: string,
		el: HTMLElement,
		e: PointerEvent,
	) {
		if (e.button !== 0) return; // ignore right/middle mouse buttons
		phase = "pressing";
		pressId = appId;
		pressTileEl = el;
		pressStartX = e.clientX;
		pressStartY = e.clientY;

		// Long-press timer fires only if the pointer hasn't moved meaningfully
		clearLongPress();
		longPressTimer = setTimeout(() => {
			if (phase !== "pressing" || pressId !== appId) return;
			// Open the per-tile context menu, anchored to the tile
			openMenuForTile(appId, el);
			phase = "idle";
			pressId = null;
			if ("vibrate" in navigator) navigator.vibrate(30);
		}, LONG_PRESS_MS);
	}

	function clearLongPress() {
		if (longPressTimer) {
			clearTimeout(longPressTimer);
			longPressTimer = null;
		}
	}

	function onWindowPointerMove(e: PointerEvent) {
		if (phase === "idle" || !pressId) return;

		if (phase === "pressing") {
			const dx = e.clientX - pressStartX;
			const dy = e.clientY - pressStartY;
			if (
				Math.abs(dx) > DRAG_THRESHOLD ||
				Math.abs(dy) > DRAG_THRESHOLD
			) {
				// Promote press → drag
				clearLongPress();
				beginDrag(pressId, pressTileEl!, e);
			}
			return;
		}

		// Already dragging — update ghost and FLIP-reorder when over another tile
		if (phase === "dragging") {
			ghostDx = e.clientX - pressStartX;
			ghostDy = e.clientY - pressStartY;
			if (flipPending) return;
			const els = document.elementsFromPoint(e.clientX, e.clientY);
			for (const candidate of els) {
				const tileId = (candidate as HTMLElement).dataset?.tileId;
				if (tileId && tileId !== dragId) {
					void flipReorder(tileId);
					break;
				}
			}
		}
	}

	function beginDrag(appId: string, el: HTMLElement, _e: PointerEvent) {
		const rect = el.getBoundingClientRect();
		phase = "dragging";
		dragId = appId;
		// Anchor the ghost on the tile's current position; ghostDx/Dy are the
		// delta from the original press point so the cursor stays on the icon.
		ghostX = rect.left + (pressStartX - rect.left) - rect.width / 2;
		ghostY = rect.top + (pressStartY - rect.top) - rect.height / 2;
		ghostDx = 0;
		ghostDy = 0;
		liveOrder = orderedApps.map((a) => a.id);
		activeMenuId = null;
	}

	async function flipReorder(overId: string) {
		if (!dragId || flipPending) return;
		const fromIdx = liveOrder.indexOf(dragId);
		const toIdx = liveOrder.indexOf(overId);
		if (fromIdx === toIdx || fromIdx < 0 || toIdx < 0) return;

		flipPending = true;

		const before = new Map<string, DOMRect>();
		for (const [id, el] of tileRefs) {
			if (id === dragId) continue;
			before.set(id, el.getBoundingClientRect());
		}

		const next = [...liveOrder];
		next.splice(fromIdx, 1);
		next.splice(toIdx, 0, dragId);
		liveOrder = next;

		await tick();

		for (const [id, el] of tileRefs) {
			if (id === dragId) continue;
			const b = before.get(id);
			if (!b) continue;
			const a = el.getBoundingClientRect();
			const dx = b.left - a.left;
			const dy = b.top - a.top;
			if (Math.abs(dx) < 1 && Math.abs(dy) < 1) continue;
			el.style.transition = "none";
			el.style.transform = `translate(${dx}px,${dy}px)`;
			requestAnimationFrame(() => {
				el.style.transform = "";
				el.style.transition =
					"transform 220ms cubic-bezier(0.25,0.46,0.45,0.94)";
			});
		}

		flipPending = false;
	}

	function onWindowPointerUp(e: PointerEvent) {
		if (phase === "idle") return;

		if (phase === "pressing") {
			// Press without movement and without long-press → it's a click.
			// Open the app (or no-op if no port_map).
			clearLongPress();
			const id = pressId;
			phase = "idle";
			pressId = null;
			if (id !== null) {
				const app = appStore.installedApps.find((a) => a.id === id);
				if (app) openApp(app);
			}
			return;
		}

		if (phase === "dragging") {
			// Persist new order
			customOrder = [...liveOrder];
			localStorage.setItem("workload_order", JSON.stringify(customOrder));
			sortMode = "custom";
			phase = "idle";
			pressId = null;
			dragId = null;
			ghostDx = 0;
			ghostDy = 0;
			return;
		}
	}

	// ── Context menu (long press / right click) ────────────────────────────────
	function openMenuForTile(appId: string, _el: HTMLElement) {
		activeMenuId = appId;
		// Anchor menu near the tile — use the press point so the menu opens under
		// the cursor regardless of where the tile is in the grid.
		menuPos = {
			x: Math.min(pressStartX, window.innerWidth - 220),
			y: Math.min(pressStartY + 12, window.innerHeight - 320),
		};
	}

	function onTileContextMenu(appId: string, e: MouseEvent) {
		e.preventDefault();
		activeMenuId = appId;
		menuPos = {
			x: Math.min(e.clientX, window.innerWidth - 220),
			y: Math.min(e.clientY, window.innerHeight - 320),
		};
	}

	function closeMenu() {
		activeMenuId = null;
	}

	// ── Actions (from context menu) ────────────────────────────────────────────
	function openApp(app: { id: string; store_info: { port_map?: string } }) {
		if (app.store_info?.port_map) {
			window.open(
				`http://${window.location.hostname}:${app.store_info.port_map}`,
				"_blank",
			);
		}
	}

	async function handleUninstall(id: string) {
		confirmingUninstall = null;
		activeMenuId = null;
		await appStore.uninstallApp(id);
		customOrder = customOrder.filter((oid) => oid !== id);
		localStorage.setItem("workload_order", JSON.stringify(customOrder));
	}

	function handleEdit(appId: string) {
		activeMenuId = null;
		const app = appStore.installedApps.find((a) => a.id === appId);
		if (!app) return;
		if (appStore.isPowerLabApp(app)) {
			forkingAppId = appId;
		} else {
			goto(`/apps/new?id=${appId}`);
		}
	}

	// Native system apps. Each has its own gradient + matching shadow tint —
	// inspired by iOS/macOS app icons (gradient top-left → bottom-right, soft
	// brand-colored shadow on hover, subtle inner highlight).
	const nativeApps = [
		{
			name: t('dashboard.title'),
			icon: Gauge,
			href: '/dashboard',
			gradient: 'from-violet-400 via-indigo-500 to-blue-600',
			shadow: 'group-hover:shadow-[0_18px_40px_-12px_rgba(99,102,241,0.55)]'
		},
		{
			name: t('apps.appStore'),
			icon: ShoppingBag,
			href: '/apps',
			gradient: 'from-emerald-300 via-emerald-500 to-teal-600',
			shadow: 'group-hover:shadow-[0_18px_40px_-12px_rgba(16,185,129,0.55)]'
		},
		{
			name: t('sidebar.files'),
			icon: Folder,
			href: '/files',
			gradient: 'from-amber-300 via-orange-500 to-red-500',
			shadow: 'group-hover:shadow-[0_18px_40px_-12px_rgba(249,115,22,0.55)]'
		},
		{
			name: t('apps.customApp'),
			icon: SquareCode,
			href: '/apps/new',
			gradient: 'from-pink-400 via-rose-500 to-fuchsia-600',
			shadow: 'group-hover:shadow-[0_18px_40px_-12px_rgba(236,72,153,0.55)]'
		},
		{
			name: t('sidebar.models'),
			icon: Brain,
			href: '/models',
			gradient: 'from-blue-400 via-indigo-500 to-violet-600',
			shadow: 'group-hover:shadow-[0_18px_40px_-12px_rgba(99,102,241,0.55)]',
			badge: 'Soon'
		}
	];

	// Find the menu app reactively so the popover content is always in sync
	const menuApp = $derived.by(() => {
		if (!activeMenuId) return null;
		return (
			appStore.installedApps.find((a) => a.id === activeMenuId) ?? null
		);
	});
</script>

<svelte:head>
	<title>Launchpad — PowerLab</title>
</svelte:head>

<svelte:window
	onpointermove={onWindowPointerMove}
	onpointerup={onWindowPointerUp}
	onpointercancel={onWindowPointerUp}
/>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="h-full overflow-y-auto p-8 md:p-12" onclick={closeMenu}>
	<!-- Welcome -->
	<div class="mb-14">
		<div class="mb-3 flex items-center gap-3">
			<div
				class="h-1.5 w-1.5 animate-pulse rounded-full bg-emerald-500 shadow-[0_0_10px_rgba(16,185,129,0.8)]"
			></div>
			<span
				class="text-[10px] font-black uppercase tracking-[0.4em] text-emerald-500/80"
				>{t('launchpad.systemOperational')}</span
			>
		</div>
		<h1 class="bg-gradient-to-br from-white via-white to-zinc-400 bg-clip-text text-6xl font-black tracking-tighter text-transparent md:text-7xl">
			PowerLab<span class="bg-gradient-to-br from-emerald-300 to-teal-500 bg-clip-text text-transparent">.</span>
		</h1>
		<p
			class="mt-4 max-w-lg text-base font-medium text-zinc-500 leading-relaxed"
		>
			{t('launchpad.runningOn')} <span class="text-zinc-200"
				>{u?.os?.hostname || t('launchpad.yourServer')}</span
			>.
		</p>
	</div>

	<!-- System Apps -->
	<div class="mb-14">
		<h2
			class="mb-6 text-[10px] font-bold uppercase tracking-[0.2em] text-zinc-500"
		>
			{t('launchpad.systemApps')}
		</h2>
		<div
			class="grid grid-cols-4 gap-6 sm:grid-cols-6 md:grid-cols-8 lg:grid-cols-10"
		>
			{#each nativeApps as app}
				{@const Icon = app.icon}
				<a href={app.href} class="group flex flex-col items-center gap-3">
					<div
						class={cn(
							'relative flex h-16 w-16 items-center justify-center rounded-[1.5rem] bg-gradient-to-br text-white shadow-[0_4px_12px_rgba(0,0,0,0.25)] transition-all duration-300 group-hover:-translate-y-1 group-active:scale-95',
							app.gradient,
							app.shadow
						)}
					>
						<!-- Top inner highlight — gives the icon a glassy "lit from above" feel -->
						<span class="pointer-events-none absolute inset-x-2 top-1 h-1/2 rounded-[1.25rem] bg-gradient-to-b from-white/15 to-transparent opacity-70"></span>
						<Icon class="relative h-7 w-7" strokeWidth={2.2} />
						{#if 'badge' in app && app.badge}
							<span class="absolute -right-1 -top-1 rounded-full border border-zinc-950/60 bg-zinc-950/90 px-1.5 py-0.5 text-[8px] font-bold uppercase tracking-wider text-blue-300 backdrop-blur-sm">
								{app.badge}
							</span>
						{/if}
					</div>
					<span class="text-center text-[10px] font-semibold text-zinc-500 transition-colors group-hover:text-zinc-200">
						{app.name}
					</span>
				</a>
			{/each}
		</div>
	</div>

	<!-- Installed Apps -->
	<div>
		<div class="mb-6 flex items-center justify-between">
			<h2
				class="text-[10px] font-bold uppercase tracking-[0.2em] text-zinc-500"
			>
				{t('launchpad.installedApps')}
			</h2>
			<div class="flex items-center gap-3">
				<div
					class="flex items-center gap-1 rounded-full bg-white/[0.04] p-1"
				>
					{#each [["custom", t('launchpad.order')], ["alpha", t('launchpad.az')], ["status", t('launchpad.status')]] as [mode, label]}
						<button
							class={cn(
								"rounded-full px-2.5 py-0.5 text-[10px] font-semibold transition-colors",
								sortMode === mode
									? "bg-white text-black"
									: "text-zinc-500 hover:text-white",
							)}
							onclick={(e) => {
								e.stopPropagation();
								sortMode = mode as typeof sortMode;
							}}>{label}</button
						>
					{/each}
				</div>
			</div>
		</div>

		{#if !appStore.installedLoaded}
			<!-- Initial fetch in flight — skeleton, no flicker between empty/loaded states. -->
			<div
				class="grid grid-cols-4 gap-6 sm:grid-cols-6 md:grid-cols-8 lg:grid-cols-10"
			>
				{#each Array(6) as _}
					<div class="flex flex-col items-center gap-3">
						<div
							class="h-16 w-16 animate-pulse rounded-[1.5rem] bg-white/[0.04]"
						></div>
						<div
							class="h-2.5 w-10 animate-pulse rounded bg-white/[0.04]"
						></div>
					</div>
				{/each}
			</div>
		{:else if appStore.installedApps.length === 0}
			<div class="flex h-48 flex-col items-center justify-center gap-3 rounded-[2rem] border border-dashed border-white/[0.06] bg-white/[0.01]">
				<div class="flex h-12 w-12 items-center justify-center rounded-2xl bg-emerald-500/[0.08]">
					<Boxes class="h-6 w-6 text-emerald-400/80" strokeWidth={1.5} />
				</div>
				<p class="text-sm font-medium text-zinc-400">{t('launchpad.noAppsInstalled')}</p>
				<a
					href="/apps"
					class="inline-flex h-8 items-center gap-1.5 rounded-full border border-emerald-400/20 bg-emerald-400/[0.06] px-3 text-[11px] font-bold uppercase tracking-wider text-emerald-300 transition-colors hover:border-emerald-400/40 hover:bg-emerald-400/[0.1]"
				>
					{t('launchpad.browseStore')} →
				</a>
			</div>
		{:else}
			<p class="mb-4 text-[10px] text-zinc-700">
				{t('launchpad.interactionsHint')}
			</p>

			<div
				class="grid grid-cols-4 gap-6 sm:grid-cols-6 md:grid-cols-8 lg:grid-cols-10"
			>
				{#each displayOrder as app (app.id)}
					{@const info = app.store_info}
					{@const title = getTitle(app)}
					{@const isRunning = app.status === "running"}
					{@const isBeingDragged = dragId === app.id}
					{@const isPressed =
						pressId === app.id && phase === "pressing"}
					<!-- Gate the classification on catalog readiness. Otherwise every
						 app would briefly look "Custom" while the catalog is still loading,
						 because isPowerLabApp() lookups against an empty catalog return false.
						 PowerLab apps stay tagless (the default) — only Custom gets a tag. -->
					{@const isCustom =
						appStore.catalogLoaded && !appStore.isPowerLabApp(app)}

					<div
						use:registerTile={app.id}
						data-tile-id={app.id}
						class="relative flex flex-col items-center gap-3"
					>
						<!-- Icon button — single source of all pointer interactions -->
						<button
							class={cn(
								"relative flex h-16 w-16 items-center justify-center rounded-[1.5rem] border border-white/[0.06] bg-white/[0.04] shadow-sm overflow-hidden touch-none transition-transform duration-200",
								isBeingDragged && "invisible",
								isPressed && "scale-[0.94]",
								!isBeingDragged &&
									!isPressed &&
									"hover:scale-[1.05]",
								isRunning &&
									!isBeingDragged &&
									"shadow-[0_0_0_1.5px_rgba(16,185,129,0.25)]",
							)}
							onpointerdown={(e) =>
								onTilePointerDown(
									app.id,
									(e.currentTarget as HTMLElement)
										.parentElement!,
									e,
								)}
							oncontextmenu={(e) => onTileContextMenu(app.id, e)}
							onclick={(e) => e.stopPropagation()}
							aria-label={title}
						>
							{#if info?.icon}
								<img
									src={info.icon}
									alt={title}
									class="h-10 w-10 object-contain"
									draggable="false"
									onerror={(e) => {
										(
											e.currentTarget as HTMLImageElement
										).style.display = "none";
									}}
								/>
							{:else}
								<div
									class="flex h-10 w-10 items-center justify-center"
								>
									<Boxes
										class="h-7 w-7 text-sky-400"
										strokeWidth={1.5}
									/>
								</div>
							{/if}

							<!-- Status dot (top-right): running / stopped -->
							<div
								class={cn(
									"absolute right-1.5 top-1.5 h-2 w-2 rounded-full border border-zinc-900",
									isRunning
										? "bg-emerald-500 shadow-[0_0_6px_rgba(16,185,129,0.7)]"
										: "bg-zinc-600",
								)}
							></div>
						</button>

						<div class="flex w-full flex-col items-center gap-0.5">
							<span
								class="w-full truncate text-center text-[10px] font-semibold text-zinc-500"
								{title}
							>
								{title}
							</span>
							{#if isCustom}
								<!-- Custom app tag — uppercase, low-contrast amber. Mirrors the
									 "Custom" pill convention from the App Store but Launchpad-discrete. -->
								<span
									class="text-[7px] font-bold uppercase tracking-[0.15em] text-amber-500/60"
								>
									{t('launchpad.custom')}
								</span>
							{/if}
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</div>
</div>

<!-- Drag ghost — fixed position, follows pointer -->
{#if dragId}
	{@const dragApp = displayOrder.find((a) => a.id === dragId)}
	{#if dragApp}
		{@const info = dragApp.store_info}
		<div
			class="pointer-events-none fixed z-[999] flex h-16 w-16 items-center justify-center overflow-hidden rounded-[1.5rem] border border-white/10"
			style="top:{ghostY}px;left:{ghostX}px;transform:translate({ghostDx}px,{ghostDy}px) scale(1.14);transform-origin:center;box-shadow:0 24px 48px -10px rgba(0,0,0,0.8),0 0 0 1px rgba(255,255,255,0.08);will-change:transform"
		>
			{#if info?.icon}
				<img
					src={info.icon}
					alt=""
					class="h-10 w-10 object-contain"
					draggable="false"
					onerror={(e) => {
						(e.currentTarget as HTMLImageElement).style.display =
							"none";
					}}
				/>
			{:else}
				<div
					class="flex h-full w-full items-center justify-center bg-zinc-800"
				>
					<Boxes class="h-7 w-7 text-sky-400" strokeWidth={1.5} />
				</div>
			{/if}
		</div>
	{/if}
{/if}

<!-- Context menu — anchored at long-press / right-click point -->
{#if activeMenuId && menuApp}
	{@const info = menuApp.store_info}
	{@const title = getTitle(menuApp)}
	{@const isRunning = menuApp.status === "running"}
	{@const isPL = appStore.isPowerLabApp(menuApp)}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="fixed z-[1000] w-52 overflow-hidden rounded-2xl border border-white/10 bg-zinc-900/95 shadow-2xl backdrop-blur-xl"
		style="top:{menuPos.y}px;left:{menuPos.x}px"
		onclick={(e) => e.stopPropagation()}
	>
		<div
			class="flex items-center gap-2 border-b border-white/[0.06] px-3 py-2.5"
		>
			{#if info?.icon}
				<img
					src={info.icon}
					alt={title}
					class="h-7 w-7 rounded-lg object-contain"
				/>
			{/if}
			<span class="flex-1 truncate text-[11px] font-semibold text-white"
				>{title}</span
			>
			<button
				class="flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-zinc-500 transition-colors hover:bg-white/10 hover:text-white"
				onclick={closeMenu}
				aria-label="Close menu"
			>
				<X class="h-3 w-3" strokeWidth={2.5} />
			</button>
		</div>

		<div class="py-1">
			{#if info?.port_map}
				<button
					class="flex w-full items-center gap-2.5 px-3 py-2 text-[12px] text-zinc-300 hover:bg-white/5 hover:text-white"
					onclick={() => {
						openApp(menuApp);
						closeMenu();
					}}
				>
					<ExternalLink class="h-3.5 w-3.5 shrink-0" /> {t('launchpad.openUI')}
				</button>
			{/if}
			<button
				class="flex w-full items-center gap-2.5 px-3 py-2 text-[12px] text-zinc-300 hover:bg-white/5 hover:text-white"
				onclick={() => {
					appStore.toggleAppStatus(menuApp.id, menuApp.status);
					closeMenu();
				}}
			>
				{#if isRunning}
					<Square class="h-3.5 w-3.5 shrink-0" /> {t('action.stop')}
				{:else}
					<Play class="h-3.5 w-3.5 shrink-0" /> {t('action.start')}
				{/if}
			</button>
			<button
				class="flex w-full items-center gap-2.5 px-3 py-2 text-[12px] text-zinc-300 hover:bg-white/5 hover:text-white"
				onclick={() => {
					activeLogAppId = menuApp.id;
					closeMenu();
				}}
			>
				<ScrollText class="h-3.5 w-3.5 shrink-0" /> {t('launchpad.viewLogs')}
			</button>
			<button
				class="flex w-full items-center gap-2.5 px-3 py-2 text-[12px] text-zinc-300 hover:bg-white/5 hover:text-white"
				onclick={() => handleEdit(menuApp.id)}
			>
				<Settings2 class="h-3.5 w-3.5 shrink-0" />
				{isPL ? t('launchpad.forkApp') : t('launchpad.editApp')}
			</button>
			<div class="mx-3 my-1 border-t border-white/[0.06]"></div>
			<button
				class="flex w-full items-center gap-2.5 px-3 py-2 text-[12px] text-red-400 hover:bg-red-500/10 hover:text-red-300"
				onclick={() => {
					confirmingUninstall = menuApp.id;
					closeMenu();
				}}
			>
				<Trash2 class="h-3.5 w-3.5 shrink-0" /> {t('launchpad.uninstall')}
			</button>
		</div>
	</div>
{/if}

<!-- Logs modal -->
{#if activeLogAppId}
	<ContainerLogs
		appId={activeLogAppId}
		onClose={() => {
			activeLogAppId = null;
		}}
	/>
{/if}

<!-- Fork confirmation -->
{#if forkingAppId}
	<div
		class="fixed inset-0 z-50 flex items-end justify-center bg-black/60 backdrop-blur-md sm:items-center"
		onclick={() => (forkingAppId = null)}
		role="presentation"
	>
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<!-- svelte-ignore a11y_interactive_supports_focus -->
		<div
			class="w-full max-w-sm rounded-t-[2rem] border border-white/[0.06] bg-zinc-950/90 p-6 shadow-[0_24px_48px_-12px_rgba(0,0,0,0.6)] backdrop-blur-2xl sm:rounded-2xl"
			onclick={(e) => e.stopPropagation()}
			role="dialog"
			tabindex="-1"
		>
			<div class="mb-4 flex items-center gap-3">
				<div class="flex h-10 w-10 items-center justify-center rounded-xl bg-amber-500/10 text-amber-400">
					<Pencil class="h-5 w-5" />
				</div>
				<div>
					<p class="font-semibold text-white">{t('launchpad.forkApp')}</p>
					<p class="text-xs text-zinc-500">{t('launchpad.originalUntouched')}</p>
				</div>
			</div>
			<p class="mb-5 text-sm leading-relaxed text-zinc-400">
				{t('launchpad.forkDesc')}
			</p>
			<div class="flex gap-2">
				<Button
					variant="ghost"
					class="flex-1 rounded-xl"
					onclick={() => (forkingAppId = null)}
				>
					{t('action.cancel')}
				</Button>
				<Button
					class="flex-1 rounded-xl bg-amber-500 font-bold text-black shadow-[0_8px_24px_-8px_rgba(245,158,11,0.5)] hover:bg-amber-400"
					onclick={() => {
						goto(`/apps/new?id=${forkingAppId}&fork=1`);
						forkingAppId = null;
					}}
				>
					{t('launchpad.forkBtn')}
				</Button>
			</div>
		</div>
	</div>
{/if}

<!-- Uninstall confirmation -->
{#if confirmingUninstall}
	<div
		class="fixed inset-0 z-50 flex items-end justify-center bg-black/60 backdrop-blur-md sm:items-center"
		onclick={() => (confirmingUninstall = null)}
		role="presentation"
	>
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<!-- svelte-ignore a11y_interactive_supports_focus -->
		<div
			class="w-full max-w-sm rounded-t-[2rem] border border-white/[0.06] bg-zinc-950/90 p-6 shadow-[0_24px_48px_-12px_rgba(0,0,0,0.6)] backdrop-blur-2xl sm:rounded-2xl"
			onclick={(e) => e.stopPropagation()}
			role="dialog"
			tabindex="-1"
		>
			<div class="mb-4 flex items-center gap-3">
				<div class="flex h-10 w-10 items-center justify-center rounded-xl bg-red-500/10 text-red-400">
					<Trash2 class="h-5 w-5" />
				</div>
				<div>
					<p class="font-semibold text-white">Uninstall app</p>
					<p class="text-xs text-zinc-500">Containers stop and are removed</p>
				</div>
			</div>
			<p class="mb-5 text-sm leading-relaxed text-zinc-400">
				Your data and config files in <code class="rounded bg-white/5 px-1 py-0.5 font-mono text-[11px] text-zinc-300">/DATA/AppData</code> are preserved.
			</p>
			<div class="flex gap-2">
				<Button
					variant="ghost"
					class="flex-1 rounded-xl"
					onclick={() => (confirmingUninstall = null)}>Cancel</Button>
				<Button
					class="flex-1 rounded-xl bg-red-600 text-white shadow-[0_8px_24px_-8px_rgba(220,38,38,0.6)] hover:bg-red-500"
					onclick={() => handleUninstall(confirmingUninstall!)}
				>
					Uninstall
				</Button>
			</div>
		</div>
	</div>
{/if}
