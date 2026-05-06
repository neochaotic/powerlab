<script lang="ts">
	import { onMount, onDestroy, untrack } from 'svelte';
	import { ArrowLeft, Play, Code, LayoutDashboard, AlertCircle, X, Loader2, CheckCircle2, Terminal } from 'lucide-svelte';
	import { cn } from '$lib/utils';
	import { api } from '$lib/api/client';
	import { getComposeApp, getComposeAppLogs } from '$lib/api/apps';
	import { ENDPOINTS } from '$lib/api/endpoints';
	import yaml from 'js-yaml';
	import { readPowerLabExt, writePowerLabExt, deletePowerLabExtProperty } from '$lib/utils/compose-extension';
	import ComposeForm, { type ComposeModel } from '$lib/components/orchestrator/ComposeForm.svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { fade } from 'svelte/transition';

	let yamlText = $state(`version: '3.8'
services:
  web:
    image: nginx:latest
    ports:
      - "80:80"
    restart: always
`);

	let formModel = $state<ComposeModel>({
		name: 'web',
		container_name: '',
		image: 'nginx:latest',
		icon: '',
		network: 'bridge',
		ports: [{ host: '80', container: '80' }],
		volumes: [] as { host: string; container: string }[],
		devices: [] as { host: string; container: string }[],
		env: [] as { key: string; value: string }[],
		labels: [] as { key: string; value: string }[],
		restart: 'always',
		command: '',
		user: '',
		working_dir: '',
		privileged: false,
		mem_limit: '',
		mem_limit_num: 512,
		web_port: ''
	});

	let activeView = $state<'split' | 'form' | 'yaml'>('split');
	let error = $state<string | null>(null);
	let isSyncing = $state(false);
	let isDeploying = $state(false);
	let deployResult = $state<{ success: boolean; message: string } | null>(null);

	// Deploy log streaming state
	let deployLogs = $state<string[]>([]);
	let deployAppId = $state<string | null>(null);
	let logScrollEl = $state<HTMLElement | null>(null);
	let eventSource: EventSource | null = null;
	function startLogStreaming(id: string) {
		stopLogStreaming();
		deployLogs = [];
		const url = ENDPOINTS.APP_COMPOSE_TASK_LOGS.replace(':id', id);
		eventSource = new EventSource(url);
		
		eventSource.onmessage = (event) => {
			if (event.data) {
				deployLogs = [...deployLogs, event.data];
				setTimeout(() => {
					if (logScrollEl) logScrollEl.scrollTop = logScrollEl.scrollHeight;
				}, 10);
			}
		};

		eventSource.onerror = () => {
			stopLogStreaming();
		};

		eventSource.addEventListener('end', () => {
			stopLogStreaming();
		});
	}

	function stopLogStreaming() {
		if (eventSource) {
			eventSource.close();
			eventSource = null;
		}
	}

	onDestroy(stopLogStreaming);

	// Sync YAML → Form
	function syncYamlToForm() {
		if (isSyncing) return;
		try {
			isSyncing = true;
			const parsed = yaml.load(yamlText) as any;
			if (!parsed?.services) return;

			const serviceName = Object.keys(parsed.services)[0];
			const service = parsed.services[serviceName];

			formModel.name = serviceName;
			formModel.container_name = service.container_name || '';
			formModel.image = service.image || '';
			formModel.restart = service.restart || 'always';
			formModel.network = service.network_mode || 'bridge';
			formModel.command = Array.isArray(service.command) ? service.command.join(' ') : (service.command || '');
			formModel.privileged = service.privileged === true;
			formModel.user = service.user || '';
			formModel.working_dir = service.working_dir || '';

			// Memory limit
			if (service.deploy?.resources?.limits?.memory) {
				formModel.mem_limit = service.deploy.resources.limits.memory;
				formModel.mem_limit_num = parseInt(formModel.mem_limit) || 512;
			} else if (service.mem_limit) {
				formModel.mem_limit = service.mem_limit;
				formModel.mem_limit_num = parseInt(formModel.mem_limit) || 512;
			} else {
				formModel.mem_limit = '';
				formModel.mem_limit_num = 512;
			}

			// Icon
			formModel.icon = parsed['x-icon'] || '';

			// Web port (PowerLab/CasaOS extension — translation layer
			// returns whichever alias the doc uses).
			const xExt = readPowerLabExt(parsed);
			formModel.web_port = (xExt?.ext.port_map as string) || '';

			// Ports
			if (Array.isArray(service.ports)) {
				formModel.ports = service.ports.map((p: string) => {
					const [host, container] = String(p).split(':');
					return { host: host || '', container: container || host || '' };
				});
			} else {
				formModel.ports = [];
			}

			// Volumes
			if (Array.isArray(service.volumes)) {
				formModel.volumes = service.volumes.map((v: string) => {
					const [host, container] = String(v).split(':');
					return { host: host || '', container: container || host || '' };
				});
			} else {
				formModel.volumes = [];
			}

			// Devices
			if (Array.isArray(service.devices)) {
				formModel.devices = service.devices.map((d: string) => {
					const [host, container] = String(d).split(':');
					return { host: host || '', container: container || host || '' };
				});
			} else {
				formModel.devices = [];
			}

			// Environment
			if (service.environment) {
				if (Array.isArray(service.environment)) {
					formModel.env = service.environment.map((e: string) => {
						const [key, ...rest] = String(e).split('=');
						return { key, value: rest.join('=') };
					});
				} else {
					formModel.env = Object.entries(service.environment).map(([key, value]) => ({
						key,
						value: String(value)
					}));
				}
			} else {
				formModel.env = [];
			}

			// Labels
			if (service.labels) {
				if (Array.isArray(service.labels)) {
					formModel.labels = service.labels.map((l: string) => {
						const [key, ...rest] = String(l).split('=');
						return { key, value: rest.join('=') };
					});
				} else {
					formModel.labels = Object.entries(service.labels).map(([key, value]) => ({
						key,
						value: String(value)
					}));
				}
			} else {
				formModel.labels = [];
			}

			error = null;
		} catch (e) {
			error = (e as Error).message;
		} finally {
			isSyncing = false;
		}
	}

	// Sync Form → YAML
	function syncFormToYaml() {
		if (isSyncing) return;
		try {
			isSyncing = true;
			const svc: any = {
				image: formModel.image,
				restart: formModel.restart,
			};

			if (formModel.container_name) svc.container_name = formModel.container_name;
			if (formModel.network !== 'bridge') svc.network_mode = formModel.network;
			if (formModel.command) svc.command = formModel.command;
			if (formModel.user) svc.user = formModel.user;
			if (formModel.working_dir) svc.working_dir = formModel.working_dir;
			if (formModel.privileged) svc.privileged = true;

			const ports = formModel.ports.filter(p => p.host && p.container).map(p => `${p.host}:${p.container}`);
			if (ports.length) svc.ports = ports;

			const volumes = formModel.volumes.filter(v => v.host && v.container).map(v => `${v.host}:${v.container}`);
			if (volumes.length) svc.volumes = volumes;

			const devices = formModel.devices.filter(d => d.host && d.container).map(d => `${d.host}:${d.container}`);
			if (devices.length) svc.devices = devices;

			const env = formModel.env.filter(e => e.key).reduce((acc, curr) => {
				acc[curr.key] = curr.value;
				return acc;
			}, {} as Record<string, string>);
			if (Object.keys(env).length) svc.environment = env;

			const labels = formModel.labels.filter(l => l.key).reduce((acc, curr) => {
				acc[curr.key] = curr.value;
				return acc;
			}, {} as Record<string, string>);
			if (Object.keys(labels).length) svc.labels = labels;

			if (formModel.mem_limit && formModel.mem_limit !== 'Unlimited') {
				svc.deploy = { resources: { limits: { memory: formModel.mem_limit } } };
			}

			const root: any = {
				version: '3.8',
				services: { [formModel.name || 'web']: svc }
			};

			if (formModel.icon) root['x-icon'] = formModel.icon;

			if (formModel.web_port) {
				// New docs author x-powerlab. The translation helper
				// preserves the original key when editing existing YAMLs.
				writePowerLabExt(root, { port_map: formModel.web_port });
			}

			yamlText = yaml.dump(root, { indent: 2, lineWidth: -1 });
			error = null;
		} catch (e) {
			error = (e as Error).message;
		} finally {
			isSyncing = false;
		}
	}

	const isFork = $derived($page.url.searchParams.get('fork') === '1');

	onMount(async () => {
		const appId = $page.url.searchParams.get('id');
		const fork = $page.url.searchParams.get('fork') === '1';
		if (appId) {
			try {
				isDeploying = true;
				let yamlContent = await getComposeApp(appId);

				if (fork) {
					// Strip store_app_id so the forked app is treated as a Custom App.
					// Translation layer handles whichever alias the YAML uses
					// (x-powerlab / x-web / x-casaos). Also reset the top-level
					// `name` so the user gives it a unique identity.
					try {
						const parsed = yaml.load(yamlContent) as any;
						deletePowerLabExtProperty(parsed, 'store_app_id');
						// Clear the project name so the user is forced to choose one
						if (parsed?.name) {
							parsed.name = parsed.name + '-custom';
						}
						yamlContent = yaml.dump(parsed, { indent: 2, lineWidth: -1 });
					} catch {
						// If parsing fails, use the YAML as-is
					}
				}

				yamlText = yamlContent;
				syncYamlToForm();
			} catch (e) {
				error = `Failed to load app: ${(e as Error).message}`;
			} finally {
				isDeploying = false;
			}
		}
	});

	// Track yamlText changes OUTSIDE untrack so the effect re-runs on every edit
	$effect(() => {
		yamlText;
		untrack(() => syncYamlToForm());
	});

	async function handleDeploy() {
		if (!yamlText || isDeploying) return;

		isDeploying = true;
		error = null;
		deployResult = null;

		try {
			const parsed = yaml.load(yamlText) as any;
			const id = parsed?.name || Object.keys(parsed?.services || {})[0] || 'app';
			deployAppId = id;
			
			startLogStreaming(id);
			
			const response = await api.postYaml<any>(ENDPOINTS.APP_COMPOSE_DEPLOY, yamlText);
			
			// Wait for logs to finish or success event
			// For now, we wait 5 seconds before redirecting if success
			// or we can wait for the 'end' event from SSE
			deployResult = {
				success: true,
				message: response?.message || 'Deployment started successfully!'
			};
			// We don't redirect immediately to let user see logs
			setTimeout(() => { 
				if (deployResult?.success) goto('/'); 
			}, 6000);
		} catch (e) {
			deployResult = {
				success: false,
				message: (e as Error).message || 'Failed to start deployment'
			};
			error = (e as Error).message;
		} finally {
			isDeploying = false;
		}
	}

	function handleFormChange() {
		syncFormToYaml();
	}
