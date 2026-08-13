<script lang="ts">
	import { onMount } from 'svelte';
	import type { PageData } from './$types';
	import type { News } from '$lib/mockData';
	import Image from '$lib/Image.svelte';

	let { data }: { data: PageData } = $props();

	const article = $derived(data.article as News);
	const related = $derived(data.related as News[]);

	function formatDate(dateStr: string) {
		const d = new Date(dateStr);
		return d.toLocaleDateString('en-US', {
			year: 'numeric',
			month: 'long',
			day: 'numeric'
		});
	}

	function getReadingTime(text: string) {
		const words = text.split(/\s+/).length;
		const minutes = Math.ceil(words / 220);
		return `${minutes} min read`;
	}

	// Feedback Form State
	let opinion = $state('');
	let email = $state('');
	let formSubmitted = $state(false);

	function handleFeedback(e: Event) {
		e.preventDefault();
		if (opinion.trim()) {
			formSubmitted = true;
			opinion = '';
			email = '';
		}
	}

	function getViewerKey() {
		const existing = localStorage.getItem('nw_viewer_id');
		if (existing) return existing;

		const generated =
			typeof crypto !== 'undefined' && 'randomUUID' in crypto
				? crypto.randomUUID()
				: `nw-${Date.now()}-${Math.random().toString(36).slice(2)}`;

		localStorage.setItem('nw_viewer_id', generated);
		return generated;
	}

	onMount(() => {
		const viewerKey = getViewerKey();
		fetch(`http://localhost:8080/api/news/${article.id}/view`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ viewer_key: viewerKey })
		}).catch((error) => {
			console.warn('view tracking failed', error);
		});
	});
</script>

