<script lang="ts">
	import { onMount } from 'svelte';
	import FetchProgress from '$lib/FetchProgress.svelte';
	import { BASE_URL, getCategories } from '$lib/api';
	import type { Category } from '$lib/mockData';

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
	let toast = $state<{ type: 'success' | 'error'; title: string; message: string } | null>(null);
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
			const res = await fetch(`${BASE_URL}/admin/settings`, {
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
			const res = await fetch(`${BASE_URL}/admin/settings`, {
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

	function showToast(type: 'success' | 'error', message: string, title?: string) {
		toast = {
			type,
			message,
			title: title ?? (type === 'success' ? 'Fetch Complete' : 'Fetch Failed')
		};
		if (toastTimer) clearTimeout(toastTimer);
		toastTimer = setTimeout(() => {
			toast = null;
		}, 6000);
	}

	// Auto Fetch & Auto Post scheduler config (STY-57). Category values are
	// slugs, matching what the scheduler compares against news.category.
	const INTERVAL_OPTIONS = [
		{ value: 5, label: '5 minutes' },
		{ value: 30, label: '30 minutes' },
		{ value: 60, label: '1 hour' },
		{ value: 360, label: '6 hours' },
		{ value: 720, label: '12 hours' },
		{ value: 1440, label: '24 hours' }
	];
	const POST_INTERVAL_OPTIONS = [{ value: 0, label: 'Same as fetch' }, ...INTERVAL_OPTIONS];
	const SCORE_LABEL_OPTIONS = [
		{ value: 'low', label: 'Low' },
		{ value: 'medium', label: 'Medium' },
		{ value: 'high', label: 'High' }
	];
	let categoriesList = $state<Category[]>([]);
	let autoConfig = $state({
		enabled: false,
		auto_post_enabled: false,
		interval_minutes: 360,
		categories: [] as string[],
		min_score_label: '' as 'low' | 'medium' | 'high' | '',
		min_score_labels: [] as string[],
		max_posts_per_cycle: 0,
		post_interval_minutes: 0
	});
	let autoConfigLoading = $state(false);
	let autoConfigSaving = $state(false);
	let autoConfigSaved = $state(false);
	let autoConfigError = $state('');
	let schedulerRunning = $state(false);
	let schedulerBusy = $state(false);

	function toggleCategory(slug: string) {
		if (autoConfig.categories.includes(slug)) {
			autoConfig.categories = autoConfig.categories.filter((s) => s !== slug);
		} else {
			autoConfig.categories = [...autoConfig.categories, slug];
		}
	}

	function selectAllCategories() {
		autoConfig.categories = categoriesList.map((c) => c.slug);
	}

	function clearCategories() {
		autoConfig.categories = [];
	}

	function toggleScoreLabel(label: string) {
		if (autoConfig.min_score_labels.includes(label)) {
			autoConfig.min_score_labels = autoConfig.min_score_labels.filter((l) => l !== label);
		} else {
			autoConfig.min_score_labels = [...autoConfig.min_score_labels, label];
		}
	}

	// Legacy mirror of the multi-select score filter: the least restrictive
	// (lowest) checked tier, so old backends keep working. Empty when all
	// scores are allowed.
	function legacyScoreLabel(): string {
		return (
			['low', 'medium', 'high'].filter((l) => autoConfig.min_score_labels.includes(l))[0] ?? ''
		);
	}

	// Applies the scheduler config fields from a backend response. Supports
	// the STY-60 multi-score filter with a legacy min_score_label fallback.
	function applyAutoConfig(data: Record<string, unknown>) {
		autoConfig.enabled = !!data.enabled;
		autoConfig.auto_post_enabled = !!data.auto_post_enabled;
		autoConfig.interval_minutes =
			Number(data.interval_minutes) > 0 ? Number(data.interval_minutes) : 360;
		autoConfig.categories = Array.isArray(data.categories) ? data.categories : [];
		const legacy = ['low', 'medium', 'high'].includes(data.min_score_label as string)
			? (data.min_score_label as string)
			: '';
		const labels = Array.isArray(data.min_score_labels)
			? (data.min_score_labels as string[]).filter((l) => ['low', 'medium', 'high'].includes(l))
			: [];
		autoConfig.min_score_label = legacy as 'low' | 'medium' | 'high' | '';
		autoConfig.min_score_labels = labels.length > 0 ? labels : legacy ? [legacy] : [];
		autoConfig.max_posts_per_cycle =
			Number.isFinite(Number(data.max_posts_per_cycle)) && Number(data.max_posts_per_cycle) > 0
				? Number(data.max_posts_per_cycle)
				: 0;
		autoConfig.post_interval_minutes =
			Number.isFinite(Number(data.post_interval_minutes)) && Number(data.post_interval_minutes) > 0
				? Number(data.post_interval_minutes)
				: 0;
	}

	async function loadAutoConfig() {
		const token = localStorage.getItem('admin_token');
		if (!token) return;
		autoConfigLoading = true;
		autoConfigError = '';
		try {
			const [res, cats] = await Promise.all([
				fetch(`${BASE_URL}/admin/autopublish`, {
					headers: { Authorization: `Bearer ${token}` }
				}),
				getCategories()
			]);
			categoriesList = cats;
			if (res.ok) {
				const data = await res.json();
				applyAutoConfig(data);
				schedulerRunning = !!data.running;
			} else {
				if (res.status === 401 || res.status === 403) {
					localStorage.removeItem('admin_token');
					window.location.reload();
					return;
				}
				autoConfigError = 'Failed to load auto publish config from the backend.';
			}
		} catch (e) {
			console.warn('load autopublish config failed', e);
			autoConfigError = 'Failed to load auto publish config from the backend.';
		} finally {
			autoConfigLoading = false;
		}
	}

	async function saveAutoConfig() {
		const token = localStorage.getItem('admin_token');
		if (!token) return;
		autoConfigSaving = true;
		autoConfigSaved = false;
		try {
			const res = await fetch(`${BASE_URL}/admin/autopublish`, {
				method: 'PUT',
				headers: {
					Authorization: `Bearer ${token}`,
					'Content-Type': 'application/json'
				},
				body: JSON.stringify({
					enabled: autoConfig.enabled,
					auto_post_enabled: autoConfig.auto_post_enabled,
					interval_minutes: Number(autoConfig.interval_minutes) || 360,
					categories: autoConfig.categories,
					min_score_label: legacyScoreLabel(),
					min_score_labels: autoConfig.min_score_labels,
					max_posts_per_cycle: Math.max(0, Math.floor(Number(autoConfig.max_posts_per_cycle) || 0)),
					post_interval_minutes: Math.max(0, Number(autoConfig.post_interval_minutes) || 0)
				})
			});
			if (res.ok) {
				const data = await res.json();
				if (data) applyAutoConfig(data);
				autoConfigSaved = true;
				showToast('success', 'Auto fetch / auto post config saved.', 'Config Saved');
				setTimeout(() => (autoConfigSaved = false), 3000);
			} else {
				if (res.status === 401 || res.status === 403) {
					localStorage.removeItem('admin_token');
					window.location.reload();
					return;
				}
				showToast('error', 'Failed to save auto publish config.', 'Save Failed');
			}
		} catch (e) {
			console.error('save autopublish config failed', e);
			showToast('error', 'Failed to save auto publish config.', 'Save Failed');
		} finally {
			autoConfigSaving = false;
		}
	}

	async function setSchedulerRunning(running: boolean) {
		const token = localStorage.getItem('admin_token');
		if (!token) return;
		schedulerBusy = true;
		try {
			const res = await fetch(`${BASE_URL}/admin/autopublish/${running ? 'start' : 'stop'}`, {
				method: 'POST',
				headers: { Authorization: `Bearer ${token}` }
			});
			if (res.ok) {
				const data = await res.json();
				schedulerRunning = !!data.running;
				showToast(
					'success',
					running ? 'Scheduler started.' : 'Scheduler stopped.',
					running ? 'Scheduler Started' : 'Scheduler Stopped'
				);
			} else {
				if (res.status === 401 || res.status === 403) {
					localStorage.removeItem('admin_token');
					window.location.reload();
					return;
				}
				const data = await res.json().catch(() => ({}));
				showToast(
					'error',
					data.error || (running ? 'Failed to start scheduler.' : 'Failed to stop scheduler.'),
					running ? 'Start Failed' : 'Stop Failed'
				);
			}
		} catch (e) {
			console.error('scheduler start/stop request failed', e);
			showToast(
				'error',
				running ? 'Failed to start scheduler.' : 'Failed to stop scheduler.',
				running ? 'Start Failed' : 'Stop Failed'
			);
		} finally {
			schedulerBusy = false;
		}
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
			const res = await fetch(`${BASE_URL}/admin/fetch`, {
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
				fetch(`${BASE_URL}/admin/news?status=draft&page_size=1`, { headers }),
				fetch(`${BASE_URL}/admin/news?status=published&page_size=1`, { headers }),
				fetch(`${BASE_URL}/admin/news?status=rejected&page_size=1`, { headers })
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
		loadAutoConfig();
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
				{toast.title}
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

		<!-- Auto Fetch & Auto Post Scheduler -->
		<div
			class="rounded-2xl border border-[rgba(255,255,255,0.08)] bg-[#0F172A]/10 p-6 font-mono text-xs"
		>
			<div class="mb-4 flex flex-wrap items-center justify-between gap-2">
				<span class="tracking-wider text-slate-500 uppercase">
					Auto Fetch & Auto Post Pipeline
				</span>
				<div class="flex items-center gap-3">
					{#if !autoConfigLoading}
						<span
							class="flex items-center gap-2 font-mono text-[10px] {schedulerRunning
								? 'text-[#22D3EE]'
								: 'text-slate-500'}"
						>
							<span
								class="h-2 w-2 rounded-full {schedulerRunning
									? 'animate-pulse bg-[#22D3EE]'
									: 'bg-slate-600'}"
								style="box-shadow: {schedulerRunning ? '0 0 8px #22D3EE' : 'none'};"
							></span>
							{schedulerRunning ? 'RUNNING' : 'STOPPED'}
						</span>
					{/if}
					{#if autoConfigSaved}
						<span class="text-[#22D3EE]">SAVED ✓</span>
					{/if}
				</div>
			</div>

			{#if autoConfigLoading}
				<div class="animate-pulse tracking-widest text-slate-500 uppercase">
					Syncing Scheduler Config...
				</div>
			{:else}
				{#if autoConfigError}
					<div class="mb-4 text-[10px] text-[#E11D48]">{autoConfigError}</div>
				{/if}

				<div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
					<!-- Toggles + Interval -->
					<div class="space-y-5">
						<!-- Auto Fetch toggle -->
						<div class="flex items-center justify-between gap-4">
							<div>
								<div class="text-white">Auto Fetch</div>
								<div class="mt-0.5 text-[10px] text-slate-500">
									Run the scheduled RSS fetch cycle. When off, no fetch runs automatically.
								</div>
							</div>
							<button
								type="button"
								role="switch"
								aria-checked={autoConfig.enabled}
								aria-label="Toggle auto fetch"
								onclick={() => (autoConfig.enabled = !autoConfig.enabled)}
								class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer items-center rounded-full border transition-colors {autoConfig.enabled
									? 'border-[#22D3EE]/50 bg-[#22D3EE]/20'
									: 'border-[rgba(255,255,255,0.15)] bg-[#070A10]'}"
							>
								<span
									class="inline-block h-4 w-4 transform rounded-full transition-transform duration-200 {autoConfig.enabled
										? 'translate-x-6 bg-[#22D3EE]'
										: 'translate-x-1 bg-slate-500'}"
								></span>
							</button>
						</div>

						<!-- Auto Post toggle -->
						<div class="flex items-center justify-between gap-4">
							<div>
								<div class="text-white">Auto Post</div>
								<div class="mt-0.5 text-[10px] text-slate-500">
									Auto-publish qualifying drafts (score + category filters). When off, the scheduler
									only fetches into the draft buffer.
								</div>
							</div>
							<button
								type="button"
								role="switch"
								aria-checked={autoConfig.auto_post_enabled}
								aria-label="Toggle auto post"
								onclick={() => (autoConfig.auto_post_enabled = !autoConfig.auto_post_enabled)}
								class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer items-center rounded-full border transition-colors {autoConfig.auto_post_enabled
									? 'border-[#22D3EE]/50 bg-[#22D3EE]/20'
									: 'border-[rgba(255,255,255,0.15)] bg-[#070A10]'}"
							>
								<span
									class="inline-block h-4 w-4 transform rounded-full transition-transform duration-200 {autoConfig.auto_post_enabled
										? 'translate-x-6 bg-[#22D3EE]'
										: 'translate-x-1 bg-slate-500'}"
								></span>
							</button>
						</div>

						<!-- Fetch interval -->
						<label class="block">
							<span class="mb-1 block text-[10px] text-slate-500 uppercase">Fetch Interval</span>
							<select
								bind:value={autoConfig.interval_minutes}
								class="w-full cursor-pointer rounded-lg border border-[rgba(255,255,255,0.08)] bg-[#070A10] px-3 py-2 text-xs text-white focus:border-[#22D3EE]/50 focus:outline-none"
							>
								{#each INTERVAL_OPTIONS as opt}
									<option value={opt.value}>{opt.label}</option>
								{/each}
								{#if !INTERVAL_OPTIONS.some((o) => o.value === autoConfig.interval_minutes)}
									<option value={autoConfig.interval_minutes}>
										{autoConfig.interval_minutes} minutes
									</option>
								{/if}
							</select>
						</label>

						<!-- Post interval (independent of fetch) -->
						<label class="block">
							<span class="mb-1 block text-[10px] text-slate-500 uppercase">Post Interval</span>
							<select
								bind:value={autoConfig.post_interval_minutes}
								class="w-full cursor-pointer rounded-lg border border-[rgba(255,255,255,0.08)] bg-[#070A10] px-3 py-2 text-xs text-white focus:border-[#22D3EE]/50 focus:outline-none"
							>
								{#each POST_INTERVAL_OPTIONS as opt}
									<option value={opt.value}>{opt.label}</option>
								{/each}
								{#if !POST_INTERVAL_OPTIONS.some((o) => o.value === autoConfig.post_interval_minutes)}
									<option value={autoConfig.post_interval_minutes}>
										{autoConfig.post_interval_minutes} minutes
									</option>
								{/if}
							</select>
						</label>
					</div>

					<!-- Filters -->
					<div class="space-y-5">
						<!-- Score filter (multi-select) -->
						<div>
							<div class="mb-2 flex items-center justify-between gap-2">
								<span class="text-[10px] text-slate-500 uppercase">Score Filter</span>
								{#if autoConfig.min_score_labels.length > 0}
									<button
										onclick={() => (autoConfig.min_score_labels = [])}
										class="cursor-pointer text-[10px] text-slate-500 transition-colors hover:text-[#22D3EE]"
									>
										CLEAR
									</button>
								{:else}
									<span class="text-[10px] text-[#22D3EE]/80">ALL SCORES (NO FILTER)</span>
								{/if}
							</div>
							<div class="flex flex-wrap gap-1">
								{#each SCORE_LABEL_OPTIONS as opt (opt.value)}
									<label
										class="flex cursor-pointer items-center gap-2 rounded-lg border border-[rgba(255,255,255,0.06)] px-3 py-2 transition-colors hover:border-[#22D3EE]/30 hover:bg-[#22D3EE]/5 {autoConfig.min_score_labels.includes(
											opt.value
										)
											? 'border-[#22D3EE]/40 bg-[#22D3EE]/10'
											: ''}"
									>
										<input
											type="checkbox"
											checked={autoConfig.min_score_labels.includes(opt.value)}
											onchange={() => toggleScoreLabel(opt.value)}
											class="h-3.5 w-3.5 cursor-pointer accent-[#22D3EE]"
										/>
										<span
											class="text-[11px] {autoConfig.min_score_labels.includes(opt.value)
												? 'text-[#22D3EE]'
												: 'text-slate-300'}"
										>
											{opt.label}
										</span>
									</label>
								{/each}
							</div>
						</div>

						<!-- Max posts per cycle -->
						<label class="block">
							<span class="mb-1 block text-[10px] text-slate-500 uppercase">
								Max posts per cycle (0 = unlimited)
							</span>
							<input
								type="number"
								bind:value={autoConfig.max_posts_per_cycle}
								min="0"
								step="1"
								placeholder="0 = unlimited"
								class="w-full rounded-lg border border-[rgba(255,255,255,0.08)] bg-[#070A10] px-3 py-2 text-xs text-white placeholder:text-slate-600 focus:border-[#22D3EE]/50 focus:outline-none"
							/>
						</label>

						<!-- Category filter -->
						<div>
							<div class="mb-2 flex items-center justify-between gap-2">
								<span class="text-[10px] text-slate-500 uppercase">Category Filter</span>
								<div class="flex items-center gap-3">
									{#if autoConfig.categories.length > 0}
										<button
											onclick={clearCategories}
											class="cursor-pointer text-[10px] text-slate-500 transition-colors hover:text-[#22D3EE]"
										>
											CLEAR
										</button>
										<button
											onclick={selectAllCategories}
											class="cursor-pointer text-[10px] text-slate-500 transition-colors hover:text-[#22D3EE]"
										>
											ALL
										</button>
									{:else}
										<span class="text-[10px] text-[#22D3EE]/80">ALL CATEGORIES (NO FILTER)</span>
									{/if}
								</div>
							</div>
							{#if categoriesList.length === 0}
								<div class="text-[10px] text-slate-600">Loading categories...</div>
							{:else}
								<div class="grid max-h-48 grid-cols-1 gap-1 overflow-y-auto sm:grid-cols-2">
									{#each categoriesList as cat (cat.slug)}
										<label
											class="flex cursor-pointer items-center gap-2 rounded-lg border border-[rgba(255,255,255,0.06)] px-3 py-2 transition-colors hover:border-[#22D3EE]/30 hover:bg-[#22D3EE]/5 {autoConfig.categories.includes(
												cat.slug
											)
												? 'border-[#22D3EE]/40 bg-[#22D3EE]/10'
												: ''}"
										>
											<input
												type="checkbox"
												checked={autoConfig.categories.includes(cat.slug)}
												onchange={() => toggleCategory(cat.slug)}
												class="h-3.5 w-3.5 cursor-pointer accent-[#22D3EE]"
											/>
											<span
												class="text-[11px] {autoConfig.categories.includes(cat.slug)
													? 'text-[#22D3EE]'
													: 'text-slate-300'}"
											>
												{cat.name}
											</span>
										</label>
									{/each}
								</div>
							{/if}
						</div>
					</div>
				</div>

				<div
					class="mt-6 flex flex-col gap-3 border-t border-[rgba(255,255,255,0.04)] pt-4 sm:flex-row sm:items-center sm:justify-between"
				>
					<span class="text-[10px] text-slate-600">
						Saving only persists the config — press Start Config to activate the scheduler. Auto
						Post publishes drafts that pass the category + score filters.
					</span>
					<div class="flex flex-wrap items-center gap-3">
						<button
							onclick={() => setSchedulerRunning(true)}
							disabled={schedulerRunning || schedulerBusy}
							class="cursor-pointer rounded-lg border border-[#22D3EE]/30 bg-[#22D3EE]/5 px-4 py-1.5 font-mono text-xs text-[#22D3EE] transition-all hover:border-[#22D3EE] hover:bg-[#22D3EE]/10 hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
						>
							Start Config
						</button>
						<button
							onclick={() => setSchedulerRunning(false)}
							disabled={!schedulerRunning || schedulerBusy}
							class="cursor-pointer rounded-lg border border-[#E11D48]/30 bg-[#E11D48]/5 px-4 py-1.5 font-mono text-xs text-[#E11D48] transition-all hover:border-[#E11D48] hover:bg-[#E11D48]/10 hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
						>
							Stop Config
						</button>
						<button
							onclick={saveAutoConfig}
							disabled={autoConfigSaving}
							class="cursor-pointer rounded-lg border border-[#22D3EE]/30 bg-[#22D3EE]/5 px-4 py-1.5 font-mono text-xs text-[#22D3EE] transition-all hover:border-[#22D3EE] hover:bg-[#22D3EE]/10 hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
						>
							{autoConfigSaving ? 'Saving...' : 'Save Config'}
						</button>
					</div>
				</div>
			{/if}
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
						{settingsSaving ? 'Saving...' : 'Save Thresholds'}
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
