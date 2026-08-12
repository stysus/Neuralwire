<script lang="ts">
	import { onMount } from 'svelte';
	import { page as pageStore } from '$app/stores';
	import { goto } from '$app/navigation';

	let currentPage = $derived(Number($pageStore.url.searchParams.get('page')) || 1);
	const pageSize = 10;

	let articles = $state<any[]>([]);
	let totalItems = $state(0);
	let totalPages = $state(0);

	let isLoading = $state(true);
	let errorMessage = $state('');
	let confirmingDeleteId = $state<number | null>(null);

	async function fetchPublished(pageNumber: number) {
		isLoading = true;
		errorMessage = '';
		const token = localStorage.getItem('admin_token');
		if (!token) return;

		try {
			const res = await fetch(
				`http://localhost:8080/api/admin/news?status=published&page=${pageNumber}&page_size=${pageSize}`,
				{
					headers: {
						Authorization: `Bearer ${token}`,
						Accept: 'application/json'
					}
				}
			);

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
				errorMessage = 'Failed to fetch published feed index.';
			}
		} catch (err) {
			console.error('Fetch published news error:', err);
			errorMessage = 'Database node link offline.';
		} finally {
			isLoading = false;
		}
	}

	onMount(() => {
		fetchPublished(currentPage);
	});

	$effect(() => {
		fetchPublished(currentPage);
	});

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
					goto(`/admin/published?page=${currentPage - 1}`);
				} else {
					fetchPublished(currentPage);
				}
			} else {
				alert('Failed to delete article.');
			}
		} catch (err) {
			console.error('Delete error:', err);
			alert('Network error executing delete.');
		}
	}

	function changePage(newPage: number) {
		if (newPage >= 1 && newPage <= totalPages) {
			goto(`/admin/published?page=${newPage}`);
		}
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
	<title>Published Log Index | Neuralwire</title>
</svelte:head>

<section
	class="mx-auto flex w-full max-w-7xl flex-grow flex-col justify-start px-4 py-12 sm:px-6 md:py-16 lg:px-8"
>
	<!-- Header -->
	<div
		class="mb-8 flex flex-col items-start justify-between gap-4 border-b border-[rgba(255,255,255,0.08)] pb-6 sm:flex-row sm:items-end"
	>
		<div>
			<span class="tag-mono mb-1 block text-xs font-bold tracking-widest text-[#22D3EE]"
				>SYS // PUBLISHED_INDEX</span
			>
			<h1 class="font-serif text-3xl font-medium text-white">LIVE ARCHIVE</h1>
		</div>
		<div class="font-mono text-xs text-slate-500">
			TOTAL_PUBLISHED: <span class="font-bold text-slate-300">{totalItems}</span> POST(S)
		</div>
	</div>

	<!-- Loading -->
	{#if isLoading}
		<div class="flex flex-grow items-center justify-center py-20">
			<div class="space-y-4 text-center font-mono text-xs">
				<div
					class="mx-auto h-6 w-6 animate-spin rounded-full border-2 border-slate-800 border-t-[#22D3EE]"
				></div>
				<div class="animate-pulse tracking-widest text-slate-500 uppercase">
					Syncing Index Stack...
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
			<p class="mb-1 font-mono text-xs tracking-wider text-slate-500 uppercase">Index Empty</p>
			<p class="font-sans text-xs text-slate-600">
				No published articles registered in the system registry.
			</p>
		</div>
	{:else}
		<!-- List -->
		<div class="flex-grow space-y-6">
			{#each articles as item}
				<div
					class="group glow-hover relative flex flex-col justify-between gap-6 rounded-xl border border-[rgba(255,255,255,0.08)] bg-[#0F172A]/20 p-6 md:flex-row"
				>
					<!-- Article Info -->
					<div class="flex-grow space-y-3">
						<div class="flex items-center space-x-3 font-mono text-[10px] text-slate-500">
							<span
								class="rounded border border-[#22D3EE]/20 bg-[#22D3EE]/5 px-1.5 py-0.5 font-bold text-[#22D3EE] uppercase"
								>{item.category}</span
							>
							<span>•</span>
							<span>{item.source.toUpperCase()}</span>
							<span>•</span>
							<span>PUBLISHED: {formatDate(item.published_at || item.created_at)}</span>
						</div>
						<h3
							class="font-serif text-lg leading-snug font-normal text-white transition-colors group-hover:text-[#22D3EE]"
						>
							{item.title}
						</h3>
						<p class="max-w-4xl font-sans text-xs leading-relaxed font-light text-slate-400">
							{item.summary}
						</p>
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
									>DELETE POST?</span
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
									[PREVIEW]
								</a>
								<button
									onclick={() => (confirmingDeleteId = item.id)}
									class="flex-1 cursor-pointer rounded border border-transparent px-2.5 py-1.5 font-mono text-[10px] text-slate-500 transition-all hover:border-[#E11D48]/50 hover:text-[#E11D48] md:w-full"
								>
									[DELETE]
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
					« PREV_PAGE
				</button>

				<span>PAGE {currentPage} OF {totalPages}</span>

				<button
					onclick={() => changePage(currentPage + 1)}
					disabled={currentPage === totalPages}
					class="cursor-pointer rounded border border-[rgba(255,255,255,0.06)] px-3 py-1.5 transition-colors hover:text-white disabled:opacity-30"
				>
					NEXT_PAGE »
				</button>
			</div>
		{/if}
	{/if}
</section>
