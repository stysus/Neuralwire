<script lang="ts">
	import { extractFirstImage } from './image';

	let {
		src,
		content,
		alt = '',
		class: className = '',
		loading = 'lazy'
	}: {
		src?: string | null;
		content?: string | null;
		alt?: string;
		class?: string;
		// Images below the fold default to lazy loading; set `loading="eager"`
		// for the above-the-fold hero/featured image so it paints immediately
		// and stays crawlable without scroll-triggered fetching.
		loading?: 'lazy' | 'eager';
	} = $props();

	// Computed array of image source candidates
	let candidates = $derived.by(() => {
		const list: string[] = [];
		if (src && (src.startsWith('http://') || src.startsWith('https://') || src.startsWith('/'))) {
			list.push(src);
		}
		if (content) {
			const extracted = extractFirstImage(content);
			if (extracted && extracted !== src) {
				list.push(extracted);
			}
		}
		return list;
	});

	let attemptIndex = $state(0);

	// The active image URL to try rendering
	let activeSrc = $derived.by(() => {
		if (attemptIndex < candidates.length) {
			return candidates[attemptIndex];
		}
		return null;
	});

	// Reset search attempt index if candidate list changes (e.g. page navigation)
	$effect(() => {
		if (candidates.length >= 0) {
			attemptIndex = 0;
		}
	});

	function handleError() {
		attemptIndex += 1;
	}
</script>

{#if activeSrc}
	<img src={activeSrc} {alt} {loading} decoding="async" class={className} onerror={handleError} />
{:else}
	<!-- Subtle cybernetic placeholder graphic -->
	<div
		class="{className} relative flex items-center justify-center overflow-hidden bg-gradient-to-br from-[#0A0E17] to-[#0F172A]"
	>
		<div class="bg-grid-pattern absolute inset-0 opacity-10"></div>
		<div class="absolute h-20 w-20 animate-pulse rounded-full bg-[#22D3EE]/3 blur-2xl"></div>
		<span
			class="pointer-events-none font-mono text-[9px] tracking-widest text-[#22D3EE]/30 uppercase"
			>NEURALWIRE</span
		>
	</div>
{/if}
