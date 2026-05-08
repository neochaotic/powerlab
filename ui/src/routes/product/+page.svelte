<script lang="ts">
	import { fade, fly, slide } from 'svelte/transition';
	import { Button } from '$lib/components/ui/button';
	import { 
		Boxes, Cpu, Zap, Shield, Zap as Flash, 
		Activity, Globe, Terminal, LayoutDashboard, 
		ChevronRight, Download, Play, CheckCircle2 
	} from 'lucide-svelte';
	import { onMount } from 'svelte';

	let mounted = $state(false);
	onMount(() => {
		mounted = true;
	});

	const features = [
		{
			icon: Cpu,
			title: "Real-time Telemetry",
			desc: "Monitor CPU, RAM, GPU, and network throughput with millisecond precision and EMA smoothing."
		},
		{
			icon: Boxes,
			title: "Docker Orchestrator",
			desc: "Manage containers with a visual form builder or a professional YAML editor with bi-directional sync."
		},
		{
			icon: Globe,
			title: "PWA Ready",
			desc: "Your home server, available anywhere. Install PowerLab to your home screen for a native experience."
		},
		{
			icon: Shield,
			title: "Hardened Security",
			desc: "Bcrypt password hashing, JWT-based sessions, and production-ready SystemD service integration."
		}
	];
</script>

<svelte:head>
	<title>PowerLab — High Performance Home Server OS</title>
	<link href="https://fonts.googleapis.com/css2?family=Outfit:wght@400;600;800&display=swap" rel="stylesheet" />
</svelte:head>

