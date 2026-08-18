<script lang="ts">
	import './layout.css';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { getSiteUrl } from '$lib/siteUrl';

	let { children, data } = $props();

	// JSON-LD structured data, rendered into <svelte:head> below. The tag is
	// assembled via concatenation so the raw tag opener never appears
	// literally in this file (Svelte's parser would otherwise treat it as a
	// second top-level tag). `</` is escaped so a value containing the
	// closing tag sequence cannot break out of the tag.
	const websiteJsonLdHtml =
		'<scr' +
		'ipt type="application/ld+json">' +
		JSON.stringify({
			'@context': 'https://schema.org',
			'@type': 'WebSite',
			name: 'Neuralwire',
			url: getSiteUrl()
		}).replace(/</g, '\\u003c') +
		'</scr' +
		'ipt>';
	const organizationJsonLdHtml =
		'<scr' +
		'ipt type="application/ld+json">' +
		JSON.stringify({
			'@context': 'https://schema.org',
			'@type': 'Organization',
			name: 'Neuralwire',
			url: getSiteUrl(),
			logo: {
				'@type': 'ImageObject',
				url: getSiteUrl() + '/favicon.svg'
			}
		}).replace(/</g, '\\u003c') +
		'</scr' +
		'ipt>';

	// Search state
	let searchQuery = $state('');
	let isMobileMenuOpen = $state(false);

	function handleSearch(e: Event) {
		e.preventDefault();
		if (searchQuery.trim()) {
			goto(`/search?q=${encodeURIComponent(searchQuery.trim())}`);
			searchQuery = ''; // clear after submit
			isMobileMenuOpen = false;
		}
	}

	function toggleMobileMenu() {
		isMobileMenuOpen = !isMobileMenuOpen;
	}
</script>

<svelte:head>
	<!-- Preconnect to the most-used article image hosts (from live DB data) so
		 feed/hero images start downloading sooner. -->
	<link rel="preconnect" href="https://storage.googleapis.com" />
	<link rel="preconnect" href="https://d2908q01vomqb2.cloudfront.net" />
	<link rel="preconnect" href="https://lh3.googleusercontent.com" />
	<link rel="preconnect" href="https://news.mit.edu" />
	<link rel="preconnect" href="https://images.ctfassets.net" />
	<link rel="preconnect" href="https://images.unsplash.com" />
	<!-- Primary SEO Meta Tags (page-level heads override with page-specific tags) -->
	<title>Neuralwire | AI News, Neural Networks & Future Computation</title>
	<meta
		name="description"
		content="An editorial news portal for artificial intelligence, neural networks, and the future of computation. Bridging the gap between silicon and humanity."
	/>
	<meta name="viewport" content="width=device-width, initial-scale=1.0" />
	<meta name="robots" content="index, follow" />
	<meta property="og:site_name" content="Neuralwire" />
	<meta name="twitter:card" content="summary_large_image" />
	<!-- Structured data: WebSite + Organization -->
	{@html websiteJsonLdHtml}
	{@html organizationJsonLdHtml}
</svelte:head>

<div
	class="flex-column bg-grid-pattern relative flex min-h-screen flex-col overflow-x-hidden bg-[#0A0E17] text-slate-100"
