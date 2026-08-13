<script lang="ts">
	import { onMount } from 'svelte';
	import { page as pageStore } from '$app/stores';
	import { goto } from '$app/navigation';
	import Image from '$lib/Image.svelte';
	import FetchProgress from '$lib/FetchProgress.svelte';

	// Retrieve active page from URL query params
	let currentPage = $derived(Number($pageStore.url.searchParams.get('page')) || 1);
	let selectedCategory = $derived($pageStore.url.searchParams.get('category') || '');
	let selectedValueLabel = $derived($pageStore.url.searchParams.get('value_label') || '');
	const pageSize = 10;

	let articles = $state<any[]>([]);
	let totalItems = $state(0);
	let totalPages = $state(0);

	let isLoading = $state(true);
	let errorMessage = $state('');
	let confirmingDeleteId = $state<number | null>(null);

	let isScraping = $state(false);
	let isDeletingAll = $state(false);
	let scrapeResult = $state<string | null>(null);
	let scrapeError = $state<string | null>(null);
	let lastFetchInfo = $state<{ time: string; result: string } | null>(null);

	// Toast notification shown when a fetch cycle finishes.
	let toast = $state<{ type: 'success' | 'error'; message: string } | null>(null);
	let toastTimer: ReturnType<typeof setTimeout> | undefined;

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

				await fetchDrafts(currentPage);
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

	async function fetchDrafts(pageNumber: number) {
		isLoading = true;
		errorMessage = '';
		const token = localStorage.getItem('admin_token');
		if (!token) return;

		try {
			let url = `http://localhost:8080/api/admin/news?status=draft&page=${pageNumber}&page_size=${pageSize}`;
			if (selectedCategory) {
				url += `&category=${encodeURIComponent(selectedCategory)}`;
			}
			if (selectedValueLabel) {
				url += `&value_label=${encodeURIComponent(selectedValueLabel)}`;
			}
			const res = await fetch(url, {
				headers: {
					Authorization: `Bearer ${token}`,
					Accept: 'application/json'
				}
			});

			if (res.ok) {
				const result = await res.json();
				articles = result.data || [];
				totalItems = result.pagination.total || 0;
				totalPages = result.pagination.total_pages || 0;
			} else {
				if (res.status === 401 || res.status === 403) {
					localStorage.removeItem('admin_token');
					window.location.reload();
					return;
				}
				errorMessage = 'Failed to fetch draft buffers.';
			}
		} catch (err) {
			console.error('Fetch drafts error:', err);
			errorMessage = 'Database node link offline.';
		} finally {
			isLoading = false;
		}
	}

	onMount(() => {
		fetchDrafts(currentPage);
		loadLastFetch();
	});

	// Trigger fetch on query param page change
	$effect(() => {
		fetchDrafts(currentPage);
	});

	async function handlePublish(id: number) {
		const token = localStorage.getItem('admin_token');
		if (!token) return;

		try {
			const res = await fetch(`http://localhost:8080/api/admin/news/${id}/publish`, {
				method: 'POST',
				headers: {
					Authorization: `Bearer ${token}`
				}
			});

			if (res.ok) {
				// Remove published article from local list
				articles = articles.filter((a) => a.id !== id);
				totalItems -= 1;
				if (articles.length === 0 && currentPage > 1) {
					goto(`/admin/drafts?page=${currentPage - 1}`);
				} else {
					fetchDrafts(currentPage);
				}
			} else {
				alert('Failed to publish article.');
			}
		} catch (err) {
			console.error('Publish error:', err);
			alert('Network issue committing publish operation.');
		}
	}

	async function handleReject(id: number) {
		const token = localStorage.getItem('admin_token');
		if (!token) return;

		try {
			const res = await fetch(`http://localhost:8080/api/admin/news/${id}/reject`, {
				method: 'POST',
				headers: {
					Authorization: `Bearer ${token}`
				}
			});

			if (res.ok) {
				// Remove rejected article from local list
				articles = articles.filter((a) => a.id !== id);
				totalItems -= 1;
				if (articles.length === 0 && currentPage > 1) {
					goto(`/admin/drafts?page=${currentPage - 1}`);
				} else {
					fetchDrafts(currentPage);
				}
			} else {
				alert('Failed to reject article.');
			}
		} catch (err) {
			console.error('Reject error:', err);
			alert('Network issue committing reject operation.');
		}
	}

	async function handleDelete(id: number) {
		const token = localStorage.getItem('admin_token');
		if (!token) return;

		try {
			const res = await fetch(`http://localhost:8080/api/admin/news/${id}`, {
				method: 'DELETE',
				headers: {
					Authorization: `Bearer ${token}`
				}
			});

			if (res.ok) {
				confirmingDeleteId = null;
				articles = articles.filter((a) => a.id !== id);
				totalItems -= 1;
				if (articles.length === 0 && currentPage > 1) {
					goto(`/admin/drafts?page=${currentPage - 1}`);
				} else {
					fetchDrafts(currentPage);
				}
			} else {
				alert('Failed to delete article.');
			}
		} catch (err) {
			console.error('Delete error:', err);
			alert('Network error executing delete.');
		}
	}

	async function handleDeleteAll() {
		const token = localStorage.getItem('admin_token');
		if (!token) return;

		if (
			!confirm(
				'WARNING: Are you sure you want to delete ALL draft articles? This action cannot be undone.'
			)
		) {
			return;
		}

		isDeletingAll = true;
		try {
			const res = await fetch('http://localhost:8080/api/admin/news?status=draft', {
				method: 'DELETE',
				headers: {
					Authorization: `Bearer ${token}`
				}
			});

			if (res.ok) {
				articles = [];
				totalItems = 0;
				totalPages = 0;
				goto('/admin/drafts?page=1');
			} else {
				alert('Failed to delete all drafts.');
			}
		} catch (err) {
			console.error('Delete all error:', err);
			alert('Network error executing bulk delete.');
		} finally {
			isDeletingAll = false;
		}
	}

	function changePage(newPage: number) {
		if (newPage >= 1 && newPage <= totalPages) {
			let path = `/admin/drafts?page=${newPage}`;
			if (selectedCategory) {
				path += `&category=${encodeURIComponent(selectedCategory)}`;
			}
			if (selectedValueLabel) {
				path += `&value_label=${encodeURIComponent(selectedValueLabel)}`;
			}
			goto(path);
		}
	}

	function handleCategoryChange(e: Event) {
		const val = (e.target as HTMLSelectElement).value;
		let path = `/admin/drafts?page=1`;
		if (val) {
			path += `&category=${encodeURIComponent(val)}`;
		}
		if (selectedValueLabel) {
			path += `&value_label=${encodeURIComponent(selectedValueLabel)}`;
		}
		goto(path);
	}

	function handleValueLabelChange(e: Event) {
		const val = (e.target as HTMLSelectElement).value;
		let path = `/admin/drafts?page=1`;
		if (selectedCategory) {
			path += `&category=${encodeURIComponent(selectedCategory)}`;
		}
		if (val) {
			path += `&value_label=${encodeURIComponent(val)}`;
		}
		goto(path);
	}

	// Score display helpers -------------------------------------------------
	function scoreColor(label: string) {
		if (label === 'HIGH') return '#22D3EE';
		if (label === 'MEDIUM') return '#F59E0B';
		if (label === 'LOW') return '#64748B';
		return '#64748B';
	}

	function scoreClass(label: string) {
		if (label === 'HIGH') return 'border-[#22D3EE]/40 bg-[#22D3EE]/10 text-[#22D3EE]';
		if (label === 'MEDIUM') return 'border-amber-400/40 bg-amber-400/10 text-amber-400';
		return 'border-slate-500/40 bg-slate-500/10 text-slate-400';
	}

	function parseBreakdown(item: any): any {
		try {
			return item.value_breakdown ? JSON.parse(item.value_breakdown) : null;
		} catch {
			return null;
		}
	}

	function formatScore(item: any) {
		const s = item.value_score;
		if (s === null || s === undefined) return '—';
		return s;
	}

	function formatReason(item: any) {
		const r = item.value_reason;
		if (!r) return 'No scoring reason available.';
		return r;
	}

	function formatDate(dateStr: string) {
		return new Date(dateStr).toLocaleString('en-US', {
			year: 'numeric',
			month: 'short',
			day: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		});
	}
