<script lang="ts">
	import { AlertCircle, AlertTriangle, ExternalLink, Hash, KeyRound, RefreshCw, Download } from 'lucide-svelte';
	import { Button } from '$lib/components/ui/button';
	import { fade } from 'svelte/transition';
	import { cn } from '$lib/utils';
	import { t } from '$lib/i18n/index.svelte';
	import type { OS } from '$lib/utils/os';

	interface Props {
		activeSecurityTab: OS;
		onTabChange: (tab: OS) => void;
		isTestingConnection: boolean;
		isHttpsSelfSigned: boolean;
		onDownloadCA: (format: 'mobileconfig' | 'crt' | 'cer') => void;
		onOpenHttpDownload: (format: 'mobileconfig' | 'crt' | 'cer') => void;
		onTestHttpsConnection: () => void;
		onResetTrust: () => void;
		onConfirmRotateCA: () => void;
	}

	let {
		activeSecurityTab,
		onTabChange,
		isTestingConnection,
		isHttpsSelfSigned,
		onDownloadCA,
		onOpenHttpDownload,
		onTestHttpsConnection,
		onResetTrust,
		onConfirmRotateCA
	}: Props = $props();
</script>

<div>
	<header class="mb-8">
		<h1 class="text-2xl font-bold tracking-tight text-white">Security</h1>
		<p class="mt-1 text-sm text-zinc-500">HTTPS infrastructure and session management.</p>
	</header>

	<!-- HTTPS Onboarding (Issue #43) -->
	<section class="mb-10">
		<h3 class="mb-4 text-[10px] font-bold uppercase tracking-[0.2em] text-zinc-500">Local HTTPS Establishment</h3>
		<div class="overflow-hidden rounded-2xl border border-white/5 bg-white/[0.02]">
			<!-- Tabs Header -->
			<div class="flex border-b border-white/5 bg-white/[0.02]">
				{#each ['ios', 'macos', 'android', 'windows'] as tab}
					<button
						class={cn(
							"flex-1 px-4 py-3 text-[11px] font-bold uppercase tracking-wider transition-colors",
							activeSecurityTab === tab ? "bg-white/5 text-white shadow-[inset_0_-2px_0_white]" : "text-zinc-500 hover:text-zinc-300"
						)}
						onclick={() => onTabChange(tab as OS)}
					>
						{tab}
					</button>
				{/each}
			</div>

			<!-- Tab Content -->
			<div class="p-6">
				<div class="grid grid-cols-1 lg:grid-cols-2 gap-8">
					<div class="space-y-6">
						{#if activeSecurityTab === 'ios'}
							<div class="space-y-4" in:fade>
								<h4 class="text-lg font-semibold text-white">iOS Installation</h4>
								<ol class="space-y-3 list-decimal list-inside text-sm text-zinc-400">
									<li>Download the <button type="button" onclick={() => onDownloadCA('mobileconfig')} class="text-emerald-400 hover:underline cursor-pointer bg-transparent p-0 border-0">Security Profile</button>.</li>
									<li>Go to <strong>Settings → Profile Downloaded</strong> and click <strong>Install</strong>.</li>
									<li>Go to <strong>Settings → General → About → Certificate Trust Settings</strong>.</li>
									<li>Enable full trust for <strong>PowerLab Root CA</strong>.</li>
								</ol>
								<Button class="w-full bg-white text-zinc-950 font-bold" onclick={() => onDownloadCA('mobileconfig')}>
									<Download class="h-4 w-4 mr-2" />
									Download Profile
								</Button>
							</div>
						{:else if activeSecurityTab === 'macos'}
							<div class="space-y-4" in:fade>
								<h4 class="text-lg font-semibold text-white">macOS Installation</h4>
								<ol class="space-y-3 list-decimal list-inside text-sm text-zinc-400">
									<li>Download the <button type="button" onclick={() => onDownloadCA('mobileconfig')} class="text-emerald-400 hover:underline cursor-pointer bg-transparent p-0 border-0">Security Profile</button>.</li>
									<li>Open the profile and click <strong>Install</strong> in System Settings.</li>
									<li>Alternatively, use the <button type="button" onclick={() => onDownloadCA('crt')} class="text-emerald-400 hover:underline cursor-pointer bg-transparent p-0 border-0">CRT file</button> and set trust in Keychain Access.</li>
								</ol>
								<div class="flex gap-2">
									<Button variant="secondary" class="flex-1 font-bold" onclick={() => onDownloadCA('mobileconfig')}>.mobileconfig</Button>
									<Button variant="secondary" class="flex-1 font-bold" onclick={() => onDownloadCA('crt')}>.crt</Button>
								</div>
							</div>
						{:else if activeSecurityTab === 'android'}
							<div class="space-y-4" in:fade>
								<h4 class="text-lg font-semibold text-white">Android Installation</h4>
								<ol class="space-y-3 list-decimal list-inside text-sm text-zinc-400">
									<li>Download the <button type="button" onclick={() => onDownloadCA('crt')} class="text-emerald-400 hover:underline cursor-pointer bg-transparent p-0 border-0">CA Certificate</button>.</li>
									<li>Settings → Security → Encryption → Install from storage.</li>
									<li>Select <strong>CA certificate</strong> and pick the file.</li>
								</ol>
								<Button class="w-full bg-white text-zinc-950 font-bold" onclick={() => onDownloadCA('crt')}>
									<Download class="h-4 w-4 mr-2" />
									Download CRT
								</Button>
							</div>
						{:else if activeSecurityTab === 'windows'}
							<div class="space-y-4" in:fade>
								<h4 class="text-lg font-semibold text-white">Windows / Linux</h4>
								<ol class="space-y-3 list-decimal list-inside text-sm text-zinc-400">
									<li>Download the <button type="button" onclick={() => onDownloadCA('crt')} class="text-emerald-400 hover:underline cursor-pointer bg-transparent p-0 border-0">CA Certificate</button>.</li>
									<li>Right-click → Install → Local Machine.</li>
									<li>Place in <strong>Trusted Root Certification Authorities</strong>.</li>
									<li>On Linux: Copy to <code>/usr/local/share/ca-certificates/</code> and run <code>update-ca-certificates</code>.</li>
								</ol>
								<Button class="w-full bg-white text-zinc-950 font-bold" onclick={() => onDownloadCA('crt')}>
									<Download class="h-4 w-4 mr-2" />
									Download CRT
								</Button>
							</div>
						{/if}

						{#if isHttpsSelfSigned}
							<div class="pt-4 border-t border-white/5">
								<details class="group">
									<summary class="cursor-pointer flex items-center gap-2 text-[11px] font-bold uppercase tracking-wider text-amber-400/80 hover:text-amber-400">
										<AlertCircle class="h-3.5 w-3.5" />
										{t('settings.caBlockedTitle')}
										<span class="ml-auto text-zinc-600 transition-transform group-open:rotate-90">›</span>
									</summary>
									<div class="mt-3 space-y-3 text-[11px] leading-relaxed text-zinc-400">
										<div class="rounded-lg border border-emerald-400/20 bg-emerald-500/[0.05] p-3">
											<p class="font-semibold text-emerald-300 mb-1">{t('settings.caKeepHint')}</p>
											<p class="text-zinc-400">{t('settings.caKeepExplain')}</p>
										</div>

										<p class="text-zinc-500">{t('settings.caBlockedExplain')}</p>

										<button
											type="button"
											onclick={() => onOpenHttpDownload('crt')}
											class="flex items-center justify-center gap-2 rounded-lg border border-white/10 bg-white/[0.02] px-3 py-2 text-[11px] font-bold text-white hover:bg-white/[0.05] transition-colors w-full"
											title={t('settings.caOpenViaHttpHint')}
										>
											<ExternalLink class="h-3.5 w-3.5" />
											{t('settings.caOpenViaHttp')}
										</button>
									</div>
								</details>
							</div>
						{/if}

						<div class="pt-6 border-t border-white/5">
							<div class="flex items-center justify-between gap-4 p-4 rounded-xl bg-emerald-500/5 border border-emerald-500/10">
								<div class="min-w-0">
									<p class="text-sm font-semibold text-white">Verification</p>
									<p class="text-[11px] text-zinc-500 leading-relaxed">Test if your device trusts PowerLab.</p>
								</div>
								<button
									type="button"
									onclick={onTestHttpsConnection}
									disabled={isTestingConnection}
									class={cn(
										"shrink-0 inline-flex items-center gap-1.5 rounded-full px-3 py-1.5 text-[11px] font-bold transition-all",
										isTestingConnection
											? "bg-zinc-800 text-zinc-500 cursor-not-allowed"
											: "bg-emerald-500 text-zinc-950 hover:bg-emerald-400 active:scale-95"
									)}
								>
									{#if isTestingConnection}
										<RefreshCw class="h-3 w-3 animate-spin" />
										Testing…
									{:else}
										Test
									{/if}
								</button>
							</div>

							<!-- Reset / Rotate — recovery actions. Reset (light):
								 clear HSTS, keep CA. Rotate (destructive):
								 regenerate CA, void trust on every device. Both
								 behind a "Show advanced" so a casual user doesn't
								 accidentally tap them. -->
							<details class="mt-3 group">
								<summary class="cursor-pointer text-[11px] font-bold uppercase tracking-wider text-zinc-500 hover:text-zinc-300 list-none flex items-center gap-1.5 select-none">
									<span class="transition-transform group-open:rotate-90">›</span>
									Recovery
								</summary>
								<div class="mt-3 grid grid-cols-1 gap-2">
									<button
										type="button"
										onclick={onResetTrust}
										class="flex items-center justify-between gap-3 rounded-xl border border-white/[0.06] bg-white/[0.02] px-4 py-3 text-left transition-colors hover:border-amber-500/30 hover:bg-amber-500/[0.04]"
									>
										<div class="min-w-0">
											<p class="text-[12px] font-semibold text-white">Reset trust</p>
											<p class="text-[11px] text-zinc-500 leading-relaxed">Re-run the trust dance. CA stays the same.</p>
										</div>
										<RefreshCw class="h-3.5 w-3.5 text-zinc-500 shrink-0" />
									</button>
									<button
										type="button"
										onclick={onConfirmRotateCA}
										class="flex items-center justify-between gap-3 rounded-xl border border-white/[0.06] bg-white/[0.02] px-4 py-3 text-left transition-colors hover:border-rose-500/30 hover:bg-rose-500/[0.04]"
									>
										<div class="min-w-0">
											<p class="text-[12px] font-semibold text-white">Rotate CA</p>
											<p class="text-[11px] text-zinc-500 leading-relaxed">Generate a new CA. Voids trust on every installed device.</p>
										</div>
										<AlertTriangle class="h-3.5 w-3.5 text-rose-400 shrink-0" />
									</button>
								</div>
							</details>
						</div>
					</div>

					<!-- Visual Guide -->
					<div class="relative aspect-[4/3] overflow-hidden rounded-xl border border-white/5 bg-black/40">
						<img
							src={`/docs/security/${activeSecurityTab}.png`}
							alt={`${activeSecurityTab} installation guide`}
							class="h-full w-full object-cover opacity-80"
						/>
						<div class="absolute inset-0 bg-gradient-to-t from-zinc-950/60 to-transparent"></div>
						<p class="absolute bottom-4 left-4 text-[10px] font-bold uppercase tracking-widest text-white/40">Reference Mockup</p>
					</div>
				</div>
			</div>
		</div>
	</section>

	<section class="mb-8">
		<h3 class="mb-3 text-[10px] font-bold uppercase tracking-[0.2em] text-zinc-500">Account</h3>
		<div class="rounded-2xl border border-white/5 bg-white/[0.02] divide-y divide-white/5">
			<button class="flex w-full items-center justify-between gap-4 px-5 py-4 text-left transition-colors hover:bg-white/[0.02]">
				<div class="flex items-center gap-3">
					<KeyRound class="h-4 w-4 text-zinc-500" />
					<div>
						<p class="text-sm font-medium text-white">Change password</p>
						<p class="mt-0.5 text-xs text-zinc-500">Update your PowerLab login password</p>
					</div>
				</div>
				<span class="text-zinc-500">›</span>
			</button>
			<div class="flex items-center justify-between gap-4 px-5 py-4">
				<div class="flex items-center gap-3">
					<Hash class="h-4 w-4 text-zinc-500" />
					<div>
						<p class="text-sm font-medium text-white">Session timeout</p>
						<p class="mt-0.5 text-xs text-zinc-500">How long until automatic sign-out</p>
					</div>
				</div>
				<select class="rounded-lg border border-white/8 bg-white/[0.03] px-3 py-1.5 text-xs text-white outline-none focus:border-emerald-500/40">
					<option>30 minutes</option>
					<option>2 hours</option>
					<option selected>24 hours</option>
					<option>Never</option>
				</select>
			</div>
		</div>
	</section>

	<p class="text-[11px] text-zinc-600">PowerLab uses a bcrypt-hashed local user. OS-level authentication (PAM/dscl) is on the roadmap.</p>
</div>