<svelte:head>
	<title>{article.title} | Neuralwire</title>
	<meta name="description" content={article.summary} />
	<!-- Article Specific OG -->
	<meta property="og:title" content={article.title} />
	<meta property="og:description" content={article.summary} />
	<meta property="og:type" content="article" />
	{#if article.image_url}
		<meta property="og:image" content={article.image_url} />
	{/if}
</svelte:head>

<article class="relative w-full flex-grow pb-16">
	<!-- Top Header Grid Background -->
	<div
		class="bg-grid-pattern pointer-events-none absolute inset-0 h-[500px] border-b border-[rgba(255,255,255,0.03)] bg-gradient-to-b from-[#0F172A]/10 to-transparent opacity-10"
	></div>

	<!-- Main Container -->
	<div class="relative z-10 mx-auto max-w-4xl px-4 pt-12 sm:px-6 md:pt-16 lg:px-8">
		<!-- Back to Feed link -->
		<div class="mb-8">
			<a
				href="/"
				class="group inline-flex items-center space-x-2 font-mono text-xs text-slate-400 transition-colors hover:text-[#22D3EE]"
			>
				<svg
					class="h-4 w-4 transform transition-transform group-hover:-translate-x-1"
					fill="none"
					viewBox="0 0 24 24"
					stroke="currentColor"
				>
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						stroke-width="2"
						d="M10 19l-7-7m0 0l7-7m-7 7h18"
					/>
				</svg>
				<span>RETURN TO CHRONICLE FEED</span>
			</a>
		</div>

		<!-- Category & Date Header -->
		<div class="mb-6 flex flex-wrap items-center gap-3 text-xs">
			<a
				href="/category/{article.category}"
				class="tag-mono rounded border border-[#22D3EE]/30 bg-[#22D3EE]/5 px-2.5 py-0.5 font-bold text-[#22D3EE] uppercase transition-all hover:bg-[#22D3EE]/10"
			>
				{article.category.replace('-', ' ')}
			</a>
			<span class="font-mono text-slate-600">•</span>
			<span class="font-mono text-slate-400">{formatDate(article.published_at)}</span>
			<span class="font-mono text-slate-600">•</span>
			<span class="font-mono font-bold text-[#22D3EE]">AI DIGEST SUMMARY</span>
		</div>

		<!-- Title & Subtitle -->
		<h1
			class="mb-8 font-serif text-3xl leading-tight font-medium text-white sm:text-4xl md:text-5xl"
		>
			{article.title}
		</h1>

		<!-- Large Main Image -->
		<div
			class="relative mb-12 aspect-[16/9] w-full overflow-hidden rounded-2xl border border-[rgba(255,255,255,0.08)] bg-[#0F172A]"
		>
			<Image
				src={article.image_url}
				content={article.content}
				alt={article.title}
				class="h-full w-full object-cover opacity-90 grayscale"
			/>
			<div
				class="absolute bottom-4 left-4 rounded-lg border border-[rgba(255,255,255,0.05)] bg-[#0A0E17]/80 px-3 py-1.5 font-mono text-[10px] text-slate-400 backdrop-blur-sm"
			>
				SOURCE: {article.source.toUpperCase()}
			</div>
		</div>
		<!-- Content Presentation -->
		<div class="mb-16 space-y-10">
			<!-- AI Digest Highlight Card (curated brief is the main content) -->
			{#if article.summary}
				<div
					class="rounded-2xl border border-[#22D3EE]/15 bg-[#22D3EE]/3 p-6 backdrop-blur-sm md:p-8"
				>
					<span class="mb-3 block font-mono text-[10px] font-bold tracking-wider text-[#22D3EE]">
						// NEURAL AI DIGEST
					</span>
					<div
						class="article-content max-w-none font-sans text-base leading-relaxed font-light text-slate-200 md:text-lg"
					>
						<p>{article.summary}</p>
					</div>
				</div>
			{:else}
				<div
					class="rounded-2xl border border-[#22D3EE]/15 bg-[#22D3EE]/3 p-6 backdrop-blur-sm md:p-8"
				>
					<span class="mb-3 block font-mono text-[10px] font-bold tracking-wider text-[#22D3EE]">
						// NEURAL AI DIGEST
					</span>
					<div
						class="article-content max-w-none font-sans text-base leading-relaxed font-light text-slate-200 md:text-lg"
					>
						<p>This brief is being prepared. Read the full story at the original source below.</p>
					</div>
				</div>
			{/if}

			<!-- External Source CTA Card -->
			<div
				class="relative my-12 overflow-hidden rounded-2xl border border-[rgba(255,255,255,0.08)] bg-[#0F172A]/40 p-6 text-center backdrop-blur-sm md:p-8"
			>
				<div
					class="pointer-events-none absolute top-[-50%] right-[-50%] h-64 w-64 rounded-full bg-[#22D3EE]/3 blur-3xl"
				></div>
				<h3 class="mb-3 font-mono text-[10px] font-bold tracking-widest text-[#22D3EE] uppercase">
					// ORIGINAL COVERAGE
				</h3>
				<p class="mx-auto mb-6 max-w-lg font-sans text-xs leading-relaxed text-slate-400">
					For interactive features, code blocks, or full research diagrams, read the original
					publication.
				</p>
				<a
					href={article.url}
					target="_blank"
					rel="noopener noreferrer"
					class="inline-block rounded-xl bg-[#22D3EE] px-8 py-3.5 font-mono text-xs font-bold tracking-widest text-[#0A0E17] uppercase shadow-[0_0_15px_rgba(34,211,238,0.2)] transition-all hover:scale-[1.02] hover:bg-[#22D3EE]/90 active:scale-[0.98]"
				>
					READ FULL STORY ON {article.source.toUpperCase()}
				</a>
			</div>
		</div>
		<!-- Share & Source telemetry footer -->
		<div
			class="mb-16 flex flex-col items-center justify-between gap-4 border-y border-[rgba(255,255,255,0.08)] py-6 font-mono text-xs text-slate-500 sm:flex-row"
		>
			<div>
				<span>TELEMETRY: NW-ID-{article.id}</span>
				<span class="mx-3">|</span>
				<span>ORIGIN: {article.source.toUpperCase()}</span>
			</div>

			<div class="flex items-center space-x-4">
				<span class="text-slate-400">SHARE:</span>
				<button class="cursor-pointer transition-colors hover:text-[#22D3EE]">[ X ]</button>
				<button class="cursor-pointer transition-colors hover:text-[#22D3EE]">[ LINKEDIN ]</button>
				<button class="cursor-pointer transition-colors hover:text-[#22D3EE]">[ REDDIT ]</button>
			</div>
		</div>

		<!-- Telemetry feedback form -->
		<section
			class="relative mb-20 overflow-hidden rounded-2xl border border-[rgba(255,255,255,0.08)] bg-[#0F172A]/30 p-6 md:p-8"
		>
			<!-- subtle glow -->
			<div
				class="pointer-events-none absolute top-[-10%] right-[-10%] h-32 w-32 rounded-full bg-[#22D3EE]/3 blur-2xl"
			></div>

			<span class="tag-mono mb-2 block text-[10px] font-bold tracking-wider text-[#22D3EE]"
				>// FEEDBACK_RECEIVER_V2.0</span
			>
			<h3 class="mb-2 font-serif text-xl font-medium text-white">OPINION TRANSMISSION</h3>
			<p class="mb-6 font-sans text-xs leading-relaxed font-light text-slate-400">
				Provide your mathematical or ethical stance on this report. All submissions are queued for
				model adjustment evaluation.
			</p>

			{#if formSubmitted}
				<div class="rounded-xl border border-[#22D3EE]/30 bg-[#22D3EE]/5 p-6 text-center">
					<svg
						class="mx-auto mb-3 h-8 w-8 animate-pulse text-[#22D3EE]"
						fill="none"
						viewBox="0 0 24 24"
						stroke="currentColor"
					>
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
						/>
					</svg>
					<p class="mb-1 font-mono text-sm tracking-wider text-[#22D3EE] uppercase">
						TRANSMISSION RECEIVED
					</p>
					<p class="font-sans text-[11px] text-slate-400">
						Telemetry logs have been queued. Thank you for participating in public cognition cycles.
					</p>
				</div>
			{:else}
				<form onsubmit={handleFeedback} class="space-y-4">
					<div>
						<label
							for="opinion"
							class="mb-1.5 block font-mono text-[10px] tracking-wider text-slate-400 uppercase"
							>Opinion Content</label
						>
						<textarea
							id="opinion"
							rows="4"
							placeholder="Type opinion here..."
							bind:value={opinion}
							required
							class="w-full rounded-xl border border-[rgba(255,255,255,0.08)] bg-[#070A10] px-3 py-2.5 font-mono text-xs text-slate-100 placeholder-slate-600 focus:border-[#22D3EE]/50 focus:outline-none"
						></textarea>
					</div>

					<div class="grid grid-cols-1 items-end gap-4 sm:grid-cols-2">
						<div>
							<label
								for="email"
								class="mb-1.5 block font-mono text-[10px] tracking-wider text-slate-400 uppercase"
								>Email Signature (Optional)</label
							>
							<input
								type="email"
								id="email"
								placeholder="quantum@neutralwire.media"
								bind:value={email}
								class="w-full rounded-xl border border-[rgba(255,255,255,0.08)] bg-[#070A10] px-3 py-2 font-mono text-xs text-slate-100 placeholder-slate-600 focus:border-[#22D3EE]/50 focus:outline-none"
							/>
						</div>
						<div class="text-right">
							<button
								type="submit"
								class="accent-glow-glow w-full cursor-pointer rounded-xl bg-[#22D3EE] px-6 py-2 font-mono text-xs font-bold tracking-widest text-[#0A0E17] uppercase transition-all hover:bg-[#22D3EE]/90 sm:w-auto"
							>
								EXECUTE_SUBMIT
							</button>
						</div>
					</div>
				</form>
			{/if}
		</section>

		<!-- Related Recommendations Feed -->
		{#if related.length > 0}
			<section class="border-t border-[rgba(255,255,255,0.08)] pt-12">
				<h3 class="mb-6 font-mono text-xs font-bold tracking-widest text-[#22D3EE] uppercase">
					// RELATED_TRANSMISSIONS
				</h3>
				<div class="grid grid-cols-1 gap-6 md:grid-cols-3">
					{#each related as post}
						<article
							class="group glow-hover flex flex-col overflow-hidden rounded-xl border border-[rgba(255,255,255,0.08)] bg-[#0F172A]/20"
						>
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

							<div class="flex flex-grow flex-col space-y-2 p-4">
								<div class="font-mono text-[9px] text-slate-500">
									{formatDate(post.published_at)}
								</div>
								<h4
									class="line-clamp-2 font-serif text-sm leading-snug font-normal text-white transition-colors group-hover:text-[#22D3EE]"
								>
									<a href="/{post.slug}">
										{post.title}
									</a>
								</h4>
							</div>
						</article>
					{/each}
				</div>
			</section>
		{/if}
	</div>
</article>

<style>
	/* Custom rich text rendering styles for article body */
	:global(.article-content p) {
		margin-bottom: 1.5rem;
		line-height: 1.8;
		font-weight: 300;
		color: #cbd5e1; /* slate-300 */
	}

	:global(.article-content h2) {
		font-family: 'Playfair Display', Georgia, serif;
		font-size: 1.5rem;
		font-weight: 500;
		color: #ffffff;
		margin-top: 2.25rem;
		margin-bottom: 1rem;
		padding-bottom: 0.5rem;
		border-bottom: 1px solid rgba(255, 255, 255, 0.08);
	}

	@media (min-width: 768px) {
		:global(.article-content h2) {
			font-size: 1.75rem;
		}
	}

	:global(.article-content blockquote) {
		font-family: 'Playfair Display', Georgia, serif;
		font-style: italic;
		font-size: 1.125rem;
		border-left: 2px solid #22d3ee;
		padding-left: 1.25rem;
		padding-top: 0.75rem;
		padding-bottom: 0.75rem;
		margin: 2rem 0;
		background-color: rgba(34, 211, 238, 0.03);
		border-top-right-radius: 0.5rem;
		border-bottom-right-radius: 0.5rem;
	}

	:global(.article-content em) {
		color: #ffffff;
	}
</style>
