<script lang="ts">
	import { Plus, X, Box, Network, HardDrive, Shield, AlertTriangle, Check } from 'lucide-svelte';
	import { cn } from '$lib/utils';
	import { checkPorts } from '$lib/api/apps';
	import { t } from '$lib/i18n/index.svelte';
	import {
		viewFromYaml,
		setProjectName,
		setServiceName,
		setImage,
		setContainerName,
		setIcon,
		setWebPort,
		setNetwork,
		setCommand,
		setUser,
		setWorkingDir,
		setPrivileged,
		setRestart,
		setMemLimitMb,
		addPort,
		setPortAt,
		addVolume,
		setVolumeAt,
		addDevice,
		setDeviceAt,
		removeArrayItem,
		addEnv,
		setEnvAt,
		removeEnvAt,
		addLabel,
		setLabelAt,
		removeLabelAt
	} from '$lib/utils/compose-mutate';

	/**
	 * One-way Custom App form (per ADR-0030 + #374 / #375 patterns audit).
	 *
	 * Props:
	 *   yaml       — the compose YAML string, sole source of truth.
	 *   onChange   — emit a new YAML on every field edit; the parent
	 *                state updates and the form re-renders via $derived.
	 *   nameError  — externally-validated name issue (i18n string).
	 *
	 * No $bindable, no shared mutable model. Every edit goes through
	 * a pure mutator in compose-mutate.ts that preserves the YAML's
	 * existing shape (long-form volume/port entries stay long-form).
	 * Round-trip is byte-stable for shape; comments are stripped on
	 * write (documented trade-off).
	 */

	let {
		yaml,
		onChange,
		nameError = null
	}: {
		yaml: string;
		onChange: (newYaml: string) => void;
		nameError?: string | null;
	} = $props();

	const view = $derived(viewFromYaml(yaml));

	// ── Port availability validator (debounced) ────────────────────────────
	let portStatus = $state<Record<string, 'free' | 'inuse' | 'unknown'>>({});
	let portSuggestion = $state<Record<string, number>>({});
	let portCheckTimer: ReturnType<typeof setTimeout> | null = null;

	function refreshPortChecks() {
		if (portCheckTimer) clearTimeout(portCheckTimer);
		portCheckTimer = setTimeout(async () => {
			const wanted = new Set<string>();
			if (view.webPort.trim()) wanted.add(view.webPort.trim());
			for (const p of view.ports) {
				if (p.host.trim()) wanted.add(p.host.trim());
			}
			if (wanted.size === 0) {
				portStatus = {};
				portSuggestion = {};
				return;
			}
			try {
				const res = await checkPorts([...wanted]);
				const status: Record<string, 'free' | 'inuse' | 'unknown'> = {};
				for (const [port, available] of Object.entries(res.data)) {
					status[port] = available ? 'free' : 'inuse';
				}
				portStatus = status;
				portSuggestion = res.suggestions ?? {};
			} catch {
				portStatus = {};
				portSuggestion = {};
			}
		}, 350);
	}

	$effect(() => {
		// Re-check whenever the host ports referenced by the form change.
		void view.webPort;
		void view.ports.map((p) => p.host).join(',');
		refreshPortChecks();
	});

	function statusFor(p: string): 'free' | 'inuse' | 'unknown' {
		if (!p?.trim()) return 'unknown';
		return portStatus[p.trim()] ?? 'unknown';
	}

	function suggestionFor(p: string): number | null {
		return portSuggestion[p.trim()] ?? null;
	}

	function applySuggestion(target: 'web' | number, port: string) {
		const next = suggestionFor(port);
		if (next == null) return;
		if (target === 'web') {
			onChange(setWebPort(yaml, String(next)));
		} else {
			onChange(setPortAt(yaml, target, 'host', String(next)));
		}
	}
</script>

