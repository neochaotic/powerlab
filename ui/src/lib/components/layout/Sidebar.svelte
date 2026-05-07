<script lang="ts">
	import { onMount, onDestroy } from "svelte";
	import { fade, slide } from "svelte/transition";
	import { page } from "$app/stores";
	import {
		User,
		Settings,
		Terminal,
		Cpu,
		MemoryStick,
		Activity,
		Thermometer,
		LogOut,
		House,
		Gauge,
		Folder,
		ShoppingBag,
		Brain,
		BookOpen,
		HardDrive,
		Zap,
		Sun,
		Moon,
	} from "lucide-svelte";
	import Sparkline from "$lib/components/dashboard/Sparkline.svelte";
	import MiniProgress from "$lib/components/dashboard/MiniProgress.svelte";
	import { useSystemStore } from "$lib/stores/system.svelte";
	import { useAppStore } from "$lib/stores/apps.svelte";
	import { auth } from "$lib/stores/auth.svelte";
	import { cn, formatSize } from "$lib/utils";
	import { ui } from "$lib/stores/ui.svelte";
	import { t } from "$lib/i18n/index.svelte";
	import TerminalComponent from "$lib/components/terminal/Terminal.svelte";

	const store = useSystemStore();
	const appStore = useAppStore();
	let isCollapsed = $state(false);
	let isMobileOpen = $state(false);
	let now = $state(new Date());
	let clockInterval: ReturnType<typeof setInterval>;

	// Terminal — local pty (no SSH, no credentials). Opening just sets
	// ui.isTerminalOpen and the Terminal component connects to /v1/sys/wsshell.
	let isDark = $state(true);

	onMount(() => {
		const saved = localStorage.getItem("sidebar-collapsed");
		if (saved !== null) {
			isCollapsed = saved === "true";
		}

		const savedTheme = localStorage.getItem("theme");
		if (savedTheme === "light") {
			isDark = false;
			document.documentElement.setAttribute("data-theme", "light");
		} else {
			document.documentElement.setAttribute("data-theme", "dark");
		}

		store.startPolling(1000);
		appStore.fetchInstalledApps();
		clockInterval = setInterval(() => {
			now = new Date();
		}, 60000);
	});

	onDestroy(() => {
		store.stopPolling();
		clearInterval(clockInterval);
	});

	function toggleSidebar() {
		isCollapsed = !isCollapsed;
		localStorage.setItem("sidebar-collapsed", isCollapsed.toString());
	}

	function openTerminalFeature() {
		ui.isTerminalOpen = true;
	}

	function toggleTheme() {
		isDark = !isDark;
		const theme = isDark ? "dark" : "light";
		document.documentElement.setAttribute("data-theme", theme);
		localStorage.setItem("theme", theme);
	}

	onDestroy(() => {
		store.stopPolling();
		if (clockInterval) clearInterval(clockInterval);
	});

	const u = $derived(store.utilization);

	// Same icons as the Launchpad system tiles — one symbol per concept,
	// used everywhere in the OS.
	// `/docs` opens in a new tab because Scalar mounts its own
	// router inside the page; we don't want it sharing history
	// with the SPA router. Icon: BookOpen — same metaphor used
	// for "documentation" across the rest of the OS.
	const navItems = [
		{ href: '/dashboard', icon: Gauge, label: t('nav.dashboard') },
		{ href: '/files', icon: Folder, label: t('nav.files') },
		{ href: '/apps', icon: ShoppingBag, label: t('nav.apps') },
		{ href: '/models', icon: Brain, label: t('nav.models') },
		{ href: '/docs', icon: BookOpen, label: t('nav.apiDocs'), external: true }
	];

	const currentPath = $derived($page.url.pathname);

	// Status calculators for Bleeding Edge glow
	const cpuStatus = $derived.by<"normal" | "warning" | "critical">(() => {
		if (!u) return "normal";
		if (u.cpu.percent > 95) return "critical";
		if (u.cpu.percent > 90) return "warning";
		return "normal";
	});

	const ramStatus = $derived.by<"normal" | "warning" | "critical">(() => {
		if (!u) return "normal";
		if (u.mem.usedPercent > 95) return "critical";
		if (u.mem.usedPercent > 85) return "warning";
		return "normal";
	});

	const tempStatus = $derived.by<"normal" | "warning" | "critical">(() => {
		if (!u || !u.cpu.temperature) return "normal";
		if (u.cpu.temperature > 80) return "critical";
		if (u.cpu.temperature > 70) return "warning";
		return "normal";
	});

	function formatClock(d: Date) {
		const time = d.toLocaleTimeString("en-US", {
			hour: "2-digit",
			minute: "2-digit",
			hour12: false,
		});
		const date = d.toLocaleDateString("en-US", {
			weekday: "short",
			day: "numeric",
			month: "short",
		});
		return `${time} · ${date}`;
	}

	import { Menu, X } from "lucide-svelte";
