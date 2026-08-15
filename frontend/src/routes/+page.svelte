<script lang="ts">
	import { onMount } from 'svelte';
	import type { PageData } from './$types';
	import type { News } from '$lib/mockData';
	import Image from '$lib/Image.svelte';
	import TrendingNews from '$lib/TrendingNews.svelte';
	import { getSiteUrl } from '$lib/siteUrl';

	let { data }: { data: PageData } = $props();

	// Active category filter (for the grid, leaving the hero as the absolute latest)
	let activeCategory = $state('all');
	let heroIndex = $state(0);
	let isHeroPaused = $state(false);
	let visibleCount = $state(15);

	$effect(() => {
		if (activeCategory) {
			visibleCount = 15;
		}
	});

	// Compute filtered news for the feed grid
	const articles = $derived(data.news as News[]);
	const heroArticles = $derived(articles.slice(0, 5));
	const featuredArticle = $derived(heroArticles[heroIndex] ?? articles[0]);
	const feedArticles = $derived(articles.slice(1));

	const filteredFeed = $derived(
		activeCategory === 'all'
			? feedArticles
			: feedArticles.filter((item) => item.category === activeCategory)
	);

	const visibleFeed = $derived(filteredFeed.slice(0, visibleCount));

	const categories = $derived([
		{ name: 'All News', slug: 'all' },
		...((data.categories as any[]) || []).map((c) => ({ name: c.name, slug: c.slug }))
	]);

	// Helpers
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
		const minutes = Math.ceil(words / 220); // average speed
		return `${minutes} min read`;
	}

	function getCategoryName(slug: string) {
		const found = data.categories?.find((c: any) => c.slug === slug);
		return found ? found.name : slug.replace('-', ' ');
	}

	function selectHero(index: number) {
		if (heroArticles.length === 0) return;
		heroIndex = (index + heroArticles.length) % heroArticles.length;
	}

	function nextHero() {
		selectHero(heroIndex + 1);
	}

	function previousHero() {
		selectHero(heroIndex - 1);
	}

	function collapseFeed() {
		visibleCount = 15;
		const element = document.getElementById('chronicle-feed');
		if (element) {
			element.scrollIntoView({ behavior: 'smooth' });
		}
	}

	$effect(() => {
		if (heroIndex >= heroArticles.length) {
			heroIndex = 0;
		}
	});

	onMount(() => {
		const interval = window.setInterval(() => {
			if (!isHeroPaused && heroArticles.length > 1) {
				nextHero();
			}
		}, 6500);

		return () => window.clearInterval(interval);
	});
</script>

<svelte:head>
	<link rel="canonical" href="{getSiteUrl()}/" />
	<meta property="og:title" content="Neuralwire | AI News & Editorial" />
	<meta
		property="og:description"
		content="An editorial news portal for artificial intelligence, neural networks, and the future of computation."
	/>
	<meta property="og:type" content="website" />
	<meta property="og:url" content="{getSiteUrl()}/" />
	<meta property="og:image" content="{getSiteUrl()}/favicon.svg" />
	<meta name="twitter:title" content="Neuralwire | AI News & Editorial" />
	<meta
		name="twitter:description"
		content="An editorial news portal for artificial intelligence, neural networks, and the future of computation."
	/>
	<meta name="twitter:image" content="{getSiteUrl()}/favicon.svg" />
</svelte:head>

