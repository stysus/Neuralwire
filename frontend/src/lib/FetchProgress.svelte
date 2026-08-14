<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { BASE_URL } from '$lib/api';

	interface FetchProgressData {
		running: boolean;
		total_sources: number;
		done_sources: number;
		current_source: string;
		percent: number;
		started_at?: string;
	}

	// active hints that THIS page started a fetch, but progress visibility is
	// driven by the backend's running flag so it survives page refreshes.
	let { active = false } = $props<{ active?: boolean }>();

	let progress = $state<FetchProgressData | null>(null);
	let pollTimer: ReturnType<typeof setInterval> | undefined;
	let cancelling = $state(false);
	let cancelledToast = $state(false);

	async function pollProgress() {
		const token = localStorage.getItem('admin_token');
		if (!token) return;
		try {
			const res = await fetch(`${BASE_URL}/admin/fetch/progress`, {
				headers: { Authorization: `Bearer ${token}` }
			});
			if (res.ok) {
				progress = await res.json();
			}
		} catch (e) {
			console.warn('fetch progress poll failed', e);
		}
	}

	async function cancelFetch() {
		const token = localStorage.getItem('admin_token');
		if (!token || cancelling) return;
		cancelling = true;
		try {
			await fetch(`${BASE_URL}/admin/fetch/cancel`, {
				method: 'POST',
				headers: { Authorization: `Bearer ${token}` }
			});
			cancelledToast = true;
			// Poll once more so the UI flips to stopped quickly.
			setTimeout(pollProgress, 500);
			setTimeout(() => (cancelledToast = false), 4000);
		} catch (e) {
			console.error('cancel fetch failed', e);
		} finally {
			cancelling = false;
		}
	}

	// Poll on mount whenever a token exists; running state comes from the
	// backend, so a refresh mid-fetch still shows the live progress bar.
	onMount(() => {
		pollProgress();
		pollTimer = setInterval(pollProgress, 1500);
	});

	onDestroy(() => {
		if (pollTimer) {
			clearInterval(pollTimer);
		}
	});

	// Stop polling shortly after the backend reports not running to reduce
	// needless requests, but keep the component mounted while a cycle is live.
	let idleTimer: ReturnType<typeof setTimeout> | undefined;
	$effect(() => {
		if (progress && !progress.running && idleTimer === undefined) {
			idleTimer = setTimeout(() => {
				if (pollTimer) clearInterval(pollTimer);
				pollTimer = undefined;
			}, 3000);
		}
		if (progress && progress.running && idleTimer !== undefined) {
			clearTimeout(idleTimer);
			idleTimer = undefined;
			// Resume polling if it was stopped earlier.
			if (pollTimer === undefined) {
				pollProgress();
				pollTimer = setInterval(pollProgress, 1500);
			}
		}
	});
</script>

{#if progress?.running}
	<div
		class="mb-6 rounded-xl border border-[#22D3EE]/30 bg-[#0F172A]/40 p-5 font-mono text-xs"
		data-active={active}
	>
		<div class="mb-3 flex flex-wrap items-center justify-between gap-2">
			<span class="animate-pulse font-bold tracking-widest text-[#22D3EE] uppercase"
				>Fetch Cycle in Progress</span
			>
			<span class="text-slate-300">
				{progress.done_sources}/{progress.total_sources} SOURCES
				<span class="ml-2 font-bold text-[#22D3EE]">{progress.percent}%</span>
			</span>
		</div>

		<!-- Animated percentage bar -->
		<div class="relative h-2 w-full overflow-hidden rounded-full bg-slate-800">
			<div
				class="h-full rounded-full bg-gradient-to-r from-[#22D3EE] via-cyan-400 to-[#22D3EE] transition-all duration-700 ease-out"
				style="width: {progress.percent}%"
			></div>
			<!-- Shimmer overlay -->
			<div
				class="animate-shimmer absolute inset-0 w-1/3 bg-gradient-to-r from-transparent via-white/30 to-transparent"
			></div>
		</div>

		<div class="mt-3 flex flex-wrap items-center justify-between gap-2 text-[10px] text-slate-400">
			<span>
				<span class="text-slate-500 uppercase">TARGET:</span>
				<span class="text-slate-300">{progress.current_source || '—'}</span>
			</span>
			<div class="flex items-center gap-3">
				{#if cancelledToast}
					<span class="animate-pulse text-amber-400 uppercase">cancellation requested…</span>
				{:else}
					<span class="animate-pulse text-[#22D3EE]/70 uppercase">processing…</span>
				{/if}
				<button
					onclick={cancelFetch}
					disabled={cancelling}
					class="cursor-pointer rounded border border-[#E11D48]/40 bg-[#E11D48]/10 px-2.5 py-1 font-bold text-[#E11D48] uppercase transition-all hover:bg-[#E11D48]/20 hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
				>
					{cancelling ? '[CANCELLING...]' : '[CANCEL_FETCH]'}
				</button>
			</div>
		</div>
	</div>
{/if}

<style>
	@keyframes shimmer {
		0% {
			transform: translateX(-150%);
		}
		100% {
			transform: translateX(350%);
		}
	}
	.animate-shimmer {
		animation: shimmer 1.8s linear infinite;
	}
</style>
