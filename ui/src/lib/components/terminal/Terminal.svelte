<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { Terminal as XTerm } from 'xterm';
	import { FitAddon } from 'xterm-addon-fit';
	import { X, Maximize2, Minimize2 } from 'lucide-svelte';
	import 'xterm/css/xterm.css';

	// Local-pty terminal. No credentials, no SSH, no Remote-Login config.
	// PowerLab is already running with the user's privileges; the JWT in
	// localStorage authorises this session. The backend's /v1/sys/wsshell
	// endpoint spawns $SHELL (bash/sh) directly via creack/pty.
	let {
		isOpen = $bindable(false),
		onClose
	} = $props<{
		isOpen: boolean;
		onClose: () => void;
	}>();

	let terminalContainer = $state<HTMLElement | undefined>();
	let terminal: XTerm | null = null;
	let fitAddon: FitAddon | null = null;
	let ws: WebSocket | null = null;
	let isFullscreen = $state(false);

	function toggleFullscreen() {
		isFullscreen = !isFullscreen;
		setTimeout(() => {
			if (fitAddon) fitAddon.fit();
		}, 50);
	}

	function closeTerminal() {
		if (ws) {
			ws.close();
			ws = null;
		}
		if (terminal) {
			terminal.dispose();
			terminal = null;
		}
		onClose();
	}

	$effect(() => {
		if (isOpen && terminalContainer && !terminal) {
			// Initialize Xterm.js
			terminal = new XTerm({
				cursorBlink: true,
				theme: {
					background: '#09090b', // zinc-950
					foreground: '#e4e4e7', // zinc-200
					cursor: '#10b981', // emerald-500
					black: '#000000',
					red: '#ef4444',
					green: '#10b981',
					yellow: '#f59e0b',
					blue: '#3b82f6',
					magenta: '#d946ef',
					cyan: '#06b6d4',
					white: '#ffffff',
				},
				fontFamily: 'Menlo, Monaco, "Courier New", monospace',
				fontSize: 14,
				scrollback: 5000
			});

			fitAddon = new FitAddon();
			terminal.loadAddon(fitAddon);
			terminal.open(terminalContainer);
			
			// Initial fit
			setTimeout(() => {
				if (fitAddon && terminal) {
					fitAddon.fit();
					connectWebSocket();
				}
			}, 50);

			// Handle Window Resize
			const resizeObserver = new ResizeObserver(() => {
				if (fitAddon) fitAddon.fit();
			});
			resizeObserver.observe(terminalContainer);

			return () => {
				resizeObserver.disconnect();
			};
		}
	});

	function connectWebSocket() {
		if (!terminal) return;

		const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
		const host = window.location.host;
		// Local pty endpoint — no SSH, no credentials. Vite proxies /v1.
		const wsUrl = `${protocol}//${host}/v1/sys/wsshell?cols=${terminal.cols}&rows=${terminal.rows}`;

		ws = new WebSocket(wsUrl);
		ws.binaryType = 'arraybuffer';

		ws.onmessage = (event) => {
			if (event.data instanceof ArrayBuffer) {
				terminal?.write(new Uint8Array(event.data));
			} else {
				terminal?.write(event.data);
			}
		};

		ws.onclose = () => {
			terminal?.writeln('\r\n\x1b[2m── session ended ──\x1b[0m');
		};

		ws.onerror = () => {
			terminal?.writeln('\r\n\x1b[31mLost connection to PowerLab. Is the backend running?\x1b[0m');
		};

		// Forward typed input to pty stdin
		terminal.onData((data) => {
			if (ws && ws.readyState === WebSocket.OPEN) {
				ws.send(data);
			}
		});

		// Forward resize events to pty (resize protocol: \x04cols,rows)
		terminal.onResize(({ cols, rows }) => {
			if (ws && ws.readyState === WebSocket.OPEN) {
				ws.send(`\x04${cols},${rows}`);
			}
		});

	}

	onDestroy(() => {
		if (ws) ws.close();
		if (terminal) terminal.dispose();
	});
</script>

{#if isOpen}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm transition-all">
		<!-- Terminal Window -->
		<div class="flex flex-col overflow-hidden border border-zinc-800 bg-zinc-950 shadow-2xl transition-all duration-300 {isFullscreen ? 'fixed inset-0 rounded-none' : 'h-[80vh] w-[80vw] max-w-6xl rounded-xl'}">
			
			<!-- Title Bar -->
			<div class="flex h-12 shrink-0 items-center justify-between border-b border-zinc-900 bg-zinc-900/50 px-4">
				<div class="flex items-center gap-3">
					<div class="flex items-center gap-1.5">
						<button class="h-3 w-3 rounded-full bg-red-500 transition-colors hover:bg-red-600" onclick={closeTerminal} title="Close"></button>
						<button class="h-3 w-3 rounded-full bg-yellow-500 transition-colors hover:bg-yellow-600" title="Minimize (Not Supported)"></button>
						<button class="h-3 w-3 rounded-full bg-emerald-500 transition-colors hover:bg-emerald-600" onclick={toggleFullscreen} title="Toggle Fullscreen"></button>
					</div>
					<div class="ml-4 flex items-center gap-2 text-xs font-medium text-zinc-400">
						<span class="text-emerald-500">root</span>
						<span class="text-zinc-600">@</span>
						<span class="text-zinc-300">powerlab</span>
					</div>
				</div>
				<div class="flex items-center gap-2">
					<button class="rounded-md p-1.5 text-zinc-400 transition-colors hover:bg-zinc-800 hover:text-white" onclick={toggleFullscreen}>
						{#if isFullscreen}
							<Minimize2 class="h-4 w-4" />
						{:else}
							<Maximize2 class="h-4 w-4" />
						{/if}
					</button>
					<button class="rounded-md p-1.5 text-zinc-400 transition-colors hover:bg-red-500/20 hover:text-red-400" onclick={closeTerminal}>
						<X class="h-4 w-4" />
					</button>
				</div>
			</div>

			<!-- Terminal Container -->
			<div class="flex-1 bg-[#09090b] p-2" bind:this={terminalContainer}></div>
		</div>
	</div>
{/if}

<style>
	/* Make sure xterm takes full height and hides its own scrollbar */
	:global(.xterm) {
		height: 100%;
		padding: 8px;
	}
	:global(.xterm-viewport) {
		/* Custom scrollbar for xterm */
		scrollbar-width: thin;
		scrollbar-color: #3f3f46 transparent;
	}
	:global(.xterm-viewport::-webkit-scrollbar) {
		width: 8px;
	}
	:global(.xterm-viewport::-webkit-scrollbar-thumb) {
		background-color: #3f3f46;
		border-radius: 4px;
	}
</style>