<div class="h-full w-full overflow-y-auto overflow-x-hidden bg-zinc-950 font-sans text-zinc-100 selection:bg-emerald-500/30">
	<!-- Glass Navbar -->
	<nav class="sticky top-0 z-50 flex h-20 w-full items-center justify-between border-b border-white/5 bg-zinc-950/50 px-8 backdrop-blur-xl md:px-16">
		<div class="flex items-center gap-3">
			<div class="flex h-10 w-10 items-center justify-center rounded-xl bg-white/[0.03] border border-white/10 shadow-inner">
				<span class="text-xl font-black text-white leading-none">P<span class="text-emerald-500">.</span></span>
			</div>
			<span class="text-xl font-black tracking-tighter text-white">PowerLab<span class="text-emerald-500">.</span></span>
		</div>
		
		<div class="hidden items-center gap-8 md:flex">
			<a href="#features" class="text-sm font-bold text-zinc-400 transition-colors hover:text-white">Features</a>
			<a href="#telemetry" class="text-sm font-bold text-zinc-400 transition-colors hover:text-white">Telemetry</a>
			<a href="https://github.com/neochaotic/powerlab" class="text-sm font-bold text-zinc-400 transition-colors hover:text-white">GitHub</a>
		</div>

		<Button variant="outline" class="h-10 rounded-xl border-white/10 bg-white/5 font-bold hover:bg-emerald-500 hover:text-zinc-950 transition-all" onclick={() => window.location.href = '/'}>
			Sign In <ChevronRight class="ml-2 h-4 w-4" />
		</Button>
	</nav>

	<!-- Hero Section -->
	<section class="relative flex min-h-[90vh] flex-col items-center justify-center px-8 py-24 text-center md:px-16">
		<!-- Background Glows -->
		<div class="absolute left-1/2 top-1/2 -z-10 h-[500px] w-[500px] -translate-x-1/2 -translate-y-1/2 rounded-full bg-emerald-500/10 blur-[120px]"></div>
		<div class="absolute left-[20%] top-[30%] -z-10 h-[300px] w-[300px] rounded-full bg-blue-500/5 blur-[100px]"></div>

		{#if mounted}
			<div in:fly={{ y: 40, duration: 800, delay: 100 }}>
				<div class="mb-8 inline-flex items-center gap-2 rounded-full border border-emerald-500/20 bg-emerald-500/5 px-4 py-1.5 backdrop-blur-md">
					<Flash class="h-4 w-4 text-emerald-500" />
					<span class="text-[10px] font-black uppercase tracking-[0.2em] text-emerald-500">Now with Svelte 5 & Go</span>
				</div>
				
				<h1 class="mx-auto mb-6 max-w-4xl font-['Outfit'] text-5xl font-extrabold leading-[1.1] tracking-tight text-white md:text-8xl">
					The High-Performance <br/>
					<span class="bg-gradient-to-r from-emerald-400 to-cyan-400 bg-clip-text text-transparent">Home Server OS.</span>
				</h1>

				<p class="mx-auto mb-12 max-w-2xl text-lg font-medium leading-relaxed text-zinc-400 md:text-xl">
					Manage your infrastructure with a Silicon Valley-grade interface. 
					Minimalist, lightning-fast, and built for edge computing.
				</p>

				<div class="flex flex-col items-center justify-center gap-4 sm:flex-row">
					<Button class="h-14 rounded-2xl bg-emerald-500 px-8 text-sm font-black uppercase tracking-widest text-zinc-950 transition-all hover:bg-emerald-400 hover:shadow-[0_0_40px_rgba(16,185,129,0.3)] active:scale-95">
						Get Started Now
					</Button>
					<Button variant="ghost" class="h-14 rounded-2xl border border-white/5 bg-white/5 px-8 text-sm font-black uppercase tracking-widest text-white transition-all hover:bg-white/10 active:scale-95">
						View on GitHub
					</Button>
				</div>
			</div>
		{/if}

		<!-- Dashboard Preview -->
		{#if mounted}
			<div class="relative mt-24 w-full max-w-6xl px-4" in:fly={{ y: 60, duration: 1000, delay: 400 }}>
				<div class="relative overflow-hidden rounded-[2.5rem] border border-white/10 bg-zinc-900/50 p-4 shadow-2xl backdrop-blur-md">
					<div class="aspect-[16/9] w-full rounded-[1.5rem] bg-zinc-950/80 p-8 shadow-inner overflow-hidden flex flex-col">
						<div class="flex items-center justify-between mb-12">
							<div class="flex gap-2">
								<div class="h-3 w-3 rounded-full bg-red-500/20 border border-red-500/50"></div>
								<div class="h-3 w-3 rounded-full bg-amber-500/20 border border-amber-500/50"></div>
								<div class="h-3 w-3 rounded-full bg-emerald-500/20 border border-emerald-500/50"></div>
							</div>
							<div class="px-3 py-1 rounded-full bg-white/[0.03] border border-white/5 text-[10px] font-bold text-zinc-500 uppercase tracking-widest">
								Dashboard Cockpit
							</div>
						</div>
						
						<div class="grid grid-cols-1 md:grid-cols-3 gap-8 flex-1">
							<div class="rounded-3xl bg-white/[0.02] border border-white/5 p-6 flex flex-col items-center justify-center group hover:bg-white/[0.04] transition-colors">
								<div class="h-32 w-32 rounded-full border-8 border-emerald-500/20 border-t-emerald-500 relative flex items-center justify-center">
									<span class="text-2xl font-black text-white">42%</span>
								</div>
								<span class="mt-4 text-[10px] font-black uppercase tracking-widest text-zinc-500">CPU Usage</span>
							</div>
							<div class="rounded-3xl bg-white/[0.02] border border-white/5 p-6 flex flex-col items-center justify-center group hover:bg-white/[0.04] transition-colors">
								<div class="h-32 w-32 rounded-full border-8 border-purple-500/20 border-t-purple-500 relative flex items-center justify-center">
									<span class="text-2xl font-black text-white">68%</span>
								</div>
								<span class="mt-4 text-[10px] font-black uppercase tracking-widest text-zinc-500">Memory Load</span>
							</div>
							<div class="rounded-3xl bg-white/[0.02] border border-white/5 p-6 flex flex-col items-center justify-center group hover:bg-white/[0.04] transition-colors">
								<div class="h-32 w-32 rounded-full border-8 border-blue-500/20 border-t-blue-500 relative flex items-center justify-center">
									<span class="text-2xl font-black text-white">12%</span>
								</div>
								<span class="mt-4 text-[10px] font-black uppercase tracking-widest text-zinc-500">Network Rate</span>
							</div>
						</div>
					</div>
				</div>
				<!-- Floating Decorative Elements -->
				<div class="absolute -right-8 -top-8 h-32 w-32 animate-bounce rounded-3xl bg-emerald-500/10 border border-emerald-500/20 backdrop-blur-xl hidden md:flex items-center justify-center">
					<Activity class="h-10 w-10 text-emerald-500" />
				</div>
				<div class="absolute -left-12 bottom-12 h-40 w-40 animate-pulse rounded-full bg-blue-500/10 border border-blue-500/20 backdrop-blur-xl hidden md:flex items-center justify-center">
					<Globe class="h-12 w-12 text-blue-500" />
				</div>
			</div>
		{/if}
	</section>

	<!-- Features Grid -->
	<section id="features" class="px-8 py-32 md:px-16 bg-zinc-900/30">
		<div class="mx-auto max-w-6xl">
			<div class="mb-24 text-center">
				<h2 class="mb-4 font-['Outfit'] text-4xl font-extrabold text-white md:text-5xl">Designed for Power Users.</h2>
				<p class="mx-auto max-w-xl text-lg text-zinc-500">
					Every feature is crafted with performance and reliability in mind.
				</p>
			</div>

			<div class="grid grid-cols-1 gap-8 md:grid-cols-2 lg:grid-cols-4">
				{#each features as feature, i}
					<div class="group flex flex-col rounded-[2rem] border border-white/5 bg-zinc-950 p-8 transition-all hover:bg-zinc-900 hover:-translate-y-2">
						<div class="mb-6 flex h-14 w-14 items-center justify-center rounded-2xl bg-white/[0.03] border border-white/10 text-emerald-500 group-hover:bg-emerald-500 group-hover:text-zinc-950 transition-all duration-500 shadow-inner">
							<feature.icon class="h-6 w-6" />
						</div>
						<h3 class="mb-3 text-lg font-bold text-white">{feature.title}</h3>
						<p class="text-sm font-medium leading-relaxed text-zinc-500 group-hover:text-zinc-400 transition-colors">
							{feature.desc}
						</p>
					</div>
				{/each}
			</div>
		</div>
	</section>

	<!-- Integration Section -->
	<section class="relative overflow-hidden px-8 py-32 md:px-16">
		<div class="absolute right-0 top-0 -z-10 h-[600px] w-[600px] rounded-full bg-blue-500/5 blur-[120px]"></div>
		<div class="mx-auto max-w-6xl flex flex-col md:flex-row items-center gap-24">
			<div class="flex-1">
				<div class="mb-6 inline-flex items-center gap-2 rounded-full border border-blue-500/20 bg-blue-500/5 px-4 py-1.5">
					<Terminal class="h-4 w-4 text-blue-500" />
					<span class="text-[10px] font-black uppercase tracking-[0.2em] text-blue-500">Developer First</span>
				</div>
				<h2 class="mb-8 font-['Outfit'] text-4xl font-extrabold leading-tight text-white md:text-6xl">
					Professional Orchesrtation at your fingertips.
				</h2>
				<div class="space-y-6">
					<div class="flex gap-4">
						<div class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-emerald-500 text-zinc-950 mt-1">
							<CheckCircle2 class="h-4 w-4" />
						</div>
						<div>
							<h4 class="font-bold text-white mb-1">CodeMirror 6 Editor</h4>
							<p class="text-sm text-zinc-500">Full syntax highlighting for Docker Compose, YAML, and JSON.</p>
						</div>
					</div>
					<div class="flex gap-4">
						<div class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-emerald-500 text-zinc-950 mt-1">
							<CheckCircle2 class="h-4 w-4" />
						</div>
						<div>
							<h4 class="font-bold text-white mb-1">Live Log Streaming</h4>
							<p class="text-sm text-zinc-500">Monitor installation progress with real-time SSE log streaming.</p>
						</div>
					</div>
					<div class="flex gap-4">
						<div class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-emerald-500 text-zinc-950 mt-1">
							<CheckCircle2 class="h-4 w-4" />
						</div>
						<div>
							<h4 class="font-bold text-white mb-1">Custom Form Builder</h4>
							<p class="text-sm text-zinc-500">Not comfortable with YAML? Use our visual builder with bi-directional sync.</p>
						</div>
					</div>
				</div>
			</div>
			
			<div class="flex-1 w-full">
				<div class="rounded-[2.5rem] border border-white/10 bg-zinc-950 p-2 shadow-2xl shadow-emerald-500/5">
					<div class="overflow-hidden rounded-[2rem] bg-zinc-900 border border-white/5">
						<div class="flex h-10 items-center border-b border-white/5 bg-zinc-950 px-4 gap-1.5">
							<div class="h-2 w-2 rounded-full bg-white/10"></div>
							<div class="h-2 w-2 rounded-full bg-white/10"></div>
							<div class="h-2 w-2 rounded-full bg-white/10"></div>
						</div>
						<pre class="p-8 font-mono text-sm leading-relaxed text-zinc-500">
<span class="text-emerald-500">services</span>:
  <span class="text-blue-500">powerlab-app</span>:
    <span class="text-zinc-300">image</span>: nginx:latest
    <span class="text-zinc-300">ports</span>:
      - <span class="text-amber-500">"8080:80"</span>
    <span class="text-zinc-300">volumes</span>:
      - <span class="text-amber-500">/DATA/AppData/web:/usr/share/nginx</span>
    <span class="text-zinc-300">x-powerlab</span>:
      <span class="text-zinc-300">title</span>:
        <span class="text-amber-500">en_us</span>: PowerLab Web
						</pre>
					</div>
				</div>
			</div>
		</div>
	</section>

	<!-- Final CTA -->
	<section class="px-8 py-32 md:px-16 text-center">
		<div class="mx-auto max-w-4xl rounded-[3rem] bg-gradient-to-b from-zinc-900 to-zinc-950 p-16 border border-white/10 shadow-2xl relative overflow-hidden">
			<div class="absolute inset-0 bg-emerald-500/5 blur-[100px] -z-10"></div>
			<h2 class="mb-6 font-['Outfit'] text-4xl font-extrabold text-white md:text-6xl">Ready to upgrade your lab?</h2>
			<p class="mx-auto mb-12 max-w-xl text-lg text-zinc-500">
				Install PowerLab in minutes and take control of your infrastructure with style.
			</p>
			<div class="flex flex-col items-center justify-center gap-4 sm:flex-row">
				<Button class="h-16 rounded-[1.5rem] bg-white px-12 text-sm font-black uppercase tracking-widest text-zinc-950 transition-all hover:bg-emerald-500 hover:text-zinc-950 hover:shadow-[0_0_40px_rgba(16,185,129,0.3)] active:scale-95" onclick={() => window.location.href = '/'}>
					Deploy Now
				</Button>
				<Button variant="ghost" class="h-16 rounded-[1.5rem] border border-white/5 bg-white/5 px-12 text-sm font-black uppercase tracking-widest text-white transition-all hover:bg-white/10 active:scale-95">
					View Installation
				</Button>
			</div>
		</div>
	</section>

	<!-- Footer -->
	<footer class="px-8 py-16 md:px-16 border-t border-white/5 bg-zinc-950">
		<div class="mx-auto max-w-6xl flex flex-col md:flex-row justify-between items-center gap-8">
			<div class="flex items-center gap-3">
				<div class="flex h-8 w-8 items-center justify-center rounded-lg bg-white/[0.03] border border-white/10">
					<span class="text-sm font-black text-white leading-none">P<span class="text-emerald-500">.</span></span>
				</div>
				<span class="text-sm font-black tracking-tighter text-white uppercase tracking-widest">PowerLab</span>
			</div>
			
			<div class="flex gap-8 text-[11px] font-bold text-zinc-600 uppercase tracking-[0.2em]">
				<a href="/" class="hover:text-white transition-colors">Privacy</a>
				<a href="/" class="hover:text-white transition-colors">Documentation</a>
				<a href="/" class="hover:text-white transition-colors">Support</a>
			</div>

			<p class="text-[11px] font-bold text-zinc-700 uppercase tracking-widest">
				&copy; {new Date().getFullYear()} <a href="https://github.com/neochaotic" target="_blank" rel="noopener noreferrer" class="hover:text-emerald-400 transition-colors">neochaotic</a>
			</p>
		</div>
	</footer>
</div>

<style>
	:global(html) {
		scroll-behavior: smooth;
	}

	/* Custom Glow Scrollbar */
	div::-webkit-scrollbar {
		width: 8px;
	}
	div::-webkit-scrollbar-track {
		background: #09090b;
	}
	div::-webkit-scrollbar-thumb {
		background: #18181b;
		border-radius: 4px;
		border: 2px solid #09090b;
	}
	div::-webkit-scrollbar-thumb:hover {
		background: #27272a;
	}
</style>
