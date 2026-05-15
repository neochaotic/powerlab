<script lang="ts">
	import { onMount, onDestroy, untrack } from 'svelte';
	import {
		ArrowLeft,
		Play,
		Code,
		LayoutDashboard,
		AlertCircle,
		X,
		Loader2
	} from 'lucide-svelte';
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
	import ComposeForm from '$lib/components/orchestrator/ComposeForm.svelte';
	import InstallModal from '$lib/components/apps/InstallModal.svelte';
	import { useAppStore } from '$lib/stores/apps.svelte';

	const appStore = useAppStore();
	import { page } from '$app/stores';
	import { toast } from '$lib/stores/toast.svelte';
	import { goto } from '$app/navigation';
	import { t } from '$lib/i18n/index.svelte';
	import { parseLatestPhase, phaseProgress } from '$lib/utils/install-phase';
	import { validateComposeName } from '$lib/utils/compose-name';
	import { viewFromYaml } from '$lib/utils/compose-mutate';

	/**
	 * Custom-app builder, one-way data flow per ADR-0030.
	 *
	 * yamlText is the SOLE source of truth. ComposeForm renders a
	 * `$derived(viewFromYaml(yaml))` view (read-only) and emits a NEW
	 * YAML on every field edit via `onChange`. Long-form
	 * volume/port/device entries keep their shape per-entry — the
	 * mutator detects string vs object and writes back in kind.
	 * Replaces the prior bidirectional `$bindable` model that
	 * collapsed long-form to "[object Object]" on round-trip (#332).
	 */

	let yamlText = $state(`name: my-app
services:
  web:
    image: nginx:latest
    ports:
      - "80:80"
    restart: always
`);

	let activeView = $state<'split' | 'form' | 'yaml'>('split');
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

	const view = $derived(viewFromYaml(yamlText));

	const nameValidationError = $derived.by<string | null>(() => {
		// Validate the SERVICE key (services.<key>) — the original
		// form's `name` field bound to this. Project name (top-level
		// `name:`) is informational and may be absent / different.
		const err = validateComposeName(view.serviceName);
		if (err === 'empty') return t('form.nameRequired');
		if (err === 'invalid_chars') return t('form.nameInvalidChars');
		return null;
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
			if (event.data) deployLogs = [...deployLogs, event.data];
		};
		eventSource.onerror = () => stopLogStreaming();
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
				try {
					const parsed = yaml.load(yamlContent) as Record<string, unknown>;
					deletePowerLabExtProperty(parsed, 'store_app_id');
					if (typeof parsed?.name === 'string') {
						parsed.name = `${parsed.name}-custom`;
					}
					yamlContent = yaml.dump(parsed, { indent: 2, lineWidth: -1 });
				} catch {
					// If parsing fails, use the YAML as-is and let the user fix it.
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

			const editingId = $page.url.searchParams.get('id');
			const fork = $page.url.searchParams.get('fork') === '1';
			if (editingId && !fork) {
				await applyComposeAppSettings(editingId, yamlText);
			} else {
				await installComposeApp(yamlText);
			}
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
				deployResult = { success: true, message: t('orchestrator.serviceRunning') };
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

	$effect(() => {
		if (!isDeploying) return;
		const timer = setTimeout(() => {
			if (isDeploying && !deployResult) {
				untrack(() => {
					deployResult = { success: false, message: t('orchestrator.deployTimedOut') };
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

		<div class="flex items-center gap-3">
			<!-- View switcher -->
			<div class="mr-4 flex items-center gap-1 rounded-xl bg-white/5 p-1">
				<button
					onclick={() => (activeView = 'split')}
					aria-label={t('orchestrator.viewSplit')}
					class={cn(
						'flex h-7 items-center gap-2 rounded-lg px-3 text-[10px] font-bold uppercase tracking-widest transition-all',
						activeView === 'split'
							? 'bg-white text-black shadow-lg'
							: 'text-zinc-500 hover:text-white'
					)}
				>
					<LayoutDashboard class="h-3 w-3" />
					{t('orchestrator.viewSplit')}
				</button>
				<button
					onclick={() => (activeView = 'form')}
					aria-label={t('orchestrator.viewForm')}
					class={cn(
						'flex h-7 items-center gap-2 rounded-lg px-3 text-[10px] font-bold uppercase tracking-widest transition-all',
						activeView === 'form'
							? 'bg-white text-black shadow-lg'
							: 'text-zinc-500 hover:text-white'
					)}
				>
					{t('orchestrator.viewForm')}
				</button>
				<button
					onclick={() => (activeView = 'yaml')}
					aria-label={t('orchestrator.viewYaml')}
					class={cn(
						'flex h-7 items-center gap-2 rounded-lg px-3 text-[10px] font-bold uppercase tracking-widest transition-all',
						activeView === 'yaml'
							? 'bg-white text-black shadow-lg'
							: 'text-zinc-500 hover:text-white'
					)}
				>
					<Code class="h-3 w-3" />
					{t('orchestrator.viewYaml')}
				</button>
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
		</div>
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
		appTitle={view.serviceName || view.projectName || 'app'}
		appIcon={view.icon}
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
			<!-- Form panel -->
			{#if activeView === 'split' || activeView === 'form'}
				<div
					class={cn(
						'h-full overflow-y-auto border-r border-white/5 custom-scrollbar transition-all duration-500',
						activeView === 'split' ? 'w-1/2' : 'w-full'
					)}
				>
					<div class="mx-auto max-w-2xl p-8">
						<ComposeForm
							yaml={yamlText}
							onChange={(newYaml) => (yamlText = newYaml)}
							nameError={nameValidationError}
						/>
					</div>
				</div>
			{/if}

			<!-- YAML editor panel -->
			{#if activeView === 'split' || activeView === 'yaml'}
				<div
					class={cn(
						'h-full flex flex-col bg-[#08080a] transition-all duration-500',
						activeView === 'split' ? 'w-1/2' : 'w-full'
					)}
				>
					<div
						class="flex h-10 items-center justify-between border-b border-white/5 bg-white/[0.02] px-4"
					>
						<div class="flex items-center gap-1.5">
							<button
								onclick={() => goto('/')}
								aria-label={t('orchestrator.exitToApps')}
								class="group relative h-3 w-3 rounded-full bg-red-500/80 transition-colors hover:bg-red-500"
							>
								<X
									class="absolute left-1/2 top-1/2 h-1.5 w-1.5 -translate-x-1/2 -translate-y-1/2 text-black opacity-0 transition-opacity group-hover:opacity-100"
								/>
							</button>
							<button
								onclick={() => (activeView = activeView === 'split' ? 'yaml' : 'split')}
								aria-label={t('orchestrator.toggleSplit')}
								class="h-3 w-3 rounded-full bg-amber-500"
							></button>
							<button
								onclick={() => (activeView = 'form')}
								aria-label={t('orchestrator.switchToForm')}
								class="h-3 w-3 rounded-full bg-emerald-500"
							></button>
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
							class="h-full w-full resize-none bg-transparent p-8 font-mono text-sm text-emerald-500/90 outline-none selection:bg-emerald-500/20 selection:text-white custom-scrollbar"
						></textarea>
					</div>
				</div>
			{/if}
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
