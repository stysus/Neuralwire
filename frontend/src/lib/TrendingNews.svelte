<script lang="ts">
	import { onMount } from 'svelte';

	interface TrendingArticle {
		id: number;
		title: string;
		slug: string;
		source: string;
		image_url?: string;
		view_count: number;
	}

	interface TrendingResponse {
		window: string;
		data: TrendingArticle[];
	}

	let isLoading = $state(true);
	let articles = $state<TrendingArticle[]>([]);
	let errorMessage = $state('');
	let windowLabel = $state('week');

	function formatReads(count: number) {
		return `${count.toLocaleString('en-US')} ${count === 1 ? 'read' : 'reads'}`;
	}

	async function fetchTrending() {
		isLoading = true;
		errorMessage = '';

		try {
			const res = await fetch('http://localhost:8080/api/news/trending?window=week&limit=5', {
				headers: { Accept: 'application/json' }
			});

			if (!res.ok) {
				errorMessage = 'Trending signal unavailable.';
				return;
			}

			const result = (await res.json()) as TrendingResponse;
			windowLabel = result.window || 'week';
			articles = result.data || [];
		} catch (error) {
			console.warn('Trending news fetch failed', error);
			errorMessage = 'Trending signal unavailable.';
		} finally {
			isLoading = false;
		}
	}

	onMount(() => {
		fetchTrending();
	});
</script>

<section
	class="mx-auto w-full max-w-7xl px-4 py-10 sm:px-6 lg:px-8"
	aria-labelledby="trending-heading"
>
	<div
		class="border-y border-[rgba(255,255,255,0.08)] bg-[#0F172A]/10 py-6 backdrop-blur-sm"
	>
		<div class="mb-5 flex flex-col justify-between gap-2 sm:flex-row sm:items-end">
			<div>
				<h2
					id="trending-heading"
					class="mb-1 font-mono text-xs font-bold tracking-widest text-[#22D3EE] uppercase"
				>
					Trending Signal
				</h2>
				<p class="font-serif text-xl font-medium text-white md:text-2xl">MOST-READ THIS WEEK</p>
			</div>
			<div class="font-mono text-[10px] tracking-widest text-slate-500 uppercase">
				WINDOW: {windowLabel}
			</div>
		</div>

		{#if isLoading}
			<div class="grid gap-3 md:grid-cols-5">
				{#each Array(5) as _, index}
					<div
						class="h-24 animate-pulse rounded-lg border border-[rgba(255,255,255,0.06)] bg-[#0A0E17]/60"
						aria-label="Loading trending item {index + 1}"
					></div>
				{/each}
			</div>
		{:else if articles.length > 0}
			<div class="grid gap-3 md:grid-cols-5">
				{#each articles as article, index}
					<a
						href="/{article.slug}"
						class="group flex min-h-28 flex-col justify-between rounded-lg border border-[rgba(255,255,255,0.07)] bg-[#0A0E17]/45 p-4 transition-all hover:border-[#22D3EE]/35 hover:bg-[#22D3EE]/5"
					>
						<div class="mb-3 flex items-start justify-between gap-3">
							<span class="font-mono text-xl leading-none font-bold text-[#22D3EE]/80">
								{String(index + 1).padStart(2, '0')}
							</span>
							<span class="font-mono text-[10px] tracking-wider text-slate-500 uppercase">
								{formatReads(article.view_count || 0)}
							</span>
						</div>
						<h3
							class="line-clamp-2 font-serif text-sm leading-snug text-white transition-colors group-hover:text-[#22D3EE]"
						>
							{article.title}
						</h3>
						<div
							class="mt-3 border-t border-[rgba(255,255,255,0.05)] pt-3 font-mono text-[10px] tracking-wider text-slate-500 uppercase"
						>
							{article.source}
						</div>
					</a>
				{/each}
			</div>
		{:else}
			<div
				class="rounded-lg border border-dashed border-[rgba(255,255,255,0.08)] bg-[#0A0E17]/40 px-4 py-8 text-center"
			>
				<p class="font-mono text-xs tracking-wider text-slate-500 uppercase">
					{errorMessage || 'No trending articles registered yet.'}
				</p>
			</div>
		{/if}
	</div>
</section>
