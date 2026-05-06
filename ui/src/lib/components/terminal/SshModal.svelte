<script lang="ts">
	import { Server, Key, Network } from 'lucide-svelte';

	let { isOpen = $bindable(false), onSubmit, onCancel } = $props<{
		isOpen: boolean;
		onSubmit: (credentials: { username: string; password: string; port: string }) => void;
		onCancel: () => void;
	}>();

	let username = $state('');
	let password = $state('');
	let port = $state('22');
	
	function handleSubmit(e: Event) {
		e.preventDefault();
		if (username && password && port) {
			onSubmit({ username, password, port });
		}
	}
</script>

{#if isOpen}
	<!-- Backdrop -->
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm transition-opacity">
		
		<!-- Modal Dialog -->
		<div class="relative w-full max-w-sm overflow-hidden rounded-2xl border border-zinc-800 bg-zinc-950 p-6 shadow-2xl animate-in fade-in zoom-in-95 duration-200">
			
			<div class="mb-6 flex flex-col items-center justify-center text-center">
				<div class="mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-emerald-500/10 text-emerald-500">
					<Server class="h-6 w-6 stroke-[2]" />
				</div>
				<h2 class="text-lg font-semibold tracking-tight text-white">Host Terminal Login</h2>
				<p class="mt-1 text-xs text-zinc-400">Enter SSH credentials for this server</p>
			</div>

			<form onsubmit={handleSubmit} class="flex flex-col gap-4">
				
				<div class="space-y-1">
					<label for="username" class="text-xs font-medium uppercase tracking-wider text-zinc-500">Username</label>
					<div class="relative">
						<div class="absolute inset-y-0 left-0 flex items-center pl-3 text-zinc-400">
							<Server class="h-4 w-4" />
						</div>
						<input 
							type="text" 
							id="username" 
							bind:value={username}
							class="w-full rounded-lg border border-zinc-800 bg-zinc-900 py-2 pl-10 pr-3 text-sm text-white placeholder-zinc-500 transition-colors focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500" 
							placeholder="root" 
							required
						/>
					</div>
				</div>

				<div class="flex gap-4">
					<div class="space-y-1 flex-1">
						<label for="password" class="text-xs font-medium uppercase tracking-wider text-zinc-500">Password</label>
						<div class="relative">
							<div class="absolute inset-y-0 left-0 flex items-center pl-3 text-zinc-400">
								<Key class="h-4 w-4" />
							</div>
							<input 
								type="password" 
								id="password" 
								bind:value={password}
								class="w-full rounded-lg border border-zinc-800 bg-zinc-900 py-2 pl-10 pr-3 text-sm text-white placeholder-zinc-500 transition-colors focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500" 
								placeholder="••••••••" 
								required
							/>
						</div>
					</div>

					<div class="space-y-1 w-24">
						<label for="port" class="text-xs font-medium uppercase tracking-wider text-zinc-500">Port</label>
						<div class="relative">
							<div class="absolute inset-y-0 left-0 flex items-center pl-3 text-zinc-400">
								<Network class="h-4 w-4" />
							</div>
							<input 
								type="text" 
								id="port" 
								bind:value={port}
								class="w-full rounded-lg border border-zinc-800 bg-zinc-900 py-2 pl-9 pr-2 text-sm text-white transition-colors focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500" 
								required
							/>
						</div>
					</div>
				</div>

				<div class="mt-4 flex gap-3">
					<button 
						type="button" 
						onclick={onCancel}
						class="flex-1 rounded-lg border border-zinc-800 bg-transparent px-4 py-2.5 text-sm font-medium text-zinc-300 transition-colors hover:bg-zinc-800 hover:text-white"
					>
						Cancel
					</button>
					<button 
						type="submit" 
						class="flex-1 rounded-lg bg-emerald-500 px-4 py-2.5 text-sm font-medium text-zinc-950 transition-all hover:bg-emerald-400 hover:shadow-[0_0_12px_rgba(16,185,129,0.4)]"
					>
						Connect
					</button>
				</div>
			</form>
			
		</div>
	</div>
{/if}
