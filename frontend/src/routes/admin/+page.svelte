<script lang="ts">
	import { onMount } from 'svelte';
	import FetchProgress from '$lib/FetchProgress.svelte';

	let draftsCount = $state(0);
	let publishedCount = $state(0);
	let rejectedCount = $state(0);

	let isLoading = $state(true);
	let errorMessage = $state('');
	let serverTime = $state('');

	let isScraping = $state(false);
	let scrapeResult = $state<string | null>(null);
	let scrapeError = $state<string | null>(null);
	let lastFetchInfo = $state<{ time: string; result: string } | null>(null);

	// Toast notification shown when a fetch cycle finishes.
	let toast = $state<{ type: 'success' | 'error'; message: string } | null>(null);
	let toastTimer: ReturnType<typeof setTimeout> | undefined;

	// Scoring thresholds (admin-configurable, backend persisted).
	let thresholds = $state<{
		low_max: number;
		medium_min: number;
		medium_max: number;
		high_min: number;
	} | null>(null);
	let settingsSaving = $state(false);
	let settingsSaved = $state(false);

	async function loadSettings() {
		const token = localStorage.getItem('admin_token');
		if (!token) return;
		try {
			const res = await fetch('http://localhost:8080/api/admin/settings', {
				headers: { Authorization: `Bearer ${token}` }
			});
			if (res.ok) thresholds = await res.json();
		} catch (e) {
			console.warn('load settings failed', e);
		}
	}

	async function saveSettings() {
		const token = localStorage.getItem('admin_token');
		if (!token || !thresholds) return;
		settingsSaving = true;
		settingsSaved = false;
		try {
			const res = await fetch('http://localhost:8080/api/admin/settings', {
				method: 'PUT',
				headers: {
					Authorization: `Bearer ${token}`,
					'Content-Type': 'application/json'
				},
				body: JSON.stringify(thresholds)
			});
			if (res.ok) {
				thresholds = await res.json();
				settingsSaved = true;
				setTimeout(() => (settingsSaved = false), 3000);
			} else {
				showToast('error', 'Failed to save scoring thresholds.');
			}
		} catch (e) {
			console.error('save settings failed', e);
			showToast('error', 'Failed to save scoring thresholds.');
		} finally {
			settingsSaving = false;
		}
	}

	function showToast(type: 'success' | 'error', message: string) {
		toast = { type, message };
		if (toastTimer) clearTimeout(toastTimer);
		toastTimer = setTimeout(() => {
			toast = null;
		}, 6000);
	}

	function loadLastFetch() {
		const stored = localStorage.getItem('last_fetch_info');
		if (stored) {
			try {
				lastFetchInfo = JSON.parse(stored);
			} catch (e) {
				console.error(e);
			}
		}
	}

	async function handleScrape() {
		const token = localStorage.getItem('admin_token');
		if (!token) return;

		isScraping = true;
		scrapeResult = null;
		scrapeError = null;

		try {
			const res = await fetch('http://localhost:8080/api/admin/fetch', {
				method: 'POST',
				headers: {
					Authorization: `Bearer ${token}`
				}
			});

			if (res.ok) {
				const data = await res.json();
				const resultMessage = `Scraped ${data.total_new ?? 0} new, skipped ${data.skipped_low_quality ?? 0} low quality`;
				scrapeResult = resultMessage;
				showToast('success', `Fetch cycle finished. ${resultMessage}`);

				const nowStr = new Date().toLocaleString('en-US', {
					month: 'short',
					day: 'numeric',
					hour: '2-digit',
					minute: '2-digit',
					second: '2-digit'
				});
				lastFetchInfo = { time: nowStr, result: resultMessage };
				localStorage.setItem('last_fetch_info', JSON.stringify(lastFetchInfo));

				await fetchStats();
			} else if (res.status === 409) {
				// Fetch was cancelled by the admin via CANCEL_FETCH.
				scrapeResult = null;
				showToast('error', 'Fetch cycle cancelled.');
			} else {
				if (res.status === 401 || res.status === 403) {
					localStorage.removeItem('admin_token');
					window.location.reload();
					return;
				}
				const data = await res.json().catch(() => ({}));
				scrapeError = data.error || 'Scrape operation failed on the backend.';
				showToast('error', `Fetch cycle failed. ${data.error || 'Unknown backend error.'}`);
			}
		} catch (err) {
			console.error('Scrape request failed:', err);
			scrapeError = 'Failed to communicate with the scraping node.';
			showToast('error', 'Fetch cycle failed. Failed to communicate with the backend.');
		} finally {
			isScraping = false;
		}
	}

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
		loadLastFetch();
		loadSettings();
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

