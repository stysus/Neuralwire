<script lang="ts">
	import type { PageData } from './$types';
	import type { News } from '$lib/mockData';
	import { goto } from '$app/navigation';
	import Image from '$lib/Image.svelte';

	let { data }: { data: PageData } = $props();

	// Local query state synchronized from URL query
	let inputQuery = $state('');

	// Watch data.query and update inputQuery if it changes from external navigation
	$effect(() => {
		inputQuery = data.query;
	});

	function handleSubmit(e: Event) {
		e.preventDefault();
		goto(`/search?q=${encodeURIComponent(inputQuery.trim())}`);
	}

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

	const articles = $derived(data.news as News[]);
</script>

<svelte:head>
	<title>Search Archives | Neuralwire</title>
	<meta
		name="description"
		content="Search the Neuralwire archives for artificial intelligence news, documentation, and chronicles."
	/>
</svelte:head>

<section class="mx-auto max-w-7xl flex-grow px-4 py-12 sm:px-6 md:py-16 lg:px-8">
	<!-- Page Header & Input -->
	<div class="mb-12 border-b border-[rgba(255,255,255,0.08)] pb-8">
		<span class="tag-mono mb-2 block text-xs font-bold tracking-widest text-[#22D3EE]"
			>SYSTEM // SEARCH_ARCHIVES</span
		>
		<h1 class="mb-6 font-serif text-3xl font-medium text-white">INDEX RECONSTRUCTION</h1>

		<form onsubmit={handleSubmit} class="relative max-w-xl">
			<input
				type="text"
				placeholder="Enter search terms (e.g. Vatican, quantum, EU)..."
				bind:value={inputQuery}
				class="w-full rounded-xl border border-[rgba(255,255,255,0.08)] bg-[#0F172A]/60 px-4 py-3 pl-12 font-mono text-sm text-slate-100 placeholder-slate-500 transition-all focus:border-[#22D3EE]/50 focus:ring-1 focus:ring-[#22D3EE]/20 focus:outline-none"
			/>
			<svg
				class="absolute top-3.5 left-4 h-5 w-5 text-slate-500"
				fill="none"
				viewBox="0 0 24 24"
				stroke="currentColor"
			>
				<path
					stroke-linecap="round"
					stroke-linejoin="round"
					stroke-width="2"
					d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
				/>
			</svg>
			<button
				type="submit"
				class="absolute top-2 right-2 rounded-lg border border-[#22D3EE]/30 bg-[#22D3EE]/10 px-3 py-1.5 font-mono text-xs text-[#22D3EE] uppercase transition-all hover:bg-[#22D3EE]/20"
			>
				QUERY
			</button>
		</form>
	</div>

	<!-- Results Info -->
	<div class="mb-8">
		<p class="font-mono text-xs text-slate-400">
			{#if data.query}
				SEARCH RESULTS: FOUND <span class="font-bold text-[#22D3EE]">{articles.length}</span>
				RECORDS MATCHING <span class="text-white">"{data.query}"</span>
			{:else}
				ENTER A SEARCH QUERY TO QUERY THE ARCHIVES
			{/if}
		</p>
	</div>

	<!-- Results Feed Grid -->
	{#if articles.length > 0}
		<div class="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3 lg:gap-8">
			{#each articles as post}
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
							class="h-full w-full object-cover opacity-75 grayscale transition-all duration-550 group-hover:scale-105 group-hover:opacity-100 group-hover:grayscale-0"
						/>
					</a>

					<!-- Card Body -->
					<div class="flex flex-grow flex-col space-y-3 p-5">
						<!-- Meta -->
						<div class="flex items-center space-x-2 font-mono text-[10px] text-slate-500">
							<span class="font-bold text-slate-400">{post.category.toUpperCase()}</span>
							<span>•</span>
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
	{:else if data.query}
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
						d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728A9 9 0 015.636 5.636m12.728 12.728L5.636 5.636"
					/>
				</svg>
			</div>
			<p class="mb-2 font-mono text-sm tracking-wider text-[#22D3EE] uppercase">
				No Matching Records
			</p>
			<p class="mx-auto max-w-sm text-xs text-slate-500">
				Your query did not return any records in the archive index. Check spelling or try different
				keywords.
			</p>
		</div>
	{:else}
		<!-- Search Prompt State -->
		<div
			class="rounded-2xl border border-dashed border-[rgba(255,255,255,0.08)] bg-[#0F172A]/5 px-4 py-20 text-center"
		>
			<p class="font-mono text-xs tracking-widest text-slate-500 uppercase">
				Awaiting Command Input...
			</p>
		</div>
	{/if}
</section>