>
	<!-- Ambient Background Glows -->
	<div
		class="pulse-glow-bg pointer-events-none absolute top-[-20%] left-[-10%] h-[50vw] w-[50vw] rounded-full bg-[#22D3EE]/5 blur-[120px]"
	></div>
	<div
		class="pulse-glow-bg pointer-events-none absolute right-[-10%] bottom-[20%] h-[40vw] w-[40vw] rounded-full bg-[#22D3EE]/3 blur-[100px]"
		style="animation-delay: -4s;"
	></div>

	<!-- Sticky Header -->
	<header
		class="sticky top-0 z-50 w-full border-b border-[rgba(255,255,255,0.08)] bg-[#0A0E17]/80 backdrop-blur-md"
	>
		<div class="mx-auto flex h-16 max-w-7xl items-center justify-between px-4 sm:px-6 lg:px-8">
			<!-- Logo -->
			<a href="/" class="group flex items-center space-x-2">
				<span
					class="rounded border border-[#22D3EE]/30 bg-[#22D3EE]/10 px-1.5 py-0.5 font-mono text-xs tracking-wider text-[#22D3EE] uppercase"
					>nw</span
				>
				<span class="font-sans text-lg font-bold tracking-tight text-white uppercase">
					Neural<span class="group-hover:glow-text text-[#22D3EE] transition-all duration-300"
						>wire</span
					>
				</span>
			</a>

			<!-- Desktop Nav Categories -->
			<nav class="hidden items-center space-x-1 md:flex">
				<a
					href="/"
					class="rounded-md px-3 py-2 font-mono text-xs tracking-wider uppercase transition-colors {$page
						.url.pathname === '/'
						? 'bg-[#22D3EE]/5 text-[#22D3EE]'
						: 'text-slate-400 hover:text-white'}"
				>
					FEED
				</a>
				{#each data.categories || [] as cat}
					<a
						href="/category/{cat.slug}"
						class="rounded-md px-3 py-2 font-mono text-xs tracking-wider uppercase transition-colors {$page
							.url.pathname === `/category/${cat.slug}`
							? 'bg-[#22D3EE]/5 text-[#22D3EE]'
							: 'text-slate-400 hover:text-white'}"
					>
						{cat.name}
					</a>
				{/each}
				<a
					href="/about"
					class="rounded-md px-3 py-2 font-mono text-xs tracking-wider uppercase transition-colors {$page
						.url.pathname === '/about'
						? 'bg-[#22D3EE]/5 text-[#22D3EE]'
						: 'text-slate-400 hover:text-white'}"
				>
					ABOUT
				</a>
			</nav>

			<!-- Search and Actions -->
			<div class="hidden items-center space-x-4 md:flex">
				<form onsubmit={handleSearch} class="relative">
					<input
						type="text"
						placeholder="Search archives..."
						bind:value={searchQuery}
						class="w-48 rounded-lg border border-[rgba(255,255,255,0.08)] bg-[#0F172A] px-3 py-1.5 pl-8 font-mono text-xs text-slate-200 placeholder-slate-500 transition-all focus:border-[#22D3EE]/50 focus:ring-1 focus:ring-[#22D3EE]/30 focus:outline-none"
					/>
					<svg
						class="absolute top-2.5 left-2.5 h-3.5 w-3.5 text-slate-500"
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
				</form>
			</div>

			<!-- Mobile Menu Button -->
			<div class="flex items-center space-x-2 md:hidden">
				<button
					onclick={toggleMobileMenu}
					class="rounded-md p-2 text-slate-400 hover:bg-slate-800/30 hover:text-white focus:outline-none"
					aria-label="Toggle Menu"
				>
					{#if isMobileMenuOpen}
						<svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
							<path
								stroke-linecap="round"
								stroke-linejoin="round"
								stroke-width="2"
								d="M6 18L18 6M6 6l12 12"
							/>
						</svg>
					{:else}
						<svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
							<path
								stroke-linecap="round"
								stroke-linejoin="round"
								stroke-width="2"
								d="M4 6h16M4 12h16M4 18h16"
							/>
						</svg>
					{/if}
				</button>
			</div>
		</div>

		<!-- Mobile Dropdown Menu -->
		{#if isMobileMenuOpen}
			<div
				class="space-y-2 border-b border-[rgba(255,255,255,0.08)] bg-[#0A0E17] px-4 pt-2 pb-4 md:hidden"
			>
				<form onsubmit={handleSearch} class="relative mb-3 w-full">
					<input
						type="text"
						placeholder="Search..."
						bind:value={searchQuery}
						class="w-full rounded-lg border border-[rgba(255,255,255,0.08)] bg-[#0F172A] px-3 py-2 pl-8 font-mono text-sm text-slate-200 placeholder-slate-500 focus:border-[#22D3EE]/50 focus:outline-none"
					/>
					<svg
						class="absolute top-3 left-2.5 h-4 w-4 text-slate-500"
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
				</form>
				<a
					href="/"
					onclick={() => (isMobileMenuOpen = false)}
					class="block rounded-md px-3 py-2 font-mono text-sm tracking-wider uppercase {$page.url
						.pathname === '/'
						? 'bg-[#22D3EE]/5 text-[#22D3EE]'
						: 'text-slate-400 hover:text-white'}"
				>
					FEED
				</a>
				{#each data.categories || [] as cat}
					<a
						href="/category/{cat.slug}"
						onclick={() => (isMobileMenuOpen = false)}
						class="block rounded-md px-3 py-2 font-mono text-sm tracking-wider uppercase {$page.url
							.pathname === `/category/${cat.slug}`
							? 'bg-[#22D3EE]/5 text-[#22D3EE]'
							: 'text-slate-400 hover:text-white'}"
					>
						{cat.name}
					</a>
				{/each}
				<a
					href="/about"
					onclick={() => (isMobileMenuOpen = false)}
					class="block rounded-md px-3 py-2 font-mono text-sm tracking-wider uppercase {$page.url
						.pathname === '/about'
						? 'bg-[#22D3EE]/5 text-[#22D3EE]'
						: 'text-slate-400 hover:text-white'}"
				>
					ABOUT
				</a>
			</div>
		{/if}
	</header>

	<!-- Main Content Area -->
	<main class="flex w-full flex-grow flex-col">
		{@render children()}
	</main>

	<!-- Footer -->
	<footer class="relative z-10 mt-auto border-t border-[rgba(255,255,255,0.08)] bg-[#070A10] py-12">
		<div class="mx-auto grid max-w-7xl grid-cols-1 gap-8 px-4 sm:px-6 md:grid-cols-4 lg:px-8">
			<!-- Col 1: Branding -->
			<div class="md:col-span-2">
				<a href="/" class="mb-4 flex items-center space-x-2">
					<span
						class="rounded border border-[#22D3EE]/30 bg-[#22D3EE]/10 px-1.5 py-0.5 font-mono text-xs tracking-wider text-[#22D3EE] uppercase"
						>nw</span
					>
					<span class="font-sans text-lg font-bold tracking-tight text-white uppercase">
						Neural<span class="text-[#22D3EE]">wire</span>
					</span>
				</a>
				<p class="max-w-sm text-xs leading-relaxed text-slate-400">
					Neuralwire is a professional tech media and editorial chronicle detailing the developments
					of artificial intelligence, neural networks, quantum computing, and the philosophical
					intersections of code and carbon.
				</p>
			</div>

			<!-- Col 2: Navigation -->
			<div>
				<h3 class="mb-4 font-mono text-xs font-bold tracking-wider text-[#22D3EE] uppercase">
					Navigation
				</h3>
				<ul class="space-y-2 font-mono text-xs">
					<li>
						<a href="/" class="text-slate-400 transition-colors hover:text-white">NEWS FEED</a>
					</li>
					<li>
						<a href="/search" class="text-slate-400 transition-colors hover:text-white"
							>SEARCH ARCHIVE</a
						>
					</li>
					<li>
						<a href="/about" class="text-slate-400 transition-colors hover:text-white"
							>ABOUT EDITORIAL</a
						>
					</li>
					<li>
						<a href="/copyright" class="text-slate-400 transition-colors hover:text-white"
							>COPYRIGHT & DMCA</a
						>
					</li>
				</ul>
			</div>

			<!-- Col 3: Categories -->
			<div>
				<h3 class="mb-4 font-mono text-xs font-bold tracking-wider text-[#22D3EE] uppercase">
					Categories
				</h3>
				<ul class="space-y-2 font-mono text-xs">
					{#each data.categories || [] as cat}
						<li>
							<a
								href="/category/{cat.slug}"
								class="text-slate-400 uppercase transition-colors hover:text-white"
							>
								{cat.name}
							</a>
						</li>
					{/each}
				</ul>
			</div>
		</div>

		<div
			class="mx-auto mt-12 flex max-w-7xl flex-col items-center justify-between border-t border-[rgba(255,255,255,0.05)] px-4 pt-8 font-mono text-xs text-slate-500 sm:flex-row sm:px-6 lg:px-8"
		>
			<p>© 2026 NEURALWIRE MEDIA. ALL RIGHTS RESERVED.</p>
		</div>
	</footer>
</div>
