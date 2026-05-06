<script lang="ts">
	import { Plus, X, Box, Network, HardDrive, Shield, AlertTriangle, Check } from 'lucide-svelte';
	import { cn } from '$lib/utils';
	import { checkPorts } from '$lib/api/apps';

	export interface ComposeModel {
		name: string;
		container_name: string;
		image: string;
		icon: string;
		network: string;
		ports: { host: string; container: string }[];
		volumes: { host: string; container: string }[];
		devices: { host: string; container: string }[];
		env: { key: string; value: string }[];
		labels: { key: string; value: string }[];
		restart: string;
		command: string;
		user: string;
		working_dir: string;
		privileged: boolean;
		mem_limit: string;
		mem_limit_num: number;
		web_port: string;
	}

	let { model = $bindable(), onchange }: {
		model: ComposeModel;
		onchange: () => void;
	} = $props();

	function addPort() {
		model.ports = [...model.ports, { host: '', container: '' }];
		onchange();
	}

	function removePort(index: number) {
		model.ports = model.ports.filter((_, i) => i !== index);
		onchange();
	}

	// ── Port availability validator (debounced) ────────────────────────────────
	// portStatus: "8080" → 'free' | 'inuse' | 'unknown'
	// portSuggestion: "8080" → 8081 (next available)
	let portStatus = $state<Record<string, 'free' | 'inuse' | 'unknown'>>({});
	let portSuggestion = $state<Record<string, number>>({});
	let portCheckTimer: ReturnType<typeof setTimeout> | null = null;

	function refreshPortChecks() {
		if (portCheckTimer) clearTimeout(portCheckTimer);
		portCheckTimer = setTimeout(async () => {
			const wanted = new Set<string>();
			if (model.web_port?.trim()) wanted.add(model.web_port.trim());
			for (const p of model.ports) {
				if (p.host?.trim()) wanted.add(p.host.trim());
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
				// Network error — leave status as 'unknown'
				portStatus = {};
				portSuggestion = {};
			}
		}, 350);
	}

	// Re-check whenever the user changes a port input.
	$effect(() => {
		// Reactive read so the effect re-runs when ports change.
		void model.web_port;
		void model.ports.map(p => p.host).join(',');
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
			model.web_port = String(next);
		} else {
			model.ports[target].host = String(next);
		}
		onchange();
	}

	function addVolume() {
		model.volumes = [...model.volumes, { host: '', container: '' }];
		onchange();
	}

	function removeVolume(index: number) {
		model.volumes = model.volumes.filter((_, i) => i !== index);
		onchange();
	}

	function addEnv() {
		model.env = [...model.env, { key: '', value: '' }];
		onchange();
	}

	function removeEnv(index: number) {
		model.env = model.env.filter((_, i) => i !== index);
		onchange();
	}

	function addDevice() {
		model.devices = [...model.devices, { host: '', container: '' }];
		onchange();
	}

	function removeDevice(index: number) {
		model.devices = model.devices.filter((_, i) => i !== index);
		onchange();
	}
</script>