<!-- Toast notification for finished fetch cycles -->
{#if toast}
	<div
		class="animate-slide-in fixed top-4 right-4 z-50 flex max-w-sm items-start gap-3 rounded-xl border p-4 font-mono text-xs shadow-2xl"
		style="border-color: {toast.type === 'success' ? '#22D3EE' : '#E11D48'}; background: #0F172A;"
	>
		<div
			class="mt-0.5 h-2 w-2 flex-shrink-0 rounded-full"
			style="background: {toast.type === 'success'
				? '#22D3EE'
				: '#E11D48'}; box-shadow: 0 0 8px {toast.type === 'success' ? '#22D3EE' : '#E11D48'};"
		></div>
		<div class="flex-grow space-y-1">
			<div
				class="font-bold tracking-widest uppercase"
				style="color: {toast.type === 'success' ? '#22D3EE' : '#E11D48'};"
			>
				{toast.type === 'success' ? 'Fetch Complete' : 'Fetch Failed'}
			</div>
			<div class="leading-relaxed text-slate-300">{toast.message}</div>
		</div>
		<button
			onclick={() => (toast = null)}
			class="cursor-pointer text-slate-500 transition-colors hover:text-white"
		>
			[X]
		</button>
	</div>
{/if}

<section
	class="mx-auto flex max-w-7xl flex-grow flex-col justify-center px-4 py-12 sm:px-6 md:py-16 lg:px-8"
