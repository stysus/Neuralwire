<script lang="ts">
	import { onMount } from 'svelte';

	let draftsCount = $state(0);
	let publishedCount = $state(0);
	let rejectedCount = $state(0);

	let isLoading = $state(true);
	let errorMessage = $state('');
	let serverTime = $state('');

	async function fetchStats() {
		const token = localStorage.getItem('admin_token');
		if (!token) return;

		try {
			const headers = {
				Authorization: `Bearer ${token}`,
				Accept: 'application/json'
			};

			// Query parallel count stats using page_size=1
			const [resDraft, resPub, resRej] = await Promise.all([
				fetch('http://localhost:8080/api/admin/news?status=draft&page_size=1', { headers }),
				fetch('http://localhost:8080/api/admin/news?status=published&page_size=1', { headers }),
				fetch('http://localhost:8080/api/admin/news?status=rejected&page_size=1', { headers })
			]);

			if (resDraft.ok && resPub.ok && resRej.ok) {
				const [dataDraft, dataPub, dataRej] = await Promise.all([
					resDraft.json(),
					resPub.json(),
					resRej.json()
				]);

				draftsCount = dataDraft.pagination.total || 0;
				publishedCount = dataPub.pagination.total || 0;
				rejectedCount = dataRej.pagination.total || 0;
			} else {
				if (resDraft.status === 401 || resDraft.status === 403) {
					// Session expired
					localStorage.removeItem('admin_token');
					window.location.reload();
				}
				errorMessage = 'Failed to fetch terminal records.';
			}
		} catch (err) {
			console.error('Stats request failed:', err);
			errorMessage = 'Database node connection timeout.';
		} finally {
			isLoading = false;
		}
	}

	onMount(() => {
		fetchStats();
		serverTime = new Date().toISOString();

		// Update time telemetry
		const timer = setInterval(() => {
			serverTime = new Date().toISOString();
		}, 1000);

		return () => clearInterval(timer);
	});
</script>

<svelte:head>
	<title>System Dashboard | Neuralwire</title>
</svelte:head>

<section
	class="mx-auto flex max-w-7xl flex-grow flex-col justify-center px-4 py-12 sm:px-6 md:py-16 lg:px-8"