</script>

<!-- Mobile Menu Trigger -->
<button
	class="fixed bottom-6 right-6 z-[60] flex h-14 w-14 items-center justify-center rounded-full bg-emerald-500 text-zinc-950 shadow-[0_8px_32px_rgba(16,185,129,0.4)] lg:hidden transition-transform active:scale-95"
	onclick={() => (isMobileOpen = !isMobileOpen)}
>
	{#if isMobileOpen}
		<X class="h-6 w-6" />
	{:else}
		<Menu class="h-6 w-6" />
	{/if}
</button>

<aside
	class={cn(
		"flex h-screen flex-col border-r border-white/5 bg-zinc-950/60 backdrop-blur-2xl transition-all duration-500 ease-in-out z-50 overflow-x-hidden shadow-2xl lg:shadow-none",
		"lg:relative fixed lg:translate-x-0",
		isCollapsed ? "w-[80px]" : "w-[280px]",
		isMobileOpen ? "translate-x-0" : "-translate-x-full lg:translate-x-0",
	)}
>
	<!-- Mobile Overlay -->
	{#if isMobileOpen}
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div
			class="fixed inset-0 z-[-1] bg-black/60 backdrop-blur-sm lg:hidden"
			onclick={() => (isMobileOpen = false)}
		></div>
	{/if}
	<!-- Branding Header + Toggle -->
	<div
		class="flex h-20 shrink-0 items-center justify-between px-4 overflow-hidden relative"
	>
		<a
			href="/"
			class={cn(
				"flex items-center gap-3 transition-all duration-500",
				isCollapsed
					? "opacity-0 invisible -translate-x-10"
					: "opacity-100 visible",
			)}
		>
			<div
				class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-white/[0.03] border border-white/10 shadow-inner"
			>
				<span class="text-xl font-black text-white leading-none"
					>P<span class="text-emerald-500">.</span></span
				>
			</div>
			<div class="flex flex-col whitespace-nowrap">
				<h2
					class="text-lg font-black tracking-tighter text-white leading-none"
				>
					PowerLab<span class="text-emerald-500">.</span>
				</h2>
				<span
					class="text-[9px] font-bold uppercase tracking-[0.2em] text-zinc-500"
					>{t('sidebar.homeServer')}</span
				>
			</div>
		</a>

		{#if isCollapsed}
			<div
				class="absolute left-1/2 -translate-x-1/2 flex h-10 w-10 items-center justify-center rounded-xl bg-white/[0.03] border border-white/10 transition-all duration-500"
			>
				<span class="text-xl font-black text-white leading-none"
					>P<span class="text-emerald-500">.</span></span
				>
			</div>
		{/if}

		<button
			onclick={toggleSidebar}
			aria-label={isCollapsed ? t('sidebar.expand') : t('sidebar.collapse')}
			class={cn(
				"flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-zinc-500 transition-all hover:bg-white/[0.05] hover:text-white",
				isCollapsed ? "absolute right-2 top-7 h-6 w-6" : "",
			)}
		>
			<div
				class={cn(
					"transition-transform duration-500",
					isCollapsed && "rotate-180",
				)}
			>
				<svg
					width="15"
					height="15"
					viewBox="0 0 15 15"
					fill="none"
					xmlns="http://www.w3.org/2000/svg"
					class="h-4 w-4"
				>
					<path
						d="M8.84182 3.13514C9.04327 3.32401 9.05348 3.64042 8.86462 3.84188L5.43521 7.49991L8.86462 11.1579C9.05348 11.3594 9.04327 11.6758 8.84182 11.8647C8.64036 12.0535 8.32394 12.0433 8.13508 11.8419L4.38508 7.84188C4.20477 7.64955 4.20477 7.35027 4.38508 7.15794L8.13508 3.15794C8.32394 2.95648 8.64036 2.94628 8.84182 3.13514Z"
						fill="currentColor"
						fill-rule="evenodd"
						clip-rule="evenodd"
					></path>
				</svg>
			</div>
		</button>
	</div>

	<!-- Navigation — horizontal icon dock. Labels NEVER render in the bar
		 (no isCollapsed branching). Tooltip appears below on hover. -->
	<nav class="shrink-0 px-3 pb-4 border-b border-white/[0.03]">
		<div class="flex items-center justify-center gap-1 rounded-2xl border border-white/5 bg-white/[0.02] p-1.5">
			{#each navItems as item}
				{@const Icon = item.icon}
				{@const isActive = currentPath === item.href || (item.href !== '/' && currentPath.startsWith(item.href))}
				{@const appCount = item.label === 'Store' ? appStore.installedApps.length : 0}

				<a
					href={item.href}
					aria-label={item.label}
					target={item.external ? '_blank' : undefined}
					rel={item.external ? 'noopener noreferrer' : undefined}
					class={cn(
						'group relative flex h-10 w-10 items-center justify-center rounded-xl transition-all duration-200',
						isActive
							? 'bg-emerald-400/10 text-emerald-400 shadow-[0_0_15px_rgba(52,211,153,0.15)] border border-emerald-400/20'
							: 'text-zinc-500 hover:bg-white/[0.05] hover:text-zinc-200 border border-transparent'
					)}
				>
					<Icon class="h-[18px] w-[18px] shrink-0" />

					{#if appCount > 0}
						<span class="absolute -right-1 -top-1 flex h-4 min-w-[16px] items-center justify-center rounded-full bg-gradient-to-br from-emerald-400 to-teal-500 px-1 text-[9px] font-bold text-zinc-950 shadow-[0_0_8px_rgba(52,211,153,0.5)]">
							{appCount}
						</span>
					{/if}

					{#if isActive}
						<div class="absolute -bottom-1 left-1/2 h-0.5 w-4 -translate-x-1/2 rounded-t-full bg-emerald-400 shadow-[0_0_10px_rgba(52,211,153,0.8)]"></div>
					{/if}

					<!-- Hover tooltip — positioned below the icon for the horizontal dock -->
					<div class="pointer-events-none absolute left-1/2 top-full mt-2 -translate-x-1/2 opacity-0 transition-opacity duration-150 group-hover:opacity-100 z-[100]">
						<div class="rounded-lg border border-white/10 bg-zinc-900/95 px-2.5 py-1 backdrop-blur-md whitespace-nowrap">
							<span class="text-[10px] font-bold uppercase tracking-[0.15em] text-white">{item.label}</span>
						</div>
					</div>
				</a>
			{/each}
		</div>
	</nav>

	<!-- System Telemetry Widgets -->
	<div class="flex-1 overflow-y-auto px-2 py-4 no-scrollbar">
		{#if store.loading && !u}
			<div class="flex flex-col gap-4">
				{#each Array(3) as _}
					<div
						class={cn(
							"animate-pulse rounded-xl bg-white/[0.02]",
							isCollapsed ? "h-10 w-10 mx-auto" : "h-24",
						)}
					></div>
				{/each}
			</div>
		{:else if u}
			<div class="flex flex-col gap-4">
				{#if !isCollapsed}
					<!-- OS Info & Clock -->
					<div class="mb-2 px-2 flex flex-col gap-0.5">
						<div class="flex items-center justify-between">
							<span
								class="text-[11px] font-black tracking-tighter text-white uppercase"
								>{u.os?.hostname || "PowerLab"}</span
							>
							<span
								class="text-[9px] font-bold text-zinc-600 uppercase tracking-widest"
								>{u.os?.uptime_str || ""}</span
							>
						</div>
						<div class="font-mono text-[10px] text-zinc-500">
							{formatClock(now)}
						</div>
					</div>
				{/if}

				{#if isCollapsed}
					<!-- Compact Status Indicators -->
					<div
						class="flex flex-row flex-wrap items-center justify-center gap-4 py-2"
					>
						<div
							class="group relative cursor-help"
							title={`${t('dashboard.cpu')}: ${u.cpu.percent.toFixed(0)}%`}
						>
							<Cpu
								class={cn(
									"h-5 w-5",
									cpuStatus === "critical"
										? "text-red-500"
										: cpuStatus === "warning"
											? "text-amber-500"
											: "text-blue-500",
								)}
							/>
							<div
								class="absolute -right-1 -top-1 h-2 w-2 rounded-full border border-zinc-950 bg-current shadow-[0_0_8px_currentColor]"
							></div>
						</div>

						<div
							class="group relative cursor-help"
							title={`${t('dashboard.memory')}: ${u.mem.usedPercent.toFixed(0)}%`}
						>
							<MemoryStick
								class={cn(
									"h-5 w-5",
									ramStatus === "critical"
										? "text-red-500"
										: ramStatus === "warning"
											? "text-amber-500"
											: "text-purple-500",
								)}
							/>
							<div
								class="absolute -right-1 -top-1 h-2 w-2 rounded-full border border-zinc-950 bg-current shadow-[0_0_8px_currentColor]"
							></div>
						</div>

						{#if u.gpu}
							<div
								class="group relative cursor-help"
								title={`${t('dashboard.gpu') || 'GPU'}: ${u.gpu.percent.toFixed(0)}%`}
							>
								<Zap
									class={cn(
										"h-5 w-5",
										u.gpu.percent > 90
											? "text-red-500"
											: u.gpu.percent > 75
												? "text-amber-500"
												: "text-teal-500",
									)}
								/>
								<div
									class="absolute -right-1 -top-1 h-2 w-2 rounded-full border border-zinc-950 bg-current shadow-[0_0_8px_currentColor]"
								></div>
							</div>
						{/if}

						<div
							class="group relative cursor-help"
							title={t('dashboard.storage')}
						>
							<HardDrive class="h-5 w-5 text-zinc-400" />
							<div
								class="absolute -right-1 -top-1 h-2 w-2 rounded-full border border-zinc-950 bg-current shadow-[0_0_8px_currentColor]"
							></div>
						</div>

						<div
							class="group relative cursor-help"
							title={t('dashboard.network')}
						>
							<Activity class="h-5 w-5 text-cyan-500" />
							<div
								class="absolute -right-1 -top-1 h-2 w-2 rounded-full border border-zinc-950 bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.8)] animate-pulse"
							></div>
						</div>
					</div>
				{:else}
					<!-- CPU Widget -->
					<div
						class="flex flex-col gap-3 rounded-2xl border border-white/5 bg-white/[0.02] p-4 transition-colors hover:bg-white/[0.04]"
					>
						<MiniProgress
							value={u.cpu.percent}
							label={t('dashboard.cpu')}
							sublabel={`${u.cpu.percent.toFixed(1)}%`}
							icon={Cpu}
							status={cpuStatus}
							colorClass="bg-blue-500"
						/>
						<div
							class="h-8 w-full overflow-hidden rounded-lg bg-zinc-950/50"
						>
							<Sparkline
								value={u.cpu.percent}
								color={cpuStatus === "critical"
									? "var(--color-danger)"
									: cpuStatus === "warning"
										? "var(--color-warning)"
										: "#3b82f6"}
								height={32}
							/>
						</div>
						<div
							class="flex items-center justify-between text-[10px] font-bold uppercase tracking-widest text-zinc-600"
						>
							<span>{u.cpu.num} Cores</span>
							{#if u.cpu.temperature > 0}
								<span
									class={cn(
										"flex items-center gap-1",
										tempStatus === "critical"
											? "text-red-500"
											: tempStatus === "warning"
												? "text-amber-500"
												: "",
									)}
								>
									<Thermometer class="h-2.5 w-2.5" />
									{u.cpu.temperature}°C
								</span>
							{/if}
						</div>
					</div>

					<!-- RAM Widget -->
					<div
						class="flex flex-col gap-3 rounded-2xl border border-white/5 bg-white/[0.02] p-4 transition-colors hover:bg-white/[0.04]"
					>
						<MiniProgress
							value={u.mem.usedPercent}
							label={t('dashboard.memory')}
							sublabel={`${u.mem.usedPercent.toFixed(1)}%`}
							icon={MemoryStick}
							status={ramStatus}
							colorClass="bg-purple-500"
						/>
						<div
							class="h-8 w-full overflow-hidden rounded-lg bg-zinc-950/50"
						>
							<Sparkline
								value={u.mem.usedPercent}
								color={ramStatus === "critical"
									? "var(--color-danger)"
									: ramStatus === "warning"
										? "var(--color-warning)"
										: "#a855f7"}
								height={32}
							/>
						</div>
						<div
							class="flex items-center justify-between text-[10px] font-bold uppercase tracking-widest text-zinc-600"
						>
							<span>{formatSize(u.mem.used)}</span>
							<span>{formatSize(u.mem.total)}</span>
						</div>
					</div>

					<!-- GPU Widget -->
					{#if u.gpu}
						<div
							class="flex flex-col gap-3 rounded-2xl border border-white/5 bg-white/[0.02] p-4 transition-colors hover:bg-white/[0.04]"
						>
							<MiniProgress
								value={u.gpu.percent}
								label={t('dashboard.gpu') || 'GPU'}
								sublabel={`${u.gpu.percent.toFixed(1)}%`}
								icon={Zap}
								status={u.gpu.percent > 90
									? "critical"
									: u.gpu.percent > 75
										? "warning"
										: "normal"}
								colorClass="bg-teal-500"
							/>
							<div
								class="h-8 w-full overflow-hidden rounded-lg bg-zinc-950/50"
							>
								<Sparkline
									value={u.gpu.percent}
									color={u.gpu.percent > 90
										? "var(--color-danger)"
										: u.gpu.percent > 75
											? "var(--color-warning)"
											: "#14b8a6"}
									height={32}
								/>
							</div>
							<div
								class="flex items-center justify-between text-[10px] font-bold uppercase tracking-widest text-zinc-600"
							>
								<span class="truncate max-w-[100px]"
									>{u.gpu.model}</span
								>
								<span>{formatSize(u.gpu.memoryUsed)}</span>
							</div>
						</div>
					{/if}

					<!-- Network Widget -->
					<div
						class="flex flex-col gap-3 rounded-2xl border border-white/5 bg-white/[0.02] p-4 transition-colors hover:bg-white/[0.04]"
					>
						<div
							class="flex items-center justify-between text-[10px] font-bold uppercase tracking-widest"
						>
							<div class="flex items-center gap-2 text-zinc-500">
								<Activity class="h-3 w-3" />
								<span>{t('dashboard.network')}</span>
							</div>
							<div class="flex items-center gap-3">
								<span class="text-cyan-500">↓</span>
								<span class="text-amber-500">↑</span>
							</div>
						</div>
						<div
							class="relative h-12 w-full overflow-hidden rounded-lg bg-zinc-950/50"
						>
							<div class="absolute inset-0 z-10 opacity-60">
								<Sparkline
									value={u.netInRate || 0}
									color="#06b6d4"
									height={48}
								/>
							</div>
							<div class="absolute inset-0 z-10 opacity-60">
								<Sparkline
									value={u.netOutRate || 0}
									color="#f59e0b"
									height={48}
								/>
							</div>
						</div>
						<div
							class="flex justify-between text-[10px] font-bold tabular-nums"
						>
							<span class="text-cyan-500"
								>{formatSize(u.netInRate || 0)}/s</span
							>
							<span class="text-amber-500"
								>{formatSize(u.netOutRate || 0)}/s</span
							>
						</div>
					</div>

					<!-- Storage Widget -->
					{#if store.disks.length > 0}
						{@const mainDisk = store.disks[0]}
						<div
							class="flex flex-col gap-3 rounded-2xl border border-white/5 bg-white/[0.02] p-4 transition-colors hover:bg-white/[0.04]"
						>
							<MiniProgress
								value={mainDisk.usedPercent}
								label={t('dashboard.storage')}
								sublabel={`${mainDisk.usedPercent.toFixed(0)}%`}
								icon={HardDrive}
								status={mainDisk.usedPercent > 90
									? "critical"
									: mainDisk.usedPercent > 75
										? "warning"
										: "normal"}
								colorClass="bg-zinc-500"
							/>
							<div
								class="flex items-center justify-between text-[10px] font-bold uppercase tracking-widest text-zinc-600"
							>
								<span class="truncate max-w-[80px]"
									>{mainDisk.path}</span
								>
								<span
									>{formatSize(
										mainDisk.total - mainDisk.used,
									)} {t('sidebar.free')}</span
								>
							</div>
						</div>
					{/if}
				{/if}
			</div>
		{/if}
	</div>

	<!-- Utility Menu (OS-style) -->
	<div class="shrink-0 border-t border-zinc-900/50 p-3 flex flex-col gap-2">
		<!-- Quick actions row -->
		<div
			class={cn(
				"flex items-center justify-between text-zinc-500",
				isCollapsed ? "flex-col gap-4" : "px-1",
			)}
		>
			<button
				class="flex h-8 w-8 items-center justify-center rounded-lg transition-colors hover:bg-zinc-800 hover:text-zinc-300"
				title={t('sidebar.profile')}
				aria-label={t('sidebar.profile')}
			>
				<User class="h-[18px] w-[18px]" />
			</button>
			{#if !isCollapsed}
				<a
					href="/settings"
					class="flex h-8 w-8 items-center justify-center rounded-lg transition-colors hover:bg-zinc-800 hover:text-zinc-300"
					title={t('nav.settings')}
					aria-label={t('nav.settings')}
				>
					<Settings class="h-[15px] w-[15px]" />
				</a>
			{/if}
			<button
				class="flex h-8 w-8 items-center justify-center rounded-lg transition-colors hover:bg-zinc-800 hover:text-zinc-300"
				title={t('sidebar.hostTerminal')}
				aria-label={t('sidebar.hostTerminal')}
				onclick={openTerminalFeature}
			>
				<Terminal class="h-[18px] w-[18px]" />
			</button>
			<button
				class="flex h-8 w-8 items-center justify-center rounded-lg transition-colors hover:bg-zinc-800 hover:text-zinc-300"
				title={isDark ? t('sidebar.switchToLight') : t('sidebar.switchToDark')}
				onclick={toggleTheme}
			>
				{#if isDark}
					<Sun class="h-[18px] w-[18px]" />
				{:else}
					<Moon class="h-[18px] w-[18px]" />
				{/if}
			</button>
			<button
				onclick={() => auth.logout()}
				class="flex h-8 w-8 items-center justify-center rounded-lg transition-colors hover:bg-red-500/10 hover:text-red-400"
				title={t('sidebar.logout')}
				aria-label={t('sidebar.logout')}
			>
				<LogOut class="h-[18px] w-[18px]" />
			</button>
		</div>

		{#if !isCollapsed}
			<!-- Status bar -->
			<div
				class="flex items-center justify-between rounded-xl border border-zinc-900 bg-zinc-900/30 px-3 py-2"
			>
				<div class="flex items-center gap-2">
					<div
						class="h-1.5 w-1.5 rounded-full bg-emerald-500 shadow-[0_0_6px_rgba(16,185,129,0.5)]"
					></div>
					<div
						class="text-[10px] font-bold tracking-widest text-zinc-600 uppercase"
					>
						{t('sidebar.systemOnline')}
					</div>
				</div>
			</div>
		{/if}
	</div>
</aside>

<!-- Local pty terminal — no SSH modal, no credentials needed -->
<TerminalComponent
	bind:isOpen={ui.isTerminalOpen}
	onClose={() => (ui.isTerminalOpen = false)}
/>

<style>
	/* Hide scrollbar for seamless OS feel */
	.no-scrollbar::-webkit-scrollbar {
		display: none;
	}
	.no-scrollbar {
		-ms-overflow-style: none;
		scrollbar-width: none;
	}
</style>
