<script lang="ts">
	import type { PageData } from './$types';
	import type { News } from '$lib/mockData';
	import Image from '$lib/Image.svelte';
	import { getSiteUrl } from '$lib/siteUrl';

	let { data }: { data: PageData } = $props();

	const category = $derived(data.category);
	const articles = $derived(data.news as News[]);
	let visibleCount = $state(15);

	$effect(() => {
		if (category.slug) {
			visibleCount = 15;
		}
	});

	const visibleFeed = $derived(articles.slice(0, visibleCount));

	function formatDate(dateStr: string) {
		const d = new Date(dateStr);
		return d.toLocaleDateString('en-US', {
			year: 'numeric',
			month: 'short',
			day: 'numeric'
		});
	}

	function getReadingTime(text: string) {
		const words = text.split(/\s+/).length;
		const minutes = Math.ceil(words / 220);
		return `${minutes} min read`;
	}

	function collapseFeed() {
		visibleCount = 15;
		const element = document.getElementById('category-feed');
		if (element) {
			element.scrollIntoView({ behavior: 'smooth' });
		}
	}
</script>

<svelte:head>
	<title>{category.name} | Neuralwire AI News</title>
	<meta
		name="description"
		content="Explore news and in-depth articles about {category.name} from the editors of Neuralwire."
	/>
	<meta name="robots" content="index, follow" />
	<link rel="canonical" href="{getSiteUrl()}/category/{category.slug}" />
	<meta property="og:title" content="{category.name} | Neuralwire AI News" />
	<meta
		property="og:description"
		content="Explore news and in-depth articles about {category.name} from the editors of Neuralwire."
	/>
	<meta property="og:type" content="website" />
	<meta property="og:url" content="{getSiteUrl()}/category/{category.slug}" />
	<meta property="og:image" content="{getSiteUrl()}/favicon.svg" />
	<meta name="twitter:card" content="summary_large_image" />
	<meta name="twitter:title" content="{category.name} | Neuralwire AI News" />
	<meta
		name="twitter:description"
		content="Explore news and in-depth articles about {category.name} from the editors of Neuralwire."
	/>
	<meta name="twitter:image" content="{getSiteUrl()}/favicon.svg" />
</svelte:head>