<div class="space-y-10 pb-20">
	<!-- Header Section: Service Identity -->
	<section class="space-y-6">
		<div class="grid grid-cols-1 gap-4 md:grid-cols-2">
			<div>
				<label
					for="service-name"
					class="mb-2 block text-[10px] font-bold uppercase tracking-widest text-zinc-500"
					>{t('form.serviceName')}</label
				>
				<input
					id="service-name"
					type="text"
					value={view.serviceName}
					oninput={(e) => onChange(setServiceName(yaml, (e.target as HTMLInputElement).value))}
					placeholder="e.g. web, db, app"
					aria-invalid={nameError ? 'true' : 'false'}
					aria-describedby={nameError ? 'service-name-error' : undefined}
					class={cn(
						'w-full rounded-xl border bg-white/[0.03] p-3 text-sm font-mono text-white outline-none transition-all',
						nameError
							? 'border-red-500/60 focus:border-red-500 focus:bg-red-500/[0.05]'
							: 'border-white/5 focus:border-emerald-500/50 focus:bg-white/[0.05]'
					)}
				/>
				{#if nameError}
					<p
						id="service-name-error"
						data-testid="service-name-error"
						class="mt-1.5 text-xs text-red-400"
					>
						{nameError}
					</p>
				{/if}
			</div>
			<div>
				<label
					for="container-name"
					class="mb-2 block text-[10px] font-bold uppercase tracking-widest text-zinc-500"
					>{t('form.containerName')}</label
				>
				<input
					id="container-name"
					type="text"
					value={view.containerName}
					oninput={(e) => onChange(setContainerName(yaml, (e.target as HTMLInputElement).value))}
					placeholder="e.g. my-app-container"
					class="w-full rounded-xl border border-white/5 bg-white/[0.03] p-3 text-sm text-white outline-none focus:border-emerald-500/50 focus:bg-white/[0.05] transition-all"
				/>
			</div>
		</div>

		<div class="grid grid-cols-1 gap-4 md:grid-cols-4">
			<div class="md:col-span-3">
				<label
					for="docker-image"
					class="mb-2 block text-[10px] font-bold uppercase tracking-widest text-zinc-500"
					>{t('form.dockerImage')}</label
				>
				<div class="relative">
					<input
						id="docker-image"
						type="text"
						value={view.image}
						oninput={(e) => onChange(setImage(yaml, (e.target as HTMLInputElement).value))}
						placeholder="e.g. nginx"
						class="w-full rounded-xl border border-white/5 bg-white/[0.03] p-3 text-sm text-white outline-none focus:border-emerald-500/50 focus:bg-white/[0.05] transition-all"
					/>
					{#if view.image.length > 2}
						<div
							class="absolute right-3 top-3 h-2 w-2 rounded-full bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.5)]"
						></div>
					{/if}
				</div>
			</div>

			<div>
				<label
					for="web-port"
					class="mb-2 block text-[10px] font-bold uppercase tracking-widest text-zinc-500"
					>{t('form.webPort')}</label
				>
				<div class="relative">
					<input
						id="web-port"
						type="text"
						value={view.webPort}
						oninput={(e) => onChange(setWebPort(yaml, (e.target as HTMLInputElement).value))}
						placeholder="e.g. 8080"
						class={cn(
							'w-full rounded-xl border bg-white/[0.03] p-3 text-sm text-white outline-none focus:bg-white/[0.05] transition-all',
							statusFor(view.webPort) === 'inuse'
								? 'border-amber-500/40 focus:border-amber-500/60'
								: 'border-white/5 focus:border-emerald-500/50'
						)}
					/>
					{#if statusFor(view.webPort) === 'free'}
						<Check class="absolute right-3 top-3 h-4 w-4 text-emerald-500" strokeWidth={2.5} />
					{:else if statusFor(view.webPort) === 'inuse'}
						<AlertTriangle
							class="absolute right-3 top-3 h-4 w-4 text-amber-500"
							strokeWidth={2.5}
						/>
					{/if}
				</div>
				{#if statusFor(view.webPort) === 'inuse'}
					<button
						type="button"
						onclick={() => applySuggestion('web', view.webPort)}
						class="mt-1 text-[9px] font-semibold text-amber-400 hover:text-amber-300"
					>
						{t('form.portInUse', { port: view.webPort })}{#if suggestionFor(view.webPort)} —
							{t('form.useSuggestion', {
								suggestion: String(suggestionFor(view.webPort))
							})}{/if}
					</button>
				{:else}
					<p class="mt-1 text-[9px] text-zinc-600">{t('form.webPortDesc')}</p>
				{/if}
			</div>
		</div>

		<div>
			<label
				for="app-icon"
				class="mb-2 block text-[10px] font-bold uppercase tracking-widest text-zinc-500"
				>{t('form.appIcon')}</label
			>
			<div class="flex gap-3">
				<div
					class="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl border border-white/5 bg-white/[0.03] overflow-hidden"
				>
					{#if view.icon}
						<img src={view.icon} alt="icon preview" class="h-8 w-8 object-contain" />
					{:else}
						<Box class="h-5 w-5 text-zinc-600" />
					{/if}
				</div>
				<input
					id="app-icon"
					type="text"
					value={view.icon}
					oninput={(e) => onChange(setIcon(yaml, (e.target as HTMLInputElement).value))}
					placeholder="https://..."
					class="w-full rounded-xl border border-white/5 bg-white/[0.03] p-3 text-sm text-white outline-none focus:border-emerald-500/50 transition-all"
				/>
			</div>
		</div>
	</section>

	<hr class="border-white/5" />

	<!-- Network Section -->
	<section class="space-y-6">
		<div class="flex items-center gap-3">
			<div class="flex h-8 w-8 items-center justify-center rounded-lg bg-blue-500/10 text-blue-400">
				<Network class="h-4 w-4" />
			</div>
			<h2 class="text-sm font-bold text-white">{t('form.networkSettings')}</h2>
		</div>

		<div class="grid grid-cols-1 gap-6 md:grid-cols-2">
			<div>
				<label
					for="network-mode"
					class="mb-2 block text-[10px] font-bold uppercase tracking-widest text-zinc-500"
					>{t('form.networkMode')}</label
				>
				<select
					id="network-mode"
					value={view.network}
					onchange={(e) => onChange(setNetwork(yaml, (e.target as HTMLSelectElement).value))}
					class="w-full rounded-xl border border-white/5 bg-white/[0.03] p-3 text-sm text-white outline-none focus:border-emerald-500/50 transition-all"
				>
					<option value="bridge">{t('form.bridge')}</option>
					<option value="host">{t('form.host')}</option>
					<option value="none">{t('form.none')}</option>
				</select>
			</div>
		</div>

		<div class="space-y-4">
			<div class="flex items-center justify-between">
				<p class="text-[10px] font-bold uppercase tracking-widest text-zinc-500">
					{t('form.portMapping')}
				</p>
				<button
					onclick={() => onChange(addPort(yaml))}
					class="flex items-center gap-1 text-[10px] font-bold uppercase text-emerald-500 hover:text-emerald-400"
				>
					<Plus class="h-3 w-3" />
					{t('form.addPort')}
				</button>
			</div>

			<div class="space-y-2">
				{#each view.ports as port, i (i)}
					{@const hostStatus = statusFor(port.host)}
					{@const hostSuggestion = suggestionFor(port.host)}
					<div>
						<div class="grid grid-cols-7 gap-2">
							<div class="relative col-span-3">
								<input
									type="text"
									value={port.host}
									oninput={(e) =>
										onChange(setPortAt(yaml, i, 'host', (e.target as HTMLInputElement).value))}
									placeholder={t('form.host')}
									aria-label={t('form.host')}
									class={cn(
										'w-full rounded-lg border bg-white/[0.02] p-2 pr-7 text-xs text-white outline-none',
										hostStatus === 'inuse'
											? 'border-amber-500/40 focus:border-amber-500/60'
											: 'border-white/5 focus:border-emerald-500/30'
									)}
								/>
								{#if hostStatus === 'free'}
									<Check
										class="absolute right-2 top-2 h-3.5 w-3.5 text-emerald-500"
										strokeWidth={2.5}
									/>
								{:else if hostStatus === 'inuse'}
									<AlertTriangle
										class="absolute right-2 top-2 h-3.5 w-3.5 text-amber-500"
										strokeWidth={2.5}
									/>
								{/if}
							</div>
							<div class="flex items-center justify-center text-zinc-700">:</div>
							<input
								type="text"
								value={port.container}
								oninput={(e) =>
									onChange(setPortAt(yaml, i, 'container', (e.target as HTMLInputElement).value))}
								placeholder="Container"
								aria-label="Container port"
								class="col-span-2 rounded-lg border border-white/5 bg-white/[0.02] p-2 text-xs text-white outline-none focus:border-emerald-500/30"
							/>
							<button
								onclick={() => onChange(removeArrayItem(yaml, 'ports', i))}
								aria-label={t('action.delete')}
								class="flex items-center justify-center text-zinc-600 hover:text-red-500"
							>
								<X class="h-3.5 w-3.5" />
							</button>
						</div>
						{#if hostStatus === 'inuse'}
							<button
								type="button"
								onclick={() => applySuggestion(i, port.host)}
								class="mt-1 text-[9px] font-semibold text-amber-400 hover:text-amber-300"
							>
								{t('form.portInUse', { port: port.host })}{#if hostSuggestion} —
									{t('form.useSuggestion', { suggestion: String(hostSuggestion) })}{/if}
							</button>
						{/if}
					</div>
				{/each}
			</div>
		</div>
	</section>

	<hr class="border-white/5" />

	<!-- Storage Section -->
	<section class="space-y-6">
		<div class="flex items-center gap-3">
			<div
				class="flex h-8 w-8 items-center justify-center rounded-lg bg-amber-500/10 text-amber-400"
			>
				<HardDrive class="h-4 w-4" />
			</div>
			<h2 class="text-sm font-bold text-white">{t('form.storageAndDevices')}</h2>
		</div>

		<div class="space-y-4">
			<div class="flex items-center justify-between">
				<p class="text-[10px] font-bold uppercase tracking-widest text-zinc-500">
					{t('form.volumeMounts')}
				</p>
				<button
					onclick={() => onChange(addVolume(yaml))}
					class="flex items-center gap-1 text-[10px] font-bold uppercase text-emerald-500 hover:text-emerald-400"
				>
					<Plus class="h-3 w-3" />
					{t('form.addVolume')}
				</button>
			</div>

			<div class="space-y-2">
				{#each view.volumes as vol, i (i)}
					<div class="grid grid-cols-7 gap-2">
						<input
							type="text"
							value={vol.host}
							oninput={(e) =>
								onChange(setVolumeAt(yaml, i, 'host', (e.target as HTMLInputElement).value))}
							placeholder="Host Path"
							aria-label="Host path"
							class="col-span-3 rounded-lg border border-white/5 bg-white/[0.02] p-2 text-xs text-white outline-none focus:border-emerald-500/30"
						/>
						<div class="flex items-center justify-center text-zinc-700">→</div>
						<input
							type="text"
							value={vol.container}
							oninput={(e) =>
								onChange(setVolumeAt(yaml, i, 'container', (e.target as HTMLInputElement).value))}
							placeholder="Container Path"
							aria-label="Container path"
							class="col-span-2 rounded-lg border border-white/5 bg-white/[0.02] p-2 text-xs text-white outline-none focus:border-emerald-500/30"
						/>
						<button
							onclick={() => onChange(removeArrayItem(yaml, 'volumes', i))}
							aria-label={t('action.delete')}
							class="flex items-center justify-center text-zinc-600 hover:text-red-500"
						>
							<X class="h-3.5 w-3.5" />
						</button>
					</div>
				{/each}
			</div>
		</div>

		<div class="space-y-4">
			<div class="flex items-center justify-between">
				<p class="text-[10px] font-bold uppercase tracking-widest text-zinc-500">
					{t('form.devices')}
				</p>
				<button
					onclick={() => onChange(addDevice(yaml))}
					class="flex items-center gap-1 text-[10px] font-bold uppercase text-emerald-500 hover:text-emerald-400"
				>
					<Plus class="h-3 w-3" />
					{t('form.addDevice')}
				</button>
			</div>

			<div class="space-y-2">
				{#each view.devices as dev, i (i)}
					<div class="grid grid-cols-7 gap-2">
						<input
							type="text"
							value={dev.host}
							oninput={(e) =>
								onChange(setDeviceAt(yaml, i, 'host', (e.target as HTMLInputElement).value))}
							placeholder="/dev/ttyUSB0"
							aria-label="Host device path"
							class="col-span-3 rounded-lg border border-white/5 bg-white/[0.02] p-2 text-xs text-white outline-none focus:border-emerald-500/30"
						/>
						<div class="flex items-center justify-center text-zinc-700">:</div>
						<input
							type="text"
							value={dev.container}
							oninput={(e) =>
								onChange(setDeviceAt(yaml, i, 'container', (e.target as HTMLInputElement).value))}
							placeholder="/dev/ttyUSB0"
							aria-label="Container device path"
							class="col-span-2 rounded-lg border border-white/5 bg-white/[0.02] p-2 text-xs text-white outline-none focus:border-emerald-500/30"
						/>
						<button
							onclick={() => onChange(removeArrayItem(yaml, 'devices', i))}
							aria-label={t('action.delete')}
							class="flex items-center justify-center text-zinc-600 hover:text-red-500"
						>
							<X class="h-3.5 w-3.5" />
						</button>
					</div>
				{/each}
			</div>
		</div>
	</section>

	<hr class="border-white/5" />

	<!-- Execution & Resources Section -->
	<section class="space-y-8">
		<div class="flex items-center gap-3">
			<div
				class="flex h-8 w-8 items-center justify-center rounded-lg bg-purple-500/10 text-purple-400"
			>
				<Shield class="h-4 w-4" />
			</div>
			<h2 class="text-sm font-bold text-white">{t('form.executionAndResources')}</h2>
		</div>

		<div>
			<label
				for="container-command"
				class="mb-4 block text-[10px] font-bold uppercase tracking-widest text-zinc-500"
				>{t('form.command')}</label
			>
			<input
				id="container-command"
				type="text"
				value={view.command}
				oninput={(e) => onChange(setCommand(yaml, (e.target as HTMLInputElement).value))}
				placeholder="e.g. npm start"
				class="w-full rounded-xl border border-white/5 bg-white/[0.03] p-3 text-sm font-mono text-zinc-300 outline-none focus:border-emerald-500/50 transition-all"
			/>
		</div>

		<div class="grid grid-cols-1 gap-4 md:grid-cols-2">
			<div>
				<label
					for="run-as-user"
					class="mb-4 block text-[10px] font-bold uppercase tracking-widest text-zinc-500"
					>{t('form.runAsUser')}</label
				>
				<input
					id="run-as-user"
					type="text"
					value={view.user}
					oninput={(e) => onChange(setUser(yaml, (e.target as HTMLInputElement).value))}
					placeholder="e.g. 1000:1000"
					class="w-full rounded-xl border border-white/5 bg-white/[0.03] p-3 text-sm font-mono text-zinc-300 outline-none focus:border-emerald-500/50 transition-all"
				/>
			</div>
			<div>
				<label
					for="working-dir"
					class="mb-4 block text-[10px] font-bold uppercase tracking-widest text-zinc-500"
					>{t('form.workingDir')}</label
				>
				<input
					id="working-dir"
					type="text"
					value={view.workingDir}
					oninput={(e) => onChange(setWorkingDir(yaml, (e.target as HTMLInputElement).value))}
					placeholder="e.g. /app"
					class="w-full rounded-xl border border-white/5 bg-white/[0.03] p-3 text-sm font-mono text-zinc-300 outline-none focus:border-emerald-500/50 transition-all"
				/>
			</div>
		</div>

		<div
			class="flex items-center justify-between rounded-2xl border border-white/5 bg-white/[0.02] p-4"
		>
			<div>
				<h3 class="text-sm font-bold text-white">{t('form.privilegedMode')}</h3>
				<p class="text-[10px] text-zinc-500">{t('form.privilegedDesc')}</p>
			</div>
			<button
				onclick={() => onChange(setPrivileged(yaml, !view.privileged))}
				aria-label={view.privileged ? 'Disable privileged mode' : 'Enable privileged mode'}
				aria-pressed={view.privileged}
				class={cn(
					'h-6 w-11 rounded-full p-1 transition-colors duration-300',
					view.privileged ? 'bg-emerald-500' : 'bg-zinc-800'
				)}
			>
				<div
					class={cn(
						'h-4 w-4 rounded-full bg-white transition-transform duration-300',
						view.privileged ? 'translate-x-5' : 'translate-x-0'
					)}
				></div>
			</button>
		</div>

		<div class="space-y-4">
			<div class="flex items-center justify-between">
				<label
					for="mem-limit"
					class="text-[10px] font-bold uppercase tracking-widest text-zinc-500"
					>{t('form.memLimit')}</label
				>
				<span class="text-xs font-bold text-emerald-500">{view.memLimitMb}M</span>
			</div>
			<input
				id="mem-limit"
				type="range"
				min="128"
				max="8192"
				step="128"
				value={view.memLimitMb}
				oninput={(e) =>
					onChange(setMemLimitMb(yaml, parseInt((e.target as HTMLInputElement).value, 10) || 512))}
				class="h-1.5 w-full appearance-none rounded-lg bg-zinc-800 accent-emerald-500"
			/>
			<div class="flex justify-between text-[10px] font-bold text-zinc-600">
				<span>128MB</span>
				<span>1GB</span>
				<span>2GB</span>
				<span>4GB</span>
				<span>8GB</span>
			</div>
		</div>

		<div>
			<p class="mb-2 text-[10px] font-bold uppercase tracking-widest text-zinc-500">
				{t('form.restartPolicy')}
			</p>
			<div class="flex flex-wrap gap-2">
				{#each ['no', 'always', 'unless-stopped', 'on-failure'] as policy (policy)}
					<button
						onclick={() => onChange(setRestart(yaml, policy))}
						class={cn(
							'rounded-lg border px-3 py-1.5 text-[10px] font-bold uppercase transition-all',
							view.restart === policy
								? 'border-emerald-500/50 bg-emerald-500/10 text-emerald-500 shadow-[0_0_12px_rgba(16,185,129,0.1)]'
								: 'border-white/5 bg-white/[0.02] text-zinc-500 hover:border-white/20'
						)}
					>
						{policy}
					</button>
				{/each}
			</div>
		</div>

		<div class="space-y-4">
			<div class="flex items-center justify-between">
				<p class="text-[10px] font-bold uppercase tracking-widest text-zinc-500">
					{t('form.envVars')}
				</p>
				<button
					onclick={() => onChange(addEnv(yaml))}
					class="flex items-center gap-1 text-[10px] font-bold uppercase text-emerald-500 hover:text-emerald-400"
				>
					<Plus class="h-3 w-3" />
					{t('form.addVar')}
				</button>
			</div>

			<div class="space-y-2">
				{#each view.env as env, i (i)}
					<div class="grid grid-cols-7 gap-2">
						<input
							type="text"
							value={env.key}
							oninput={(e) =>
								onChange(setEnvAt(yaml, i, 'key', (e.target as HTMLInputElement).value))}
							placeholder="KEY"
							aria-label="Environment variable key"
							class="col-span-3 rounded-lg border border-white/5 bg-white/[0.02] p-2 text-xs font-mono text-white outline-none focus:border-emerald-500/30"
						/>
						<div class="flex items-center justify-center text-zinc-700">=</div>
						<input
							type="text"
							value={env.value}
							oninput={(e) =>
								onChange(setEnvAt(yaml, i, 'value', (e.target as HTMLInputElement).value))}
							placeholder="VALUE"
							aria-label="Environment variable value"
							class="col-span-2 rounded-lg border border-white/5 bg-white/[0.02] p-2 text-xs font-mono text-white outline-none focus:border-emerald-500/30"
						/>
						<button
							onclick={() => onChange(removeEnvAt(yaml, i))}
							aria-label={t('action.delete')}
							class="flex items-center justify-center text-zinc-600 hover:text-red-500"
						>
							<X class="h-3.5 w-3.5" />
						</button>
					</div>
				{/each}
			</div>
		</div>

		<div class="space-y-4">
			<div class="flex items-center justify-between">
				<p class="text-[10px] font-bold uppercase tracking-widest text-zinc-500">
					{t('form.labels')}
				</p>
				<button
					onclick={() => onChange(addLabel(yaml))}
					class="flex items-center gap-1 text-[10px] font-bold uppercase text-emerald-500 hover:text-emerald-400"
				>
					<Plus class="h-3 w-3" />
					{t('form.addLabel')}
				</button>
			</div>

			<div class="space-y-2">
				{#each view.labels as label, i (i)}
					<div class="grid grid-cols-7 gap-2">
						<input
							type="text"
							value={label.key}
							oninput={(e) =>
								onChange(setLabelAt(yaml, i, 'key', (e.target as HTMLInputElement).value))}
							placeholder="Label Key"
							aria-label="Label key"
							class="col-span-3 rounded-lg border border-white/5 bg-white/[0.02] p-2 text-xs text-white outline-none focus:border-emerald-500/30"
						/>
						<div class="flex items-center justify-center text-zinc-700">=</div>
						<input
							type="text"
							value={label.value}
							oninput={(e) =>
								onChange(setLabelAt(yaml, i, 'value', (e.target as HTMLInputElement).value))}
							placeholder="Value"
							aria-label="Label value"
							class="col-span-2 rounded-lg border border-white/5 bg-white/[0.02] p-2 text-xs text-white outline-none focus:border-emerald-500/30"
						/>
						<button
							onclick={() => onChange(removeLabelAt(yaml, i))}
							aria-label={t('action.delete')}
							class="flex items-center justify-center text-zinc-600 hover:text-red-500"
						>
							<X class="h-3.5 w-3.5" />
						</button>
					</div>
				{/each}
			</div>
		</div>
	</section>
</div>