<!-- Hero Section -->
{#if featuredArticle}
	<section
		class="relative overflow-hidden border-b border-[rgba(255,255,255,0.08)] bg-[#080B12]/80 py-12 md:py-20 lg:py-24"
		aria-label="Featured articles"
		onmouseenter={() => (isHeroPaused = true)}
		onmouseleave={() => (isHeroPaused = false)}
		onfocusin={() => (isHeroPaused = true)}
		onfocusout={() => (isHeroPaused = false)}
	>
		<!-- Subtle light beam behind hero -->
		<div
			class="absolute inset-0 bg-[radial-gradient(ellipse_at_top,rgba(34,211,238,0.05),transparent_60%)]"
		></div>

		<div class="relative z-10 mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
			{#key featuredArticle.id}
				<div class="hero-slide grid grid-cols-1 items-center gap-8 lg:grid-cols-12 lg:gap-12">
					<!-- Hero Text Details -->
					<div
						class="flex min-h-[340px] flex-col justify-center space-y-4 md:space-y-6 lg:col-span-7 lg:min-h-[380px]"
					>
						<!-- Category and Meta -->
						<div class="flex flex-wrap items-center gap-3 text-xs">
							<a
								href="/category/{featuredArticle.category}"
								class="tag-mono rounded border border-[#22D3EE]/30 bg-[#22D3EE]/5 px-2 py-0.5 font-bold text-[#22D3EE] transition-colors hover:bg-[#22D3EE]/10"
							>
								{getCategoryName(featuredArticle.category)}
							</a>
							<span class="font-mono text-slate-500">•</span>
							<span class="font-mono text-slate-400"
								>{formatDate(featuredArticle.published_at)}</span
							>
							<span class="font-mono text-slate-500">•</span>
							<span class="font-mono text-slate-400">{getReadingTime(featuredArticle.summary)}</span
							>
						</div>

						<!-- Hero Headline -->
						<a href="/{featuredArticle.slug}" class="group">
							<h1
								class="line-clamp-2 font-serif text-3xl leading-tight font-medium tracking-tight text-white transition-colors group-hover:text-[#22D3EE]/90 sm:text-4xl md:text-5xl lg:text-6xl"
							>
								{featuredArticle.title}
							</h1>
						</a>

						<!-- Hero Summary -->
						<p
							class="line-clamp-3 max-w-2xl font-sans text-sm leading-relaxed font-light text-slate-400 md:text-base"
						>
							{featuredArticle.summary}
						</p>

						<!-- Author / Source and Call to Action -->
						<div
							class="flex flex-col gap-4 border-t border-[rgba(255,255,255,0.05)] pt-4 sm:flex-row sm:items-center sm:justify-between"
						>
							<div class="flex items-center space-x-2">
								<div
									class="flex h-8 w-8 items-center justify-center rounded-full border border-[rgba(255,255,255,0.1)] bg-[#0F172A] font-mono text-[10px] text-slate-400 uppercase"
								>
									NW
								</div>
								<div>
									<p class="text-xs font-semibold text-slate-200">{featuredArticle.source}</p>
									<p class="font-mono text-[10px] text-slate-500">CORRESPONDENT</p>
								</div>
							</div>

							<a
								href="/{featuredArticle.slug}"
								class="group inline-flex items-center justify-center space-x-2 rounded-lg border border-[#22D3EE]/20 bg-[#22D3EE]/5 px-4 py-2 font-mono text-xs text-[#22D3EE] transition-all duration-300 hover:border-[#22D3EE]/50 hover:bg-[#22D3EE]/10 hover:text-white"
							>
								<span>READ FULL BRIEF</span>
								<svg
									class="h-3.5 w-3.5 transform transition-transform group-hover:translate-x-1"
									fill="none"
									viewBox="0 0 24 24"
									stroke="currentColor"
								>
									<path
										stroke-linecap="round"
										stroke-linejoin="round"
										stroke-width="2"
										d="M14 5l7 7m0 0l-7 7m7-7H3"
									/>
								</svg>
							</a>
						</div>
					</div>

					<!-- Hero Visual -->
					<div class="group relative lg:col-span-5">
						<!-- Glow underlying background -->
						<div
							class="absolute -inset-1 rounded-2xl bg-gradient-to-r from-cyan-500/20 to-purple-500/10 opacity-30 blur-xl transition duration-1000 group-hover:opacity-50 group-hover:duration-200"
						></div>

						<div
							class="relative aspect-[4/3] w-full scale-[0.99] overflow-hidden rounded-2xl border border-[rgba(255,255,255,0.08)] bg-[#0F172A] transition-all duration-500 group-hover:scale-100"
						>
							<Image
								src={featuredArticle.image_url}
								content={featuredArticle.content}
								alt={featuredArticle.title}
								loading="eager"
								class="h-full w-full object-cover opacity-90 transition-all duration-700 group-hover:opacity-100"
							/>
							<div
								class="absolute inset-0 bg-gradient-to-t from-[#0A0E17] via-transparent to-transparent opacity-40"
							></div>
						</div>
					</div>
				</div>
			{/key}

			{#if heroArticles.length > 1}
				<div
					class="mt-8 flex flex-col gap-4 border-t border-[rgba(255,255,255,0.06)] pt-5 sm:flex-row sm:items-center sm:justify-between"
				>
					<div class="flex items-center gap-3">
						<button
							type="button"
							onclick={previousHero}
							class="flex h-9 w-9 items-center justify-center rounded-lg border border-white/10 bg-[#0F172A]/50 text-slate-400 transition-all hover:border-[#22D3EE]/40 hover:bg-[#22D3EE]/5 hover:text-[#22D3EE]"
							aria-label="Previous featured article"
							title="Previous"
						>
							<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
								<path
									stroke-linecap="round"
									stroke-linejoin="round"
									stroke-width="2"
									d="M15 19l-7-7 7-7"
								/>
							</svg>
						</button>
						<button
							type="button"
							onclick={nextHero}
							class="flex h-9 w-9 items-center justify-center rounded-lg border border-white/10 bg-[#0F172A]/50 text-slate-400 transition-all hover:border-[#22D3EE]/40 hover:bg-[#22D3EE]/5 hover:text-[#22D3EE]"
							aria-label="Next featured article"
							title="Next"
						>
							<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
								<path
									stroke-linecap="round"
									stroke-linejoin="round"
									stroke-width="2"
									d="M9 5l7 7-7 7"
								/>
							</svg>
						</button>
					</div>

					<div class="flex items-center gap-4 sm:justify-end">
						<div class="flex items-center gap-2">
							{#each heroArticles as item, index}
								<button
									type="button"
									onclick={() => selectHero(index)}
									class="h-2.5 rounded-full transition-all {index === heroIndex
										? 'w-8 bg-[#22D3EE]'
										: 'w-2.5 bg-slate-700 hover:bg-slate-500'}"
									aria-label="Show featured article {index + 1}: {item.title}"
									aria-current={index === heroIndex ? 'true' : undefined}
								></button>
							{/each}
						</div>
						<div class="font-mono text-[10px] tracking-widest text-slate-500">
							{String(heroIndex + 1).padStart(2, '0')} / {String(heroArticles.length).padStart(
								2,
								'0'
							)}
						</div>
					</div>
				</div>
			{/if}
		</div>
	</section>
{/if}

<TrendingNews />

<!-- News Feed Section -->
<section id="chronicle-feed" class="mx-auto max-w-7xl px-4 py-12 sm:px-6 md:py-16 lg:px-8">
	<!-- Feed Title & Categories Filter -->
	<div
		class="mb-8 flex flex-col justify-between gap-4 border-b border-[rgba(255,255,255,0.08)] pb-6 md:flex-row md:items-end"
	>
		<div>
			<h2 class="mb-1 font-mono text-xs font-bold tracking-widest text-[#22D3EE] uppercase">
				Chronicle Feed
			</h2>
			<h3 class="font-serif text-2xl font-medium text-white md:text-3xl">LATEST TRANSMISSIONS</h3>
		</div>

		<!-- Category Tabs -->
		<div class="flex flex-wrap gap-2">
			{#each categories as cat}
				<button
					onclick={() => (activeCategory = cat.slug)}
					class="rounded-lg border px-3 py-1.5 font-mono text-xs tracking-wider uppercase transition-all
						{activeCategory === cat.slug
						? 'accent-glow-glow border-[#22D3EE]/40 bg-[#22D3EE]/10 text-[#22D3EE]'
						: 'border-[rgba(255,255,255,0.05)] bg-[#0F172A]/40 text-slate-400 hover:border-[rgba(255,255,255,0.15)] hover:text-white'}"
				>
					{cat.name}
				</button>
			{/each}
		</div>
	</div>

	<!-- News Grid -->
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
						<div class="absolute bottom-2 left-2">
							<span
								class="tag-mono rounded border border-[#22D3EE]/30 bg-[#0A0E17]/90 px-2 py-0.5 text-[10px] font-bold text-[#22D3EE] backdrop-blur-sm"
							>
								{getCategoryName(post.category)}
							</span>
						</div>
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

		{#if filteredFeed.length > visibleCount}
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
				There are no reports currently registered under the selected matrix category.
			</p>
		</div>
	{/if}
</section>

<style>
	.hero-slide {
		animation: hero-slide-in 520ms ease-out both;
	}

	@keyframes hero-slide-in {
		from {
			opacity: 0;
			transform: translateX(1.25rem);
		}
		to {
			opacity: 1;
			transform: translateX(0);
		}
	}
</style>