<section id="category-feed" class="mx-auto max-w-7xl flex-grow px-4 py-12 sm:px-6 md:py-16 lg:px-8">
	<!-- Category Page Header -->
	<div class="mb-12 border-b border-[rgba(255,255,255,0.08)] pb-8">
		<span class="tag-mono mb-2 block text-xs font-bold tracking-widest text-[#22D3EE]"
			>CATEGORY</span
		>
		<h1 class="font-serif text-3xl font-medium tracking-tight text-white uppercase md:text-4xl">
			{category.name}
		</h1>
	</div>

	<!-- Articles Grid -->
	{#if visibleFeed.length > 0}
		<div class="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3 lg:gap-8 2xl:grid-cols-5">
			{#each visibleFeed as post}
				<article
					class="group glow-hover flex flex-col overflow-hidden rounded-xl border border-[rgba(255,255,255,0.08)] bg-[#0F172A]/25"
				>
					<!-- Thumbnail -->
					<a
						href="/{post.slug}"
						class="relative block aspect-[16/10] w-full overflow-hidden border-b border-[rgba(255,255,255,0.08)] bg-[#0A0E17]"
					>
						<Image
							src={post.image_url}
							content={post.content}
							alt={post.title}
							class="h-full w-full object-cover opacity-75 transition-all duration-550 group-hover:scale-105 group-hover:opacity-100"
						/>
					</a>

					<!-- Card Body -->
					<div class="flex flex-grow flex-col space-y-3 p-5">
						<!-- Meta -->
						<div class="flex items-center space-x-2 font-mono text-[10px] text-slate-500">
							<span>{formatDate(post.published_at)}</span>
							<span>•</span>
							<span>{getReadingTime(post.summary)}</span>
						</div>

						<!-- Title -->
						<h4
							class="flex-grow font-serif text-lg leading-snug font-normal text-white transition-colors group-hover:text-[#22D3EE]"
						>
							<a href="/{post.slug}" class="line-clamp-2">
								{post.title}
							</a>
						</h4>

						<!-- Summary -->
						<p class="line-clamp-3 font-sans text-xs leading-relaxed font-light text-slate-400">
							{post.summary}
						</p>

						<!-- Footer/Source -->
						<div
							class="flex items-center justify-between border-t border-[rgba(255,255,255,0.05)] pt-4 font-mono text-[10px] text-slate-500"
						>
							<span class="text-slate-400">{post.source.toUpperCase()}</span>
							<a
								href="/{post.slug}"
								class="flex items-center space-x-1 text-[#22D3EE]/70 group-hover:text-[#22D3EE]"
							>
								<span>READ</span>
								<svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
									<path
										stroke-linecap="round"
										stroke-linejoin="round"
										stroke-width="2"
										d="M9 5l7 7-7 7"
									/>
								</svg>
							</a>
						</div>
					</div>
				</article>
			{/each}
		</div>

		{#if articles.length > visibleCount}
			<div class="mt-12 flex justify-center gap-4">
				<button
					onclick={() => (visibleCount += 15)}
					class="cursor-pointer rounded-xl border border-[#22D3EE]/30 bg-[#22D3EE]/5 px-8 py-3 font-mono text-xs font-bold tracking-widest text-[#22D3EE] uppercase transition-all hover:border-[#22D3EE] hover:bg-[#22D3EE]/10 hover:text-white"
				>
					Load More
				</button>
				{#if visibleCount > 15}
					<button
						onclick={collapseFeed}
						class="cursor-pointer rounded-xl border border-[#E11D48]/30 bg-[#E11D48]/5 px-8 py-3 font-mono text-xs font-bold tracking-widest text-[#E11D48] uppercase transition-all hover:border-[#E11D48] hover:bg-[#E11D48]/10 hover:text-white"
					>
						Hide Feed
					</button>
				{/if}
			</div>
		{:else if visibleCount > 15}
			<div class="mt-12 flex justify-center">
				<button
					onclick={collapseFeed}
					class="cursor-pointer rounded-xl border border-[#E11D48]/30 bg-[#E11D48]/5 px-8 py-3 font-mono text-xs font-bold tracking-widest text-[#E11D48] uppercase transition-all hover:border-[#E11D48] hover:bg-[#E11D48]/10 hover:text-white"
				>
					Hide Feed
				</button>
			</div>
		{/if}

		{#if visibleCount > 15}
			<button
				onclick={collapseFeed}
				class="fixed right-6 bottom-6 z-40 flex h-10 cursor-pointer items-center justify-center space-x-2 rounded-full border border-[#E11D48]/40 bg-[#0A0E17]/90 px-4 py-2 font-mono text-[10px] font-bold tracking-widest text-[#E11D48] shadow-[0_0_15px_rgba(225,29,72,0.15)] backdrop-blur-sm transition-all hover:border-[#E11D48] hover:bg-[#E11D48]/10 hover:text-white active:scale-95"
				title="Collapse Feed"
			>
				<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						stroke-width="2.5"
						d="M5 15l7-7 7 7"
					/>
				</svg>
				<span>COLLAPSE FEED</span>
			</button>
		{/if}
	{:else}
		<!-- Empty State -->
		<div
			class="rounded-2xl border border-[rgba(255,255,255,0.08)] bg-[#0F172A]/25 px-4 py-24 text-center"
		>
			<div
				class="mx-auto mb-4 flex h-16 w-16 animate-pulse items-center justify-center rounded-full border border-dashed border-[#22D3EE]/30 bg-[#22D3EE]/5"
			>
				<svg class="h-6 w-6 text-[#22D3EE]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						stroke-width="2"
						d="M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.318.158a6 6 0 01-3.86.517L6.05 15.21a2 2 0 00-1.806.547M8 4h8l-1 1v5.172a2 2 0 00.586 1.414l5 5c1.26 1.26.367 3.414-1.415 3.414H4.828c-1.782 0-2.674-2.154-1.414-3.414l5-5A2 2 0 009 10.172V5L8 4z"
					/>
				</svg>
			</div>
			<p class="mb-2 font-mono text-sm tracking-wider text-[#22D3EE] uppercase">
				No Transmissions Found
			</p>
			<p class="mx-auto max-w-sm text-xs text-slate-500">
				There are no reports currently registered under the "{category.name}" category.
			</p>
		</div>
	{/if}
</section>