>
	<!-- Header telemetry -->
	<div
		class="mb-12 flex flex-col items-start justify-between gap-4 border-b border-[rgba(255,255,255,0.08)] pb-6 sm:flex-row sm:items-end"
	>
		<div>
			<span class="tag-mono mb-1 block text-xs font-bold tracking-widest text-[#22D3EE]"
				>System Overview</span
			>
			<h1 class="font-serif text-3xl font-medium text-white">TERMINAL CONTROL</h1>
		</div>
		<div class="flex flex-col items-end gap-2 text-right">
			<div class="space-y-0.5 font-mono text-[10px] text-slate-500">
				<div>System Time: <span class="text-slate-300">{serverTime}</span></div>
				<div>API Status: <span class="font-bold text-[#22D3EE]">Online</span></div>
				{#if lastFetchInfo}
					<div>
						Last Fetch: <span class="text-[#22D3EE]">{lastFetchInfo.time}</span>
						<span class="text-slate-400">({lastFetchInfo.result})</span>
					</div>
				{/if}
			</div>
			<div>
				<button
					onclick={handleScrape}
					disabled={isScraping}
					class="cursor-pointer rounded border border-[#22D3EE]/30 bg-[#22D3EE]/5 px-3 py-1.5 font-mono text-xs text-[#22D3EE] transition-all hover:border-[#22D3EE] hover:bg-[#22D3EE]/10 hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
				>
					{#if isScraping}
						Running Fetch...
					{:else}
						Run Fetch
					{/if}
				</button>
			</div>
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
				Retry Query
			</button>
		</div>
	{:else}
		<!-- Live fetch progress (percentage bar) while the cycle runs -->
		<FetchProgress active={isScraping} />

		{#if scrapeResult}
			<div
				class="relative mb-6 rounded-xl border border-[#22D3EE]/30 bg-[#22D3EE]/5 p-4 text-center font-mono text-xs"
			>
				<button
					onclick={() => (scrapeResult = null)}
					class="absolute top-2 right-3 cursor-pointer text-slate-500 hover:text-[#22D3EE]"
				>
					[X]
				</button>
				<div class="mb-1 font-bold text-[#22D3EE] uppercase">Scrape Cycle Success</div>
				<div class="text-slate-300">{scrapeResult}</div>
			</div>
		{/if}

		{#if scrapeError}
			<div
				class="relative mb-6 rounded-xl border border-[#E11D48]/30 bg-[#E11D48]/5 p-4 text-center font-mono text-xs"
			>
				<button
					onclick={() => (scrapeError = null)}
					class="absolute top-2 right-3 cursor-pointer text-slate-500 hover:text-[#E11D48]"
				>
					[X]
				</button>
				<div class="mb-1 font-bold text-[#E11D48] uppercase">Scrape Cycle Failure</div>
				<div class="text-slate-400">{scrapeError}</div>
			</div>
		{/if}

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
					>Draft Buffer</span
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
					>Published Index</span
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
					>Rejected Log</span
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
			<span class="mb-4 block tracking-wider text-slate-500 uppercase">Core Quick Links</span>
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

		<!-- Scoring Thresholds Settings -->
		{#if thresholds}
			<div
				class="rounded-2xl border border-[rgba(255,255,255,0.08)] bg-[#0F172A]/10 p-6 font-mono text-xs"
			>
				<div class="mb-4 flex flex-wrap items-center justify-between gap-2">
					<span class="tracking-wider text-slate-500 uppercase"> Value Score Thresholds </span>
					{#if settingsSaved}
						<span class="text-[#22D3EE]">SAVED ✓</span>
					{/if}
				</div>
				<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
					<label class="block">
						<span class="mb-1 block text-[10px] text-slate-500 uppercase">LOW MAX (&lt;)</span>
						<input
							type="number"
							bind:value={thresholds.low_max}
							min="0"
							max="100"
							class="w-full rounded-lg border border-[rgba(255,255,255,0.08)] bg-[#070A10] px-3 py-2 text-xs text-white focus:border-[#22D3EE]/50 focus:outline-none"
						/>
					</label>
					<label class="block">
						<span class="mb-1 block text-[10px] text-slate-500 uppercase">MEDIUM MIN</span>
						<input
							type="number"
							bind:value={thresholds.medium_min}
							min="0"
							max="100"
							class="w-full rounded-lg border border-[rgba(255,255,255,0.08)] bg-[#070A10] px-3 py-2 text-xs text-white focus:border-[#22D3EE]/50 focus:outline-none"
						/>
					</label>
					<label class="block">
						<span class="mb-1 block text-[10px] text-slate-500 uppercase">MEDIUM MAX</span>
						<input
							type="number"
							bind:value={thresholds.medium_max}
							min="0"
							max="100"
							class="w-full rounded-lg border border-[rgba(255,255,255,0.08)] bg-[#070A10] px-3 py-2 text-xs text-white focus:border-[#22D3EE]/50 focus:outline-none"
						/>
					</label>
					<label class="block">
						<span class="mb-1 block text-[10px] text-slate-500 uppercase">HIGH MIN (≥)</span>
						<input
							type="number"
							bind:value={thresholds.high_min}
							min="0"
							max="100"
							class="w-full rounded-lg border border-[rgba(255,255,255,0.08)] bg-[#070A10] px-3 py-2 text-xs text-white focus:border-[#22D3EE]/50 focus:outline-none"
						/>
					</label>
				</div>
				<div class="mt-4 flex items-center justify-between gap-3">
					<span class="text-[10px] text-slate-600">
						Advisory only — scoring never auto-publishes. Admin approval is always required.
					</span>
					<button
						onclick={saveSettings}
						disabled={settingsSaving}
						class="cursor-pointer rounded-lg border border-[#22D3EE]/30 bg-[#22D3EE]/5 px-4 py-1.5 font-mono text-xs text-[#22D3EE] transition-all hover:border-[#22D3EE] hover:bg-[#22D3EE]/10 hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
					>
						{settingsSaving ? '[SAVING...]' : '[SAVE_THRESHOLDS]'}
					</button>
				</div>
			</div>
		{/if}
	{/if}
</section>

<style>
	@keyframes slide-in {
		from {
			opacity: 0;
			transform: translateX(20px);
		}
		to {
			opacity: 1;
			transform: translateX(0);
		}
	}
	.animate-slide-in {
		animation: slide-in 0.3s ease-out;
	}
</style>