>
	<!-- Header telemetry -->
	<div
		class="mb-12 flex flex-col items-start justify-between gap-4 border-b border-[rgba(255,255,255,0.08)] pb-6 sm:flex-row sm:items-end"
	>
		<div>
			<span class="tag-mono mb-1 block text-xs font-bold tracking-widest text-[#22D3EE]"
				>SYSTEM // OVERVIEW</span
			>
			<h1 class="font-serif text-3xl font-medium text-white">TERMINAL CONTROL</h1>
		</div>
		<div class="space-y-0.5 text-right font-mono text-[10px] text-slate-500">
			<div>SYS_TIME: <span class="text-slate-300">{serverTime}</span></div>
			<div>API_STATUS: <span class="font-bold text-[#22D3EE]">ONLINE</span></div>
		</div>
	</div>

	{#if isLoading}
		<div class="flex flex-grow items-center justify-center py-12">
			<div class="space-y-4 text-center font-mono text-xs">
				<div
					class="mx-auto h-6 w-6 animate-spin rounded-full border-2 border-slate-800 border-t-[#22D3EE]"
				></div>
				<div class="animate-pulse tracking-widest text-slate-500 uppercase">
					Retrieving Core Metrics...
				</div>
			</div>
		</div>
	{:else if errorMessage}
		<div
			class="mx-auto max-w-md rounded-xl border border-[#E11D48]/30 bg-[#E11D48]/5 p-8 text-center"
		>
			<svg
				class="mx-auto mb-3 h-8 w-8 animate-pulse text-[#E11D48]"
				fill="none"
				viewBox="0 0 24 24"
				stroke="currentColor"
			>
				<path
					stroke-linecap="round"
					stroke-linejoin="round"
					stroke-width="2"
					d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
				/>
			</svg>
			<p class="mb-2 font-mono text-xs tracking-wider text-[#E11D48] uppercase">
				TELEMETRY FAILURE
			</p>
			<p class="font-sans text-xs leading-relaxed text-slate-400">{errorMessage}</p>
			<button
				onclick={() => {
					isLoading = true;
					fetchStats();
				}}
				class="mt-4 rounded-lg border border-[#E11D48]/30 bg-[#E11D48]/5 px-4 py-1.5 font-mono text-[10px] text-[#E11D48] uppercase transition-colors hover:border-[#E11D48] hover:bg-[#E11D48]/10"
			>
				RETRY_QUERY()
			</button>
		</div>
	{:else}
		<!-- Stats Grid -->
		<div class="mb-12 grid grid-cols-1 gap-6 sm:grid-cols-3 lg:gap-8">
			<!-- Drafts Card -->
			<a
				href="/admin/drafts"
				class="group glow-hover relative flex flex-col rounded-2xl border border-[rgba(255,255,255,0.08)] bg-[#0F172A]/20 p-6"
			>
				<div
					class="absolute top-0 left-0 h-[1px] w-full bg-gradient-to-r from-transparent via-[#22D3EE]/30 to-transparent opacity-0 transition-opacity group-hover:opacity-100"
				></div>
				<span class="mb-3 block font-mono text-[9px] tracking-widest text-[#22D3EE]/80 uppercase"
					>// DRAFT_BUFFER</span
				>
				<div class="flex items-baseline space-x-2">
					<span class="font-serif text-4xl font-medium tracking-tight text-white sm:text-5xl"
						>{draftsCount}</span
					>
					<span class="font-mono text-[10px] text-slate-500">ITEMS</span>
				</div>
				<div
					class="mt-4 flex items-center justify-between border-t border-[rgba(255,255,255,0.04)] pt-4 font-mono text-[10px] text-slate-500 transition-colors group-hover:text-slate-300"
				>
					<span>VIEW BUFFER INDEX</span>
					<span>→</span>
				</div>
			</a>

			<!-- Published Card -->
			<a
				href="/admin/published"
				class="group glow-hover relative flex flex-col rounded-2xl border border-[rgba(255,255,255,0.08)] bg-[#0F172A]/20 p-6"
			>
				<div
					class="absolute top-0 left-0 h-[1px] w-full bg-gradient-to-r from-transparent via-[#22D3EE]/30 to-transparent opacity-0 transition-opacity group-hover:opacity-100"
				></div>
				<span class="mb-3 block font-mono text-[9px] tracking-widest text-[#22D3EE]/80 uppercase"
					>// PUBLISHED_INDEX</span
				>
				<div class="flex items-baseline space-x-2">
					<span class="font-serif text-4xl font-medium tracking-tight text-white sm:text-5xl"
						>{publishedCount}</span
					>
					<span class="font-mono text-[10px] text-slate-500">POSTS</span>
				</div>
				<div
					class="mt-4 flex items-center justify-between border-t border-[rgba(255,255,255,0.04)] pt-4 font-mono text-[10px] text-slate-500 transition-colors group-hover:text-slate-300"
				>
					<span>VIEW PUBLIC INDEX</span>
					<span>→</span>
				</div>
			</a>

			<!-- Rejected Card -->
			<a
				href="/admin/rejected"
				class="group glow-hover relative flex flex-col rounded-2xl border border-[rgba(255,255,255,0.08)] bg-[#0F172A]/20 p-6"
			>
				<div
					class="absolute top-0 left-0 h-[1px] w-full bg-gradient-to-r from-transparent via-[#22D3EE]/30 to-transparent opacity-0 transition-opacity group-hover:opacity-100"
				></div>
				<span class="mb-3 block font-mono text-[9px] tracking-widest text-[#22D3EE]/80 uppercase"
					>// REJECTED_LOG</span
				>
				<div class="flex items-baseline space-x-2">
					<span class="font-serif text-4xl font-medium tracking-tight text-white sm:text-5xl"
						>{rejectedCount}</span
					>
					<span class="font-mono text-[10px] text-slate-500">BLOCKED</span>
				</div>
				<div
					class="mt-4 flex items-center justify-between border-t border-[rgba(255,255,255,0.04)] pt-4 font-mono text-[10px] text-slate-500 transition-colors group-hover:text-slate-300"
				>
					<span>VIEW REJECT INDEX</span>
					<span>→</span>
				</div>
			</a>
		</div>

		<!-- Quick Links Panel -->
		<div
			class="rounded-2xl border border-[rgba(255,255,255,0.08)] bg-[#0F172A]/10 p-6 font-mono text-xs"
		>
			<span class="mb-4 block tracking-wider text-slate-500 uppercase">// CORE_QUICK_LINKS</span>
			<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 md:grid-cols-4">
				<a
					href="/admin/drafts"
					class="flex items-center justify-between rounded-lg border border-[rgba(255,255,255,0.06)] p-3 transition-colors hover:border-[#22D3EE]/30 hover:bg-[#22D3EE]/5"
				>
					<span>1. DRAFTS</span>
					<span class="font-bold text-[#22D3EE]">»</span>
				</a>
				<a
					href="/admin/published"
					class="flex items-center justify-between rounded-lg border border-[rgba(255,255,255,0.06)] p-3 transition-colors hover:border-[#22D3EE]/30 hover:bg-[#22D3EE]/5"
				>
					<span>2. PUBLISHED</span>
					<span class="font-bold text-[#22D3EE]">»</span>
				</a>
				<a
					href="/admin/rejected"
					class="flex items-center justify-between rounded-lg border border-[rgba(255,255,255,0.06)] p-3 transition-colors hover:border-[#22D3EE]/30 hover:bg-[#22D3EE]/5"
				>
					<span>3. REJECTED</span>
					<span class="font-bold text-[#22D3EE]">»</span>
				</a>
				<a
					href="/"
					class="flex items-center justify-between rounded-lg border border-[rgba(255,255,255,0.06)] p-3 transition-colors hover:border-slate-500 hover:bg-slate-800/10"
				>
					<span>4. PUBLIC HOME</span>
					<span class="text-slate-400">»</span>
				</a>
			</div>
		</div>
	{/if}
</section>
