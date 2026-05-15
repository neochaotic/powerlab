<script lang="ts">
	import { onMount, onDestroy, untrack } from 'svelte';
	import { ArrowLeft, Play, AlertCircle, X, Loader2 } from 'lucide-svelte';
	import { cn } from '$lib/utils';
	import { getAuthToken } from '$lib/api/client';
	import {
		getComposeApp,
		applyComposeAppSettings,
		installComposeApp
	} from '$lib/api/apps';
	import { ENDPOINTS } from '$lib/api/endpoints';
	import yaml from 'js-yaml';
	import { deletePowerLabExtProperty } from '$lib/utils/compose-extension';
	import YAMLPreview from '$lib/components/orchestrator/YAMLPreview.svelte';
	import InstallModal from '$lib/components/apps/InstallModal.svelte';
	import { useAppStore } from '$lib/stores/apps.svelte';

	const appStore = useAppStore();
	import { page } from '$app/stores';
	import { toast } from '$lib/stores/toast.svelte';
	import { goto } from '$app/navigation';
	import { t } from '$lib/i18n/index.svelte';
	import { parseLatestPhase, phaseProgress } from '$lib/utils/install-phase';
	import { validateComposeName } from '$lib/utils/compose-name';

	// YAML-first design (ADR-0035 follow-on, supersedes the bidirectional
	// ComposeForm + ComposeModel round-trip that was the source of the
	// "[object Object]" bug class for long-form volumes / devices / port
	// objects). Single source of truth is `yamlText`. The summary panel
	// derives a read-only preview via $derived. Install button calls
	// `installComposeApp(yamlText)` — same helper Community Install uses,
	// so wire-level error handling stays unified.

	let yamlText = $state(`name: my-app
services:
  web:
    image: nginx:latest
    ports:
      - "80:80"
    restart: always
`);

	let error = $state<string | null>(null);
	let isDeploying = $state(false);
	let deployResult = $state<{ success: boolean; message: string } | null>(null);
	let deployMinimized = $state(false);

	let deployLogs = $state<string[]>([]);
	let deployAppId = $state<string | null>(null);
	let eventSource: EventSource | null = null;
	let sseTimeoutId: ReturnType<typeof setTimeout> | null = null;
	let deployTimedOut = $state(false);

	const deployJoinedLogs = $derived(deployLogs.join('\n'));
	const currentDeployPhase = $derived.by(() => parseLatestPhase(deployJoinedLogs));
	const deployProgress = $derived.by(() => phaseProgress(currentDeployPhase));

	// Service-name validation derived from the YAML's parsed `name`
	// field (top-level, where compose-spec puts the project name) or
	// falls back to the first service key.
	const parsedNameForValidation = $derived.by<string>(() => {
		try {
			const parsed = yaml.load(yamlText) as Record<string, unknown> | null;
			if (parsed && typeof parsed === 'object') {
				if (typeof parsed.name === 'string') return parsed.name;
				const services = parsed.services as Record<string, unknown> | undefined;
				if (services && typeof services === 'object') {
					const first = Object.keys(services)[0];
					if (first) return first;
				}
			}
		} catch {
			// fall through
		}
		return '';
	});

	const nameValidationError = $derived.by<string | null>(() => {
		const err = validateComposeName(parsedNameForValidation);
		if (err === 'empty') return t('form.nameRequired');
		if (err === 'invalid_chars') return t('form.nameInvalidChars');
		return null;
	});

	// Title for the InstallModal — derived from yaml, no formModel.
	const appTitleForModal = $derived(parsedNameForValidation || 'app');
	const appIconForModal = $derived.by<string>(() => {
		try {
			const parsed = yaml.load(yamlText) as Record<string, unknown> | null;
			if (parsed && typeof parsed['x-icon'] === 'string') {
				return parsed['x-icon'] as string;
			}
		} catch {
			// ignore
		}
		return '';
	});

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
		const token = getAuthToken();
		const path = ENDPOINTS.APP_COMPOSE_TASK_LOGS.replace(':id', id);
		const url = token ? `${path}?token=${encodeURIComponent(token)}` : path;
		eventSource = new EventSource(url);

		eventSource.onmessage = (event) => {
			if (event.data) {
				deployLogs = [...deployLogs, event.data];
			}
		};
		eventSource.onerror = () => {
			stopLogStreaming();
		};
		eventSource.addEventListener('end', async () => {
			stopLogStreaming();
			await finalizeDeploy(id);
		});

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

	const isFork = $derived($page.url.searchParams.get('fork') === '1');

	onMount(async () => {
		const appId = $page.url.searchParams.get('id');
		const fork = $page.url.searchParams.get('fork') === '1';
		if (!appId) return;

		try {
			isDeploying = true;
			let yamlContent = await getComposeApp(appId);

			if (fork) {
				// Strip store_app_id so the forked app is treated as a Custom App.
				// Append "-custom" suffix to the project name so the user
				// gets a distinct identity on the next install.
				try {
					const parsed = yaml.load(yamlContent) as Record<string, unknown>;
					deletePowerLabExtProperty(parsed, 'store_app_id');
					if (typeof parsed?.name === 'string') {
						parsed.name = `${parsed.name}-custom`;
					}
					yamlContent = yaml.dump(parsed, { indent: 2, lineWidth: -1 });
				} catch {
					// If parsing fails, use the YAML as-is and let the
					// user fix it before deploy.
				}
			}

			yamlText = yamlContent;
		} catch (e) {
			error = t('orchestrator.loadAppFailed', { error: (e as Error).message });
		} finally {
			isDeploying = false;
		}
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
			const parsed = yaml.load(yamlText) as Record<string, unknown> | null;
			const services = parsed?.services as Record<string, unknown> | undefined;
			const id =
				(typeof parsed?.name === 'string' && parsed.name) ||
				(services && Object.keys(services)[0]) ||
				'app';
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
			if (editingId && !fork) {
				await applyComposeAppSettings(editingId, yamlText);
			} else {
				await installComposeApp(yamlText);
			}

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

	async function finalizeDeploy(id: string) {
		try {
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
	// 'deploying' state with a timeout surface.
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
</script>

<svelte:head>
	<title>{t('orchestrator.newCustomApp')} — PowerLab</title>
</svelte:head>

<div class="flex h-full flex-col text-zinc-50 font-sans antialiased">
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
					{isFork
						? t('orchestrator.forkCustomApp')
						: $page.url.searchParams.get('id')
							? t('orchestrator.editCustomApp')
							: t('orchestrator.newCustomApp')}
				</h1>
				<p class="text-[10px] font-medium uppercase tracking-[0.2em] text-zinc-500">
					{t('orchestrator.builder')}
				</p>
			</div>
		</div>

		<button
			onclick={handleDeploy}
			disabled={isDeploying || !!nameValidationError}
			title={nameValidationError ?? undefined}
			data-testid="deploy-button"
			class="flex h-9 items-center gap-2 rounded-xl bg-white px-4 text-xs font-bold text-black transition-transform hover:scale-105 active:scale-95 disabled:cursor-not-allowed disabled:scale-100 disabled:opacity-50"
		>
			{#if isDeploying}
				<Loader2 class="h-3.5 w-3.5 animate-spin" />
			{:else}
				<Play class="h-3.5 w-3.5 fill-black" />
			{/if}
			{t('orchestrator.deploy')}
		</button>
	</header>

	{#if error}
		<div
			class="flex items-center gap-3 border-b border-red-500/20 bg-red-500/10 px-6 py-2 text-xs text-red-400"
		>
			<AlertCircle class="h-3.5 w-3.5 shrink-0" />
			<span class="font-mono">{error}</span>
			<button
				onclick={() => (error = null)}
				class="ml-auto text-red-400/60 hover:text-red-400"
				aria-label="Dismiss error"
			>
				<X class="h-3.5 w-3.5" />
			</button>
		</div>
	{/if}

	<InstallModal
		phase={deployTimedOut
			? 'timeout'
			: deployResult?.success
				? 'success'
				: deployResult
					? 'error'
					: isDeploying
						? 'starting'
						: 'idle'}
		currentPhase={currentDeployPhase}
		progress={deployProgress}
		logs={deployJoinedLogs}
		appTitle={appTitleForModal}
		appIcon={appIconForModal}
		error={deployResult?.success === false ? deployResult.message : null}
		portNote={null}
		minimized={deployMinimized}
		onMinimize={() => {
			deployMinimized = true;
			goto('/');
		}}
		onCancel={() => {
			deployResult = null;
			isDeploying = false;
		}}
		onRetry={() => {
			deployResult = null;
		}}
		onOpen={() => {
			deployResult = null;
			goto('/');
		}}
		onStay={() => {
			deployResult = null;
		}}
		onCheckLaunchpad={() => {
			deployResult = null;
			goto('/');
		}}
	/>

	<main class="flex-1 overflow-hidden">
		<div class="flex h-full">
			<!-- YAML editor -->
			<div class="flex h-full w-1/2 flex-col bg-[#08080a]">
				<div class="flex h-10 items-center justify-between border-b border-white/5 bg-white/[0.02] px-4">
					<div class="flex items-center gap-1.5">
						<button
							onclick={() => goto('/')}
							aria-label={t('orchestrator.exitToApps')}
							class="group relative h-3 w-3 rounded-full bg-red-500/80 transition-colors hover:bg-red-500"
						>
							<X class="absolute left-1/2 top-1/2 h-1.5 w-1.5 -translate-x-1/2 -translate-y-1/2 text-black opacity-0 transition-opacity group-hover:opacity-100" />
						</button>
						<span class="h-3 w-3 rounded-full bg-amber-500 opacity-50"></span>
						<span class="h-3 w-3 rounded-full bg-emerald-500 opacity-50"></span>
					</div>
					<div class="text-[10px] font-bold uppercase tracking-widest text-zinc-600">
						docker-compose.yml
					</div>
					<div class="w-12"></div>
				</div>

				<div class="flex-1 overflow-hidden">
					<textarea
						bind:value={yamlText}
						spellcheck="false"
						aria-label={t('orchestrator.yamlEditor')}
						data-testid="yaml-editor"
						class={cn(
							'h-full w-full resize-none bg-transparent p-8 font-mono text-sm text-emerald-500/90 outline-none selection:bg-emerald-500/20 selection:text-white custom-scrollbar'
						)}
					></textarea>
				</div>
			</div>

			<!-- Read-only summary derived from yamlText. Replaces the
				 bidirectional ComposeForm. -->
			<div class="h-full w-1/2 overflow-y-auto border-l border-white/5 bg-zinc-950/40 custom-scrollbar">
				<YAMLPreview yaml={yamlText} />
			</div>
		</div>
	</main>
</div>

<style>
	.custom-scrollbar::-webkit-scrollbar {
		width: 6px;
	}
	.custom-scrollbar::-webkit-scrollbar-track {
		background: transparent;
	}
	.custom-scrollbar::-webkit-scrollbar-thumb {
		background: rgba(255, 255, 255, 0.05);
		border-radius: 10px;
	}
	.custom-scrollbar::-webkit-scrollbar-thumb:hover {
		background: rgba(255, 255, 255, 0.1);
	}
</style>
