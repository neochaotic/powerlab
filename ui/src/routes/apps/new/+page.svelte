<script lang="ts">
	import { onMount, onDestroy, untrack } from 'svelte';
	import { ArrowLeft, Play, Code, LayoutDashboard, AlertCircle, X, Loader2, CheckCircle2, Terminal, Package, Minimize2 } from 'lucide-svelte';
	import { cn } from '$lib/utils';
	import { api, getAuthToken } from '$lib/api/client';
	import { getComposeApp, getComposeAppLogs, applyComposeAppSettings } from '$lib/api/apps';
	import { ENDPOINTS } from '$lib/api/endpoints';
	import yaml from 'js-yaml';
	import { readPowerLabExt, writePowerLabExt, deletePowerLabExtProperty } from '$lib/utils/compose-extension';
	import ComposeForm, { type ComposeModel } from '$lib/components/orchestrator/ComposeForm.svelte';
	import InstallModal from '$lib/components/apps/InstallModal.svelte';
	import { useAppStore } from '$lib/stores/apps.svelte';

	const appStore = useAppStore();
	import { page } from '$app/stores';
	import { toast } from '$lib/stores/toast.svelte';
	import { goto } from '$app/navigation';
	import { fade } from 'svelte/transition';
	import { t } from '$lib/i18n/index.svelte';
	import { parseLatestPhase, phaseProgress } from '$lib/utils/install-phase';
	import { validateComposeName } from '$lib/utils/compose-name';

	let yamlText = $state(`version: '3.8'
services:
  web:
    image: nginx:latest
    ports:
      - "80:80"
    restart: always
`);

	let formModel = $state<ComposeModel>({
		name: '',
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

	// Service-name validation: see `lib/utils/compose-name.ts`. The
	// regex lives there so the contract is unit-tested independently
	// of this 1000-line page (#240 regression lock).
	const nameValidationError = $derived.by<string | null>(() => {
		const err = validateComposeName(formModel.name);
		if (err === 'empty') return t('form.nameRequired');
		if (err === 'invalid_chars') return t('form.nameInvalidChars');
		return null;
	});
	let deployResult = $state<{ success: boolean; message: string } | null>(null);
	// Minimize the full-screen deploy overlay so the user can keep
	// editing or browsing while the install runs in the background.
	// Mirrors the same pattern used for store-app installs.
	let deployMinimized = $state(false);

	// Deploy log streaming state
	let deployLogs = $state<string[]>([]);
	let deployAppId = $state<string | null>(null);
	let eventSource: EventSource | null = null;
	let sseTimeoutId: ReturnType<typeof setTimeout> | null = null;
	let deployTimedOut = $state(false);

	// Phase indicator parity with native-app install (#116 item 3).
	// The same `parseLatestPhase` helper that drives the native-app
	// install overlay's progress bar now drives the custom-app
	// deploy overlay too. Pure derivation — feeds the new progress
	// bar in the deploy modal.
	const deployJoinedLogs = $derived(deployLogs.join('\n'));
	const currentDeployPhase = $derived.by(() => parseLatestPhase(deployJoinedLogs));
	const deployProgress = $derived.by(() => phaseProgress(currentDeployPhase));

	function clearSseTimeout() {
		if (sseTimeoutId) {
			clearTimeout(sseTimeoutId);
			sseTimeoutId = null;
		}
	}

	function startLogStreaming(id: string) {
		stopLogStreaming();
		deployLogs = [];
		deployTimedOut = false;
		// EventSource can't send Authorization header, so the JWT
		// must travel as ?token=… instead. The gateway and
		// app-management both accept it as a fallback.
		const token = getAuthToken();
		const path = ENDPOINTS.APP_COMPOSE_TASK_LOGS.replace(':id', id);
		const url = token ? `${path}?token=${encodeURIComponent(token)}` : path;
		eventSource = new EventSource(url);

		eventSource.onmessage = (event) => {
			if (event.data) {
				// LogStreamer owns scroll behaviour now (auto-scroll +
				// pause-on-manual-scroll) — see #335 / v0.6.6. The
				// manual scrollTop manipulation here is gone.
				deployLogs = [...deployLogs, event.data];
			}
		};

		eventSource.onerror = () => {
			stopLogStreaming();
		};

		// SSE close marker. Mirrors apps/+page.svelte's checkInstallResult:
		// the install ACTUALLY finished here, so transition the modal
		// to its terminal state by fetching the installed-app list
		// and deciding success/error from whether our app appears.
		// v0.6.8 fix #341: unifies Custom App with Community Install
		// terminal-state behaviour (#247 carry-over). Before this, the
		// `event: end` handler only closed the stream — deployResult
		// had been pre-set to success on POST 2xx, surfacing
		// "Service running" before any image had been pulled.
		eventSource.addEventListener('end', async () => {
			stopLogStreaming();
			await finalizeDeploy(id);
		});

		// Safety timeout: matches native-app install (apps/+page.svelte
		// streamInstallLogs). Without this a wedged SSE leaves the
		// deploy overlay spinning forever; with it, after 10 minutes
		// the user gets a "taking longer than expected" surface and can
		// dismiss / retry.
		clearSseTimeout();
		sseTimeoutId = setTimeout(() => {
			stopLogStreaming();
			deployTimedOut = true;
		}, 600_000);
	}

	function stopLogStreaming() {
		if (eventSource) {
			eventSource.close();
			eventSource = null;
		}
		clearSseTimeout();
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

			// Volumes — handle both compose forms:
			//   short: "/host:/container[:mode]"  (string)
			//   long:  {type, source, target, ...} (object) — #332 fix.
			// The short-form path also tolerates a 3rd field (mode like ":ro")
			// by ignoring it; the form has no mode column yet.
			if (Array.isArray(service.volumes)) {
				formModel.volumes = service.volumes.map((v: unknown) => {
					if (v && typeof v === 'object') {
						const obj = v as { source?: string; target?: string };
						return { host: obj.source ?? '', container: obj.target ?? '' };
					}
					const [host, container] = String(v).split(':');
					return { host: host || '', container: container || host || '' };
				});
			} else {
				formModel.volumes = [];
			}

			// Devices — same dual-form handling.
			if (Array.isArray(service.devices)) {
				formModel.devices = service.devices.map((d: unknown) => {
					if (d && typeof d === 'object') {
						const obj = d as { source?: string; target?: string; path_on_host?: string; path_in_container?: string };
						return {
							host: obj.source ?? obj.path_on_host ?? '',
							container: obj.target ?? obj.path_in_container ?? '',
						};
					}
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
				services: { [formModel.name.trim()]: svc }
			};

			if (formModel.icon) root['x-icon'] = formModel.icon;

			// port_map fallback (#278): the Launchpad tile-click handler
			// only opens an app if `store_info.port_map` is set. The
			// backend derives port_map from the compose extension's
			// `port_map`/`web`/`port` key. If the user didn't fill the
			// "Web UI Port" field explicitly but DID configure at least
			// one host port, default port_map to the first host port —
			// this makes Custom Apps "just open" on tile click without
			// the user having to fill two fields for the same value.
			// Explicit web_port still wins.
			const effectivePort = formModel.web_port
				|| (formModel.ports.find((p) => p.host)?.host ?? '');

			if (effectivePort) {
				// New docs author x-powerlab. The translation helper
				// preserves the original key when editing existing YAMLs.
				writePowerLabExt(root, { port_map: effectivePort });
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
				error = t('orchestrator.loadAppFailed', { error: (e as Error).message });
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
		if (nameValidationError) {
			toast.error(nameValidationError);
			return;
		}

		isDeploying = true;
		error = null;
		deployResult = null;

		try {
			const parsed = yaml.load(yamlText) as any;
			const id = parsed?.name || Object.keys(parsed?.services || {})[0] || 'app';
			deployAppId = id;

			startLogStreaming(id);

			// Edit-mode (URL has ?id=X without &fork=1) MUST use the
			// PUT applyComposeAppSettings endpoint — only that path
			// runs the backend's skip-self port-conflict logic. POST
			// (install) flags the app's own running ports as
			// conflicts and the deploy fails with "ports in use".
			// Closes #65.
			const editingId = $page.url.searchParams.get('id');
			const fork = $page.url.searchParams.get('fork') === '1';
			const response = editingId && !fork
				? await applyComposeAppSettings(editingId, yamlText)
				: await api.postYaml<any>(ENDPOINTS.APP_COMPOSE_DEPLOY, yamlText);
			
			// v0.6.8 fix #341: do NOT set deployResult here. POST 2xx
			// means install STARTED, not completed. The terminal state
			// gets decided by finalizeDeploy() in the SSE `end`
			// handler. This brings Custom App lifecycle to parity with
			// Community Install (apps/+page.svelte) — #247 carry-over.
		} catch (e) {
			deployResult = {
				success: false,
				message: (e as Error).message || t('orchestrator.deploymentStartFailed')
			};
			error = (e as Error).message;
			isDeploying = false;
		}
	}

	// Called when the SSE stream emits `event: end` (the backend
	// PullAndInstall goroutine completed). Fetches the installed-app
	// list and sets deployResult to success / error based on whether
	// the app actually appeared. Mirrors apps/+page.svelte
	// checkInstallResult.
	async function finalizeDeploy(id: string) {
		try {
			// Reuse the same store flow Community Install uses
			// (checkInstallResult in apps/+page.svelte). `fetchInstalledApps`
			// shapes the response as a Record<string, app>; `installedApps`
			// getter returns the derived list with `id` field on each
			// entry. Sharing the path eliminates the divergence that
			// surfaced in v0.6.7.
			await appStore.fetchInstalledApps();
			const found = appStore.installedApps.find((a) => a.id === id);
			if (found) {
				deployResult = {
					success: true,
					message: t('orchestrator.serviceRunning')
				};
			} else {
				const lastErr = deployLogs
					.slice()
					.reverse()
					.find((l) => /error|fail|denied|not found|permitted/i.test(l));
				deployResult = {
					success: false,
					message: lastErr ?? t('orchestrator.deployFailed')
				};
			}
		} catch (e) {
			deployResult = {
				success: false,
				message: (e as Error).message || t('orchestrator.deployFailed')
			};
		} finally {
			isDeploying = false;
		}
	}

	// Safety net: if event:end never fires within 10 min, drop out of
	// 'deploying' state with a timeout surface (mirrors the existing
	// deployTimedOut path used by stopLogStreaming-on-error). Without
	// this, a broken SSE leaves the modal hostage indefinitely.
	$effect(() => {
		if (!isDeploying) return;
		const timer = setTimeout(() => {
			if (isDeploying && !deployResult) {
				untrack(() => {
					deployResult = {
						success: false,
						message: t('orchestrator.deployTimedOut')
					};
					isDeploying = false;
				});
			}
		}, 600_000);
		return () => clearTimeout(timer);
	});

	function handleFormChange() {
		syncFormToYaml();
	}
</script>

<svelte:head>
	<title>{t('orchestrator.newCustomApp')} — PowerLab</title>
</svelte:head>

<div class="flex h-full flex-col text-zinc-50 font-sans antialiased">
	<!-- Header -->
	<header class="flex h-16 items-center justify-between border-b border-white/5 bg-zinc-900/50 px-6 backdrop-blur-md">
		<div class="flex items-center gap-6">
			<a
				href="/"
				aria-label={t('orchestrator.backToLaunchpad')}
				title={t('orchestrator.backToLaunchpad')}
				class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl border border-white/[0.06] bg-white/[0.02] text-zinc-400 transition-all hover:-translate-x-0.5 hover:border-white/10 hover:bg-white/[0.05] hover:text-white"
			>
				<ArrowLeft class="h-4 w-4" />
			</a>
			<div class="flex flex-col">
				<h1 class="text-sm font-bold tracking-tight text-white">
					{isFork ? t('orchestrator.forkCustomApp') : ($page.url.searchParams.get('id') ? t('orchestrator.editCustomApp') : t('orchestrator.newCustomApp'))}
				</h1>
				<p class="text-[10px] font-medium uppercase tracking-[0.2em] text-zinc-500">{t('orchestrator.builder')}</p>
			</div>
		</div>

		<div class="flex items-center gap-3">
			<!-- View switcher -->
			<div class="mr-4 flex items-center gap-1 rounded-xl bg-white/5 p-1">
				<button
					onclick={() => activeView = 'split'}
					class={cn("flex h-7 items-center gap-2 rounded-lg px-3 text-[10px] font-bold uppercase tracking-widest transition-all", activeView === 'split' ? "bg-white text-black shadow-lg" : "text-zinc-500 hover:text-white")}
				>
					<LayoutDashboard class="h-3 w-3" /> {t('orchestrator.viewSplit')}
				</button>
				<button
					onclick={() => activeView = 'form'}
					class={cn("flex h-7 items-center gap-2 rounded-lg px-3 text-[10px] font-bold uppercase tracking-widest transition-all", activeView === 'form' ? "bg-white text-black shadow-lg" : "text-zinc-500 hover:text-white")}
				>
					{t('orchestrator.viewForm')}
				</button>
				<button
					onclick={() => activeView = 'yaml'}
					class={cn("flex h-7 items-center gap-2 rounded-lg px-3 text-[10px] font-bold uppercase tracking-widest transition-all", activeView === 'yaml' ? "bg-white text-black shadow-lg" : "text-zinc-500 hover:text-white")}
				>
					<Code class="h-3 w-3" /> {t('orchestrator.viewYaml')}
				</button>
			</div>

			<button
				onclick={handleDeploy}
				disabled={isDeploying || !!nameValidationError}
				title={nameValidationError ?? undefined}
				class="flex h-9 items-center gap-2 rounded-xl bg-white px-4 text-xs font-bold text-black transition-transform hover:scale-105 active:scale-95 disabled:opacity-50 disabled:scale-100 disabled:cursor-not-allowed"
			>
				{#if isDeploying}
					<Loader2 class="h-3.5 w-3.5 animate-spin" />
				{:else}
					<Play class="h-3.5 w-3.5 fill-black" />
				{/if}
				{t('orchestrator.deploy')}
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

	<!-- Minimized: floating pill bottom-right -->
	<!-- Deploy lifecycle modal — Sprint 14 #345 unification.
		 The bespoke deploy modal here was replaced by the shared
		 InstallModal component now consumed by both /apps and
		 /apps/new. State translates: isDeploying/deployResult/
		 deployTimedOut → install lifecycle phase. -->
	<InstallModal
		phase={deployTimedOut ? 'timeout' :
			deployResult?.success ? 'success' :
			deployResult ? 'error' :
			isDeploying ? 'starting' :
			'idle'}
		currentPhase={currentDeployPhase}
		progress={deployProgress}
		logs={deployJoinedLogs}
		appTitle={formModel.name || 'app'}
		appIcon={formModel.icon}
		error={deployResult?.success === false ? deployResult.message : null}
		portNote={null}
		minimized={deployMinimized}
		onMinimize={() => { deployMinimized = true; goto('/'); }}
		onCancel={() => { deployResult = null; isDeploying = false; }}
		onRetry={() => { deployResult = null; }}
		onOpen={() => { deployResult = null; goto('/'); }}
		onStay={() => { deployResult = null; }}
		onCheckLaunchpad={() => { deployResult = null; goto('/'); }}
	/>

	<!-- Main Content Area -->
	<main class="flex-1 overflow-hidden">
		<div class="flex h-full">
			<!-- Form Panel -->
			{#if activeView === 'split' || activeView === 'form'}
				<div class={cn("h-full overflow-y-auto border-r border-white/5 custom-scrollbar transition-all duration-500", activeView === 'split' ? "w-1/2" : "w-full")}>
					<div class="mx-auto max-w-2xl p-8">
						<ComposeForm bind:model={formModel} onchange={handleFormChange} nameError={nameValidationError} />
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
								aria-label={t('orchestrator.exitToApps')}
								class="group relative h-3 w-3 rounded-full bg-red-500/80 transition-colors hover:bg-red-500"
							>
								<X class="absolute left-1/2 top-1/2 h-1.5 w-1.5 -translate-x-1/2 -translate-y-1/2 text-black opacity-0 transition-opacity group-hover:opacity-100" />
							</button>
							<button
								onclick={() => activeView = activeView === 'split' ? 'yaml' : 'split'}
								aria-label={t('orchestrator.toggleSplit')}
								class="h-3 w-3 rounded-full bg-amber-500"
							></button>
							<button
								onclick={() => activeView = 'form'}
								aria-label={t('orchestrator.switchToForm')}
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
							aria-label={t('orchestrator.yamlEditor')}
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