<div class="space-y-10 pb-20">
	<!-- Header Section: Service Identity -->
	<section class="space-y-6">
		<div class="grid grid-cols-1 gap-4 md:grid-cols-2">
			<div>
				<label for="service-name" class="mb-2 block text-[10px] font-bold uppercase tracking-widest text-zinc-500">Service Name (YAML Key) *</label>
				<input
					id="service-name"
					type="text"
					bind:value={model.name}
					oninput={onchange}
					placeholder="e.g. web, db, app"
					class="w-full rounded-xl border border-white/5 bg-white/[0.03] p-3 text-sm font-mono text-white outline-none focus:border-emerald-500/50 focus:bg-white/[0.05] transition-all"
				/>
			</div>
			<div>
				<label for="container-name" class="mb-2 block text-[10px] font-bold uppercase tracking-widest text-zinc-500">Container Name (Optional)</label>
				<input
					id="container-name"
					type="text"
					bind:value={model.container_name}
					oninput={onchange}
					placeholder="e.g. my-app-container"
					class="w-full rounded-xl border border-white/5 bg-white/[0.03] p-3 text-sm text-white outline-none focus:border-emerald-500/50 focus:bg-white/[0.05] transition-all"
				/>
			</div>
		</div>

		<div class="grid grid-cols-1 gap-4 md:grid-cols-4">
			<div class="md:col-span-3">
				<label for="docker-image" class="mb-2 block text-[10px] font-bold uppercase tracking-widest text-zinc-500">Docker Image</label>
				<div class="relative">
					<input
						id="docker-image"
						type="text"
						bind:value={model.image}
						oninput={onchange}
						placeholder="e.g. nginx"
						class="w-full rounded-xl border border-white/5 bg-white/[0.03] p-3 text-sm text-white outline-none focus:border-emerald-500/50 focus:bg-white/[0.05] transition-all"
					/>
					{#if model.image.length > 2}
						<div class="absolute right-3 top-3 h-2 w-2 rounded-full bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.5)]"></div>
					{/if}
				</div>
			</div>

			<div>
				<label for="web-port" class="mb-2 block text-[10px] font-bold uppercase tracking-widest text-zinc-500">Web UI Port (Host Port)</label>
				<div class="relative">
					<input
						id="web-port"
						type="text"
						bind:value={model.web_port}
						oninput={onchange}
						placeholder="e.g. 8080"
						class={cn(
							"w-full rounded-xl border bg-white/[0.03] p-3 text-sm text-white outline-none focus:bg-white/[0.05] transition-all",
							statusFor(model.web_port) === 'inuse' ? 'border-amber-500/40 focus:border-amber-500/60' : 'border-white/5 focus:border-emerald-500/50'
						)}
					/>
					{#if statusFor(model.web_port) === 'free'}
						<Check class="absolute right-3 top-3 h-4 w-4 text-emerald-500" strokeWidth={2.5} />
					{:else if statusFor(model.web_port) === 'inuse'}
						<AlertTriangle class="absolute right-3 top-3 h-4 w-4 text-amber-500" strokeWidth={2.5} />
					{/if}
				</div>
				{#if statusFor(model.web_port) === 'inuse'}
					<button
						type="button"
						onclick={() => applySuggestion('web', model.web_port)}
						class="mt-1 text-[9px] font-semibold text-amber-400 hover:text-amber-300"
					>
						Port {model.web_port} is in use{#if suggestionFor(model.web_port)} — use {suggestionFor(model.web_port)} instead{/if}
					</button>
				{:else}
					<p class="mt-1 text-[9px] text-zinc-600">The external port used to access this application's web interface.</p>
				{/if}
			</div>
			<div class="md:col-span-1">
				<label for="image-tag" class="mb-2 block text-[10px] font-bold uppercase tracking-widest text-zinc-500">Tag</label>
				<input
					id="image-tag"
					type="text"
					value="latest"
					readonly
					class="w-full rounded-xl border border-white/5 bg-zinc-900/50 p-3 text-sm text-zinc-500 outline-none"
				/>
			</div>
		</div>

		<div>
			<label for="app-icon" class="mb-2 block text-[10px] font-bold uppercase tracking-widest text-zinc-500">App Icon URL</label>
			<div class="flex gap-3">
				<div class="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl border border-white/5 bg-white/[0.03] overflow-hidden">
					{#if model.icon}
						<img src={model.icon} alt="icon preview" class="h-8 w-8 object-contain" />
					{:else}
						<Box class="h-5 w-5 text-zinc-600" />
					{/if}
				</div>
				<input
					id="app-icon"
					type="text"
					bind:value={model.icon}
					oninput={onchange}
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
			<h2 class="text-sm font-bold text-white">Network Settings</h2>
		</div>

		<div class="grid grid-cols-1 gap-6 md:grid-cols-2">
			<div>
				<label for="network-mode" class="mb-2 block text-[10px] font-bold uppercase tracking-widest text-zinc-500">Network Mode</label>
				<select
					id="network-mode"
					bind:value={model.network}
					onchange={onchange}
					class="w-full rounded-xl border border-white/5 bg-white/[0.03] p-3 text-sm text-white outline-none focus:border-emerald-500/50 transition-all"
				>
					<option value="bridge">Bridge (Default)</option>
					<option value="host">Host</option>
					<option value="none">None</option>
				</select>
			</div>
		</div>

		<!-- Ports -->
		<div class="space-y-4">
			<div class="flex items-center justify-between">
				<p class="text-[10px] font-bold uppercase tracking-widest text-zinc-500">Port Mapping</p>
				<button onclick={addPort} class="flex items-center gap-1 text-[10px] font-bold uppercase text-emerald-500 hover:text-emerald-400">
					<Plus class="h-3 w-3" /> Add Port
				</button>
			</div>

			<div class="space-y-2">
				{#each model.ports as port, i}
					{@const hostStatus = statusFor(port.host)}
					{@const hostSuggestion = suggestionFor(port.host)}
					<div>
						<div class="grid grid-cols-7 gap-2">
							<div class="relative col-span-3">
								<input
									type="text"
									bind:value={port.host}
									oninput={onchange}
									placeholder="Host"
									aria-label="Host port"
									class={cn(
										"w-full rounded-lg border bg-white/[0.02] p-2 pr-7 text-xs text-white outline-none",
										hostStatus === 'inuse' ? 'border-amber-500/40 focus:border-amber-500/60' : 'border-white/5 focus:border-emerald-500/30'
									)}
								/>
								{#if hostStatus === 'free'}
									<Check class="absolute right-2 top-2 h-3.5 w-3.5 text-emerald-500" strokeWidth={2.5} />
								{:else if hostStatus === 'inuse'}
									<AlertTriangle class="absolute right-2 top-2 h-3.5 w-3.5 text-amber-500" strokeWidth={2.5} />
								{/if}
							</div>
							<div class="flex items-center justify-center text-zinc-700">:</div>
							<input type="text" bind:value={port.container} oninput={onchange} placeholder="Container" aria-label="Container port" class="col-span-2 rounded-lg border border-white/5 bg-white/[0.02] p-2 text-xs text-white outline-none focus:border-emerald-500/30" />
							<button onclick={() => removePort(i)} aria-label="Remove port" class="flex items-center justify-center text-zinc-600 hover:text-red-500">
								<X class="h-3.5 w-3.5" />
							</button>
						</div>
						{#if hostStatus === 'inuse'}
							<button
								type="button"
								onclick={() => applySuggestion(i, port.host)}
								class="mt-1 text-[9px] font-semibold text-amber-400 hover:text-amber-300"
							>
								Port {port.host} is in use{#if hostSuggestion} — use {hostSuggestion} instead{/if}
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
			<div class="flex h-8 w-8 items-center justify-center rounded-lg bg-amber-500/10 text-amber-400">
				<HardDrive class="h-4 w-4" />
			</div>
			<h2 class="text-sm font-bold text-white">Storage & Devices</h2>
		</div>

		<div class="space-y-4">
			<div class="flex items-center justify-between">
				<p class="text-[10px] font-bold uppercase tracking-widest text-zinc-500">Volume Mounts</p>
				<button onclick={addVolume} class="flex items-center gap-1 text-[10px] font-bold uppercase text-emerald-500 hover:text-emerald-400">
					<Plus class="h-3 w-3" /> Add Volume
				</button>
			</div>

			<div class="space-y-2">
				{#each model.volumes as vol, i}
					<div class="grid grid-cols-7 gap-2">
						<input type="text" bind:value={vol.host} oninput={onchange} placeholder="Host Path" aria-label="Host path" class="col-span-3 rounded-lg border border-white/5 bg-white/[0.02] p-2 text-xs text-white outline-none focus:border-emerald-500/30" />
						<div class="flex items-center justify-center text-zinc-700">→</div>
						<input type="text" bind:value={vol.container} oninput={onchange} placeholder="Container Path" aria-label="Container path" class="col-span-2 rounded-lg border border-white/5 bg-white/[0.02] p-2 text-xs text-white outline-none focus:border-emerald-500/30" />
						<button onclick={() => removeVolume(i)} aria-label="Remove volume" class="flex items-center justify-center text-zinc-600 hover:text-red-500">
							<X class="h-3.5 w-3.5" />
						</button>
					</div>
				{/each}
			</div>
		</div>

		<div class="space-y-4">
			<div class="flex items-center justify-between">
				<p class="text-[10px] font-bold uppercase tracking-widest text-zinc-500">Devices (Hardware)</p>
				<button onclick={addDevice} class="flex items-center gap-1 text-[10px] font-bold uppercase text-emerald-500 hover:text-emerald-400">
					<Plus class="h-3 w-3" /> Add Device
				</button>
			</div>

			<div class="space-y-2">
				{#each model.devices as dev, i}
					<div class="grid grid-cols-7 gap-2">
						<input type="text" bind:value={dev.host} oninput={onchange} placeholder="/dev/ttyUSB0" aria-label="Host device path" class="col-span-3 rounded-lg border border-white/5 bg-white/[0.02] p-2 text-xs text-white outline-none focus:border-emerald-500/30" />
						<div class="flex items-center justify-center text-zinc-700">:</div>
						<input type="text" bind:value={dev.container} oninput={onchange} placeholder="/dev/ttyUSB0" aria-label="Container device path" class="col-span-2 rounded-lg border border-white/5 bg-white/[0.02] p-2 text-xs text-white outline-none focus:border-emerald-500/30" />
						<button onclick={() => removeDevice(i)} aria-label="Remove device" class="flex items-center justify-center text-zinc-600 hover:text-red-500">
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
			<div class="flex h-8 w-8 items-center justify-center rounded-lg bg-purple-500/10 text-purple-400">
				<Shield class="h-4 w-4" />
			</div>
			<h2 class="text-sm font-bold text-white">Execution & Resources</h2>
		</div>

		<div>
			<label for="container-command" class="mb-4 block text-[10px] font-bold uppercase tracking-widest text-zinc-500">Container Command</label>
			<input
				id="container-command"
				type="text"
				bind:value={model.command}
				oninput={onchange}
				placeholder="e.g. npm start"
				class="w-full rounded-xl border border-white/5 bg-white/[0.03] p-3 text-sm font-mono text-zinc-300 outline-none focus:border-emerald-500/50 transition-all"
			/>
		</div>

		<div class="grid grid-cols-1 gap-4 md:grid-cols-2">
			<div>
				<label for="run-as-user" class="mb-4 block text-[10px] font-bold uppercase tracking-widest text-zinc-500">Run as User (UID:GID)</label>
				<input
					id="run-as-user"
					type="text"
					bind:value={model.user}
					oninput={onchange}
					placeholder="e.g. 1000:1000"
					class="w-full rounded-xl border border-white/5 bg-white/[0.03] p-3 text-sm font-mono text-zinc-300 outline-none focus:border-emerald-500/50 transition-all"
				/>
			</div>
			<div>
				<label for="working-dir" class="mb-4 block text-[10px] font-bold uppercase tracking-widest text-zinc-500">Working Directory</label>
				<input
					id="working-dir"
					type="text"
					bind:value={model.working_dir}
					oninput={onchange}
					placeholder="e.g. /app"
					class="w-full rounded-xl border border-white/5 bg-white/[0.03] p-3 text-sm font-mono text-zinc-300 outline-none focus:border-emerald-500/50 transition-all"
				/>
			</div>
		</div>

		<div class="flex items-center justify-between rounded-2xl border border-white/5 bg-white/[0.02] p-4">
			<div>
				<h3 class="text-sm font-bold text-white">Privileged Mode</h3>
				<p class="text-[10px] text-zinc-500">Full access to host hardware</p>
			</div>
			<button
				onclick={() => { model.privileged = !model.privileged; onchange(); }}
				aria-label={model.privileged ? 'Disable privileged mode' : 'Enable privileged mode'}
				aria-pressed={model.privileged}
				class={cn(
					"h-6 w-11 rounded-full p-1 transition-colors duration-300",
					model.privileged ? "bg-emerald-500" : "bg-zinc-800"
				)}
			>
				<div class={cn(
					"h-4 w-4 rounded-full bg-white transition-transform duration-300",
					model.privileged ? "translate-x-5" : "translate-x-0"
				)}></div>
			</button>
		</div>

		<div class="space-y-4">
			<div class="flex items-center justify-between">
				<label for="mem-limit" class="text-[10px] font-bold uppercase tracking-widest text-zinc-500">Memory Limit</label>
				<span class="text-xs font-bold text-emerald-500">{model.mem_limit || 'Unlimited'}</span>
			</div>
			<input
				id="mem-limit"
				type="range"
				min="128"
				max="8192"
				step="128"
				bind:value={model.mem_limit_num}
				oninput={() => { model.mem_limit = `${model.mem_limit_num}M`; onchange(); }}
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
			<p class="mb-2 text-[10px] font-bold uppercase tracking-widest text-zinc-500">Restart Policy</p>
			<div class="flex flex-wrap gap-2">
				{#each ['no', 'always', 'unless-stopped', 'on-failure'] as policy}
					<button
						onclick={() => { model.restart = policy; onchange(); }}
						class={cn(
							"rounded-lg border px-3 py-1.5 text-[10px] font-bold uppercase transition-all",
							model.restart === policy
								? "border-emerald-500/50 bg-emerald-500/10 text-emerald-500 shadow-[0_0_12px_rgba(16,185,129,0.1)]"
								: "border-white/5 bg-white/[0.02] text-zinc-500 hover:border-white/20"
						)}
					>
						{policy}
					</button>
				{/each}
			</div>
		</div>

		<div class="space-y-4">
			<div class="flex items-center justify-between">
				<p class="text-[10px] font-bold uppercase tracking-widest text-zinc-500">Environment Variables</p>
				<button onclick={addEnv} class="flex items-center gap-1 text-[10px] font-bold uppercase text-emerald-500 hover:text-emerald-400">
					<Plus class="h-3 w-3" /> Add Var
				</button>
			</div>

			<div class="space-y-2">
				{#each model.env as env, i}
					<div class="grid grid-cols-7 gap-2">
						<input type="text" bind:value={env.key} oninput={onchange} placeholder="KEY" aria-label="Environment variable key" class="col-span-3 rounded-lg border border-white/5 bg-white/[0.02] p-2 text-xs font-mono text-white outline-none focus:border-emerald-500/30" />
						<div class="flex items-center justify-center text-zinc-700">=</div>
						<input type="text" bind:value={env.value} oninput={onchange} placeholder="VALUE" aria-label="Environment variable value" class="col-span-2 rounded-lg border border-white/5 bg-white/[0.02] p-2 text-xs font-mono text-white outline-none focus:border-emerald-500/30" />
						<button onclick={() => removeEnv(i)} aria-label="Remove environment variable" class="flex items-center justify-center text-zinc-600 hover:text-red-500">
							<X class="h-3.5 w-3.5" />
						</button>
					</div>
				{/each}
			</div>
		</div>

		<div class="space-y-4">
			<div class="flex items-center justify-between">
				<p class="text-[10px] font-bold uppercase tracking-widest text-zinc-500">Labels (Metadata)</p>
				<button onclick={() => { model.labels = [...model.labels, { key: '', value: '' }]; onchange(); }} class="flex items-center gap-1 text-[10px] font-bold uppercase text-emerald-500 hover:text-emerald-400">
					<Plus class="h-3 w-3" /> Add Label
				</button>
			</div>

			<div class="space-y-2">
				{#each model.labels as label, i}
					<div class="grid grid-cols-7 gap-2">
						<input type="text" bind:value={label.key} oninput={onchange} placeholder="Label Key" aria-label="Label key" class="col-span-3 rounded-lg border border-white/5 bg-white/[0.02] p-2 text-xs text-white outline-none focus:border-emerald-500/30" />
						<div class="flex items-center justify-center text-zinc-700">=</div>
						<input type="text" bind:value={label.value} oninput={onchange} placeholder="Value" aria-label="Label value" class="col-span-2 rounded-lg border border-white/5 bg-white/[0.02] p-2 text-xs text-white outline-none focus:border-emerald-500/30" />
						<button onclick={() => { model.labels = model.labels.filter((_, idx) => idx !== i); onchange(); }} aria-label="Remove label" class="flex items-center justify-center text-zinc-600 hover:text-red-500">
							<X class="h-3.5 w-3.5" />
						</button>
					</div>
				{/each}
			</div>
		</div>
	</section>
</div>