</script>

<svelte:head>
	<title>Custom App — PowerLab</title>
</svelte:head>

<div class="flex h-full flex-col text-zinc-50 font-sans antialiased">
	<!-- Header -->
	<header class="flex h-16 items-center justify-between border-b border-white/5 bg-zinc-900/50 px-6 backdrop-blur-md">
		<div class="flex items-center gap-6">
			<a
				href="/"
				aria-label="Back to Launchpad"
				title="Back to Launchpad"
				class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl border border-white/[0.06] bg-white/[0.02] text-zinc-400 transition-all hover:-translate-x-0.5 hover:border-white/10 hover:bg-white/[0.05] hover:text-white"
			>
				<ArrowLeft class="h-4 w-4" />
			</a>
			<div class="flex flex-col">
				<h1 class="text-sm font-bold tracking-tight text-white">
					{isFork ? 'Fork as Custom App' : ($page.url.searchParams.get('id') ? 'Edit Custom App' : 'New Custom App')}
				</h1>
				<p class="text-[10px] font-medium uppercase tracking-[0.2em] text-zinc-500">Custom App Builder</p>
			</div>
		</div>

		<div class="flex items-center gap-3">
			<!-- View switcher -->
			<div class="mr-4 flex items-center gap-1 rounded-xl bg-white/5 p-1">
				<button
					onclick={() => activeView = 'split'}
					class={cn("flex h-7 items-center gap-2 rounded-lg px-3 text-[10px] font-bold uppercase tracking-widest transition-all", activeView === 'split' ? "bg-white text-black shadow-lg" : "text-zinc-500 hover:text-white")}
				>
					<LayoutDashboard class="h-3 w-3" /> Split
				</button>
				<button
					onclick={() => activeView = 'form'}
					class={cn("flex h-7 items-center gap-2 rounded-lg px-3 text-[10px] font-bold uppercase tracking-widest transition-all", activeView === 'form' ? "bg-white text-black shadow-lg" : "text-zinc-500 hover:text-white")}
				>
					Form
				</button>
				<button
					onclick={() => activeView = 'yaml'}
					class={cn("flex h-7 items-center gap-2 rounded-lg px-3 text-[10px] font-bold uppercase tracking-widest transition-all", activeView === 'yaml' ? "bg-white text-black shadow-lg" : "text-zinc-500 hover:text-white")}
				>
					<Code class="h-3 w-3" /> YAML
				</button>
			</div>

			<button
				onclick={handleDeploy}
				disabled={isDeploying}
				class="flex h-9 items-center gap-2 rounded-xl bg-white px-4 text-xs font-bold text-black transition-transform hover:scale-105 active:scale-95 disabled:opacity-50 disabled:scale-100"
			>
				{#if isDeploying}
					<Loader2 class="h-3.5 w-3.5 animate-spin" />
				{:else}
					<Play class="h-3.5 w-3.5 fill-black" />
				{/if}
				Deploy
			</button>
		</div>
	</header>

	<!-- YAML parse error banner -->
	{#if error}
		<div class="flex items-center gap-3 border-b border-red-500/20 bg-red-500/10 px-6 py-2 text-xs text-red-400">
			<AlertCircle class="h-3.5 w-3.5 shrink-0" />
			<span class="font-mono">{error}</span>
			<button onclick={() => error = null} class="ml-auto text-red-400/60 hover:text-red-400" aria-label="Dismiss error">
				<X class="h-3.5 w-3.5" />
			</button>
		</div>
	{/if}

	<!-- Deployment Overlay -->
	{#if isDeploying || deployResult}
		<div class="fixed inset-0 z-[100] flex items-center justify-center bg-black/60 backdrop-blur-md" in:fade={{ duration: 200 }}>
			<div class="w-full max-w-sm rounded-3xl border border-white/10 bg-zinc-900 p-8 text-center shadow-2xl">
				{#if isDeploying}
					<div class="mb-6 flex justify-center">
						<div class="relative h-16 w-16">
							<div class="absolute inset-0 rounded-full border-2 border-emerald-500/20"></div>
							<div class="absolute inset-0 rounded-full border-t-2 border-emerald-500 animate-spin"></div>
						</div>
					</div>
					<h3 class="text-lg font-bold text-white">Deploying Service</h3>
					<p class="mt-1 text-[10px] text-zinc-500 uppercase tracking-[0.2em]">Orchestrating container...</p>

					<!-- Terminal Area -->
					<div class="mt-6 flex flex-col overflow-hidden rounded-xl border border-white/5 bg-black/40 text-left shadow-inner">
						<div class="flex h-6 items-center gap-1.5 border-b border-white/5 bg-white/[0.02] px-3">
							<Terminal class="h-3 w-3 text-zinc-500" />
							<span class="text-[9px] font-bold uppercase tracking-widest text-zinc-600">Installation Logs</span>
						</div>
						<div 
							bind:this={logScrollEl}
							class="h-48 overflow-y-auto p-3 font-mono text-[10px] leading-relaxed custom-scrollbar"
						>
							{#each deployLogs as log}
								<div class="flex gap-2">
									<span class="text-emerald-500/40 select-none">›</span>
									<span class="text-zinc-300 break-all">{log}</span>
								</div>
							{:else}
								<div class="flex h-full items-center justify-center text-zinc-700 animate-pulse">
									Waiting for logs...
								</div>
							{/each}
						</div>
					</div>
				{:else if deployResult}
					<div class="mb-6 flex justify-center">
						{#if deployResult.success}
							<div class="flex h-16 w-16 items-center justify-center rounded-full bg-emerald-500/10 text-emerald-500 shadow-[0_0_20px_rgba(16,185,129,0.2)]">
								<CheckCircle2 class="h-8 w-8" />
							</div>
						{:else}
							<div class="flex h-16 w-16 items-center justify-center rounded-full bg-red-500/10 text-red-500 shadow-[0_0_20px_rgba(239,68,68,0.2)]">
								<AlertCircle class="h-8 w-8" />
							</div>
						{/if}
					</div>
					<h3 class="text-lg font-bold text-white">{deployResult.success ? 'Success!' : 'Deployment Failed'}</h3>
					<p class="mt-2 text-sm text-zinc-400">{deployResult.message}</p>
					{#if !deployResult.success}
						<button
							onclick={() => deployResult = null}
							class="mt-6 w-full rounded-xl bg-white/5 py-3 text-xs font-bold uppercase tracking-widest text-white hover:bg-white/10 transition-colors"
						>
							Dismiss
						</button>
					{/if}
				{/if}
			</div>
		</div>
	{/if}

	<!-- Main Content Area -->
	<main class="flex-1 overflow-hidden">
		<div class="flex h-full">
			<!-- Form Panel -->
			{#if activeView === 'split' || activeView === 'form'}
				<div class={cn("h-full overflow-y-auto border-r border-white/5 custom-scrollbar transition-all duration-500", activeView === 'split' ? "w-1/2" : "w-full")}>
					<div class="mx-auto max-w-2xl p-8">
						<ComposeForm bind:model={formModel} onchange={handleFormChange} />
					</div>
				</div>
			{/if}

			<!-- YAML Editor Panel -->
			{#if activeView === 'split' || activeView === 'yaml'}
				<div class={cn("h-full flex flex-col bg-[#08080a] transition-all duration-500", activeView === 'split' ? "w-1/2" : "w-full")}>
					<!-- Mac-style editor chrome -->
					<div class="flex h-10 items-center justify-between border-b border-white/5 bg-white/[0.02] px-4">
						<div class="flex items-center gap-1.5">
							<button
								onclick={() => goto('/')}
								aria-label="Exit to apps"
								class="group relative h-3 w-3 rounded-full bg-red-500/80 transition-colors hover:bg-red-500"
							>
								<X class="absolute left-1/2 top-1/2 h-1.5 w-1.5 -translate-x-1/2 -translate-y-1/2 text-black opacity-0 transition-opacity group-hover:opacity-100" />
							</button>
							<button
								onclick={() => activeView = activeView === 'split' ? 'yaml' : 'split'}
								aria-label="Toggle split view"
								class="h-3 w-3 rounded-full bg-amber-500"
							></button>
							<button
								onclick={() => activeView = 'form'}
								aria-label="Switch to form view"
								class="h-3 w-3 rounded-full bg-emerald-500"
							></button>
						</div>
						<div class="text-[10px] font-bold uppercase tracking-widest text-zinc-600">docker-compose.yml</div>
						<div class="w-12"></div>
					</div>

					<div class="flex-1 overflow-hidden">
						<textarea
							bind:value={yamlText}
							spellcheck="false"
							aria-label="Docker Compose YAML editor"
							class="h-full w-full resize-none bg-transparent p-8 font-mono text-sm text-emerald-500/90 outline-none selection:bg-emerald-500/20 selection:text-white custom-scrollbar"
						></textarea>
					</div>
				</div>
			{/if}
		</div>
	</main>
</div>

<style>
	.custom-scrollbar::-webkit-scrollbar { width: 6px; }
	.custom-scrollbar::-webkit-scrollbar-track { background: transparent; }
	.custom-scrollbar::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.05); border-radius: 10px; }
	.custom-scrollbar::-webkit-scrollbar-thumb:hover { background: rgba(255,255,255,0.1); }
</style>