</script>

<svelte:head>
	<title>Draft Buffer Inventory | Neuralwire</title>
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
	class="mx-auto flex w-full max-w-7xl flex-grow flex-col justify-start px-4 py-12 sm:px-6 md:py-16 lg:px-8"
>
	<!-- Header -->
	<div
		class="mb-8 flex flex-col items-start justify-between gap-4 border-b border-[rgba(255,255,255,0.08)] pb-6 sm:flex-row sm:items-end"
	>
		<div>
			<span class="tag-mono mb-1 block text-xs font-bold tracking-widest text-[#22D3EE]"
				>Draft Inventory</span
			>
			<h1 class="font-serif text-3xl font-medium text-white">QUEUE BUFFER</h1>
		</div>
		<div class="flex flex-col items-end gap-2 text-right">
			<div class="space-y-0.5 font-mono text-[10px] text-slate-500">
				<div>
					Total Pending: <span class="font-bold text-slate-300">{totalItems}</span> Record(s)
				</div>
				{#if lastFetchInfo}
					<div>
						Last Fetch: <span class="text-[#22D3EE]">{lastFetchInfo.time}</span>
						<span class="text-slate-400">({lastFetchInfo.result})</span>
					</div>
				{/if}
			</div>
			<div class="flex flex-wrap items-center gap-3">
				<!-- Category Filter Selector -->
				<select
					value={selectedCategory}
					onchange={handleCategoryChange}
					class="cursor-pointer rounded border border-[rgba(255,255,255,0.12)] bg-[#070B13] px-3 py-1.5 font-mono text-xs text-slate-300 focus:border-[#22D3EE] focus:outline-none"
				>
					<option value="">All Categories</option>
					<option value="ai">AI</option>
					<option value="tools">Tools</option>
					<option value="research">Research</option>
					<option value="industry">Industry</option>
					<option value="machine-learning">Machine Learning</option>
				</select>

				<!-- Value score label filter -->
				<select
					value={selectedValueLabel}
					onchange={handleValueLabelChange}
					class="cursor-pointer rounded border border-[rgba(255,255,255,0.12)] bg-[#070B13] px-3 py-1.5 font-mono text-xs text-slate-300 focus:border-[#22D3EE] focus:outline-none"
				>
					<option value="">All Value Scores</option>
					<option value="HIGH">High ≥80</option>
					<option value="MEDIUM">Medium 60–79</option>
					<option value="LOW">Low &lt;60</option>
				</select>

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

				<button
					onclick={handleDeleteAll}
					disabled={isDeletingAll || articles.length === 0}
					class="cursor-pointer rounded border border-[#E11D48]/30 bg-[#E11D48]/5 px-3 py-1.5 font-mono text-xs text-[#E11D48] transition-all hover:border-[#E11D48] hover:bg-[#E11D48]/10 hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
				>
					{isDeletingAll ? 'Deleting All...' : 'Delete All Drafts'}
				</button>
			</div>
		</div>
	</div>

	<!-- Live fetch progress (percentage bar) while the cycle runs -->
	<FetchProgress active={isScraping} />

	<!-- Scrape results feedback -->
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

	<!-- Loading -->
	{#if isLoading}
		<div class="flex flex-grow items-center justify-center py-20">
			<div class="space-y-4 text-center font-mono text-xs">
				<div
					class="mx-auto h-6 w-6 animate-spin rounded-full border-2 border-slate-800 border-t-[#22D3EE]"
				></div>
				<div class="animate-pulse tracking-widest text-slate-500 uppercase">
					Syncing Buffer Stack...
				</div>
			</div>
		</div>
	{:else if errorMessage}
		<div
			class="mx-auto my-12 max-w-md rounded-xl border border-[#E11D48]/30 bg-[#E11D48]/5 p-8 text-center"
		>
			<p class="mb-2 font-mono text-xs tracking-wider text-[#E11D48] uppercase">IO FAILURE</p>
			<p class="font-sans text-xs text-slate-400">{errorMessage}</p>
		</div>
	{:else if articles.length === 0}
		<div
			class="my-12 rounded-xl border border-dashed border-[rgba(255,255,255,0.08)] bg-[#0F172A]/10 px-4 py-20 text-center"
		>
			<p class="mb-1 font-mono text-xs tracking-wider text-slate-500 uppercase">Queue Empty</p>
			<p class="font-sans text-xs text-slate-600">
				No drafts pending review in the incoming registry.
			</p>
		</div>
	{:else}
		<!-- List -->
		<div class="flex-grow space-y-6">
			{#each articles as item (item.id)}
				<div
					class="group glow-hover relative flex flex-col justify-between gap-6 rounded-xl border border-[rgba(255,255,255,0.08)] bg-[#0F172A]/20 p-6 md:flex-row"
				>
					<!-- Article Info -->
					<div class="flex flex-grow items-start gap-4">
						<!-- Thumbnail preview -->
						<div
							class="relative h-16 w-20 flex-shrink-0 overflow-hidden rounded-lg border border-[rgba(255,255,255,0.08)] bg-[#0A0E17] sm:h-20 sm:w-28"
						>
							<Image
								src={item.image_url}
								content={item.content}
								alt={item.title}
								class="h-full w-full object-cover opacity-75 grayscale transition-all duration-300 group-hover:scale-105 group-hover:opacity-100 group-hover:grayscale-0"
							/>
						</div>

						<div class="flex-grow space-y-3">
							<div
								class="flex flex-wrap items-center gap-x-3 gap-y-2 font-mono text-[10px] text-slate-500"
							>
								<span
									class="rounded border border-[#22D3EE]/20 bg-[#22D3EE]/5 px-1.5 py-0.5 font-bold text-[#22D3EE] uppercase"
									>{item.category}</span
								>
								<span>•</span>
								<span>{item.source.toUpperCase()}</span>
								<span>•</span>
								<span>{formatDate(item.created_at)}</span>
							</div>

							<!-- Value score badge + sub-scores -->
							{#if item.value_score !== null && item.value_score !== undefined}
								{@const bd = parseBreakdown(item)}
								<div class="flex flex-wrap items-center gap-2 font-mono text-[10px]">
									<span
										class="rounded border px-2 py-0.5 font-bold uppercase {scoreClass(
											item.value_label
										)}"
									>
										{item.value_label || 'UNSCORED'} • {formatScore(item)}
									</span>
									{#if bd?.ai && typeof bd.ai === 'object'}
										<span class="text-slate-500">
											AI:{bd.ai.score} (I:{bd.ai.impact} N:{bd.ai.novelty} Q:{bd.ai.quality})
										</span>
									{/if}
									{#if bd?.heuristic}
										<span class="text-slate-600">H:{bd.heuristic.score}</span>
									{/if}
									{#if item.value_confidence}
										<span class="text-slate-600">C:{item.value_confidence}</span>
									{/if}
									{#if item.value_method}
										<span class="text-slate-600">[{item.value_method}]</span>
									{/if}
								</div>
								{#if item.value_reason}
									<p
										class="max-w-4xl font-sans text-[11px] leading-relaxed font-light text-slate-500 italic"
									>
										{formatReason(item)}
									</p>
								{/if}
							{/if}

							<h3
								class="font-serif text-lg leading-snug font-normal text-white transition-colors group-hover:text-[#22D3EE]"
							>
								{item.title}
							</h3>
							<p class="max-w-4xl font-sans text-xs leading-relaxed font-light text-slate-400">
								{item.summary}
							</p>
						</div>
					</div>

					<!-- Actions Panel -->
					<div
						class="flex min-w-[140px] flex-row items-center justify-end gap-3 md:flex-col md:items-end"
					>
						{#if confirmingDeleteId === item.id}
							<!-- Inline delete confirm -->
							<div
								class="flex w-full flex-col gap-2 rounded-lg border border-[#E11D48]/30 bg-[#E11D48]/5 p-2 text-center"
							>
								<span class="font-mono text-[9px] font-bold tracking-wider text-[#E11D48] uppercase"
									>DELETE RECORD?</span
								>
								<div class="flex gap-2">
									<button
										onclick={() => handleDelete(item.id)}
										class="flex-1 cursor-pointer rounded bg-[#E11D48] px-2 py-1 font-mono text-[10px] text-white transition-colors hover:bg-[#E11D48]/90"
									>
										CONFIRM
									</button>
									<button
										onclick={() => (confirmingDeleteId = null)}
										class="flex-1 cursor-pointer rounded border border-slate-700 px-2 py-1 font-mono text-[10px] text-slate-400 transition-colors hover:bg-slate-800 hover:text-white"
									>
										CANCEL
									</button>
								</div>
							</div>
						{:else}
							<!-- Normal actions -->
							<div class="flex w-full flex-wrap gap-2 md:flex-col">
								<a
									href="/admin/preview/{item.id}"
									class="flex-1 rounded border border-[#22D3EE]/30 bg-[#22D3EE]/5 px-2.5 py-1.5 text-center font-mono text-[10px] text-[#22D3EE] transition-all hover:border-[#22D3EE] md:w-full"
								>
									Preview
								</a>
								<button
									onclick={() => handlePublish(item.id)}
									class="flex-1 cursor-pointer rounded border border-cyan-500 bg-cyan-950/20 px-2.5 py-1.5 font-mono text-[10px] text-[#22D3EE] transition-all hover:bg-cyan-500 hover:text-[#0A0E17] md:w-full"
								>
									Publish
								</button>
								<button
									onclick={() => handleReject(item.id)}
									class="flex-1 cursor-pointer rounded border border-amber-500/50 bg-amber-950/10 px-2.5 py-1.5 font-mono text-[10px] text-amber-500 transition-all hover:bg-amber-500 hover:text-[#0A0E17] md:w-full"
								>
									Reject
								</button>
								<button
									onclick={() => (confirmingDeleteId = item.id)}
									class="flex-1 cursor-pointer rounded border border-transparent px-2.5 py-1.5 font-mono text-[10px] text-slate-500 transition-all hover:border-[#E11D48]/50 hover:text-[#E11D48] md:w-full"
								>
									Delete
								</button>
							</div>
						{/if}
					</div>
				</div>
			{/each}
		</div>

		<!-- Pagination Footer -->
		{#if totalPages > 1}
			<div
				class="mt-12 flex items-center justify-between border-t border-[rgba(255,255,255,0.06)] pt-6 font-mono text-xs text-slate-500"
			>
				<button
					onclick={() => changePage(currentPage - 1)}
					disabled={currentPage === 1}
					class="cursor-pointer rounded border border-[rgba(255,255,255,0.06)] px-3 py-1.5 transition-colors hover:text-white disabled:opacity-30"
				>
					« Prev Page
				</button>

				<span>Page {currentPage} of {totalPages}</span>

				<button
					onclick={() => changePage(currentPage + 1)}
					disabled={currentPage === totalPages}
					class="cursor-pointer rounded border border-[rgba(255,255,255,0.06)] px-3 py-1.5 transition-colors hover:text-white disabled:opacity-30"
				>
					Next Page »
				</button>
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
