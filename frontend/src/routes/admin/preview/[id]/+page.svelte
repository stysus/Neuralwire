<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import type { PageData } from './$types';
	import Image from '$lib/Image.svelte';

	let { data }: { data: PageData } = $props();

	let article = $state<any>(null);
	let isLoading = $state(true);
	let errorMessage = $state('');

	async function fetchArticleDetails() {
		isLoading = true;
		errorMessage = '';
		const token = localStorage.getItem('admin_token');
		if (!token) return;

		try {
			const res = await fetch(`http://localhost:8080/api/admin/news/${data.id}`, {
				headers: {
					Authorization: `Bearer ${token}`,
					Accept: 'application/json'
				}
			});

			if (res.ok) {
				article = await res.json();
			} else {
				if (res.status === 401 || res.status === 403) {
					localStorage.removeItem('admin_token');
					window.location.reload();
					return;
				}
				errorMessage = 'Failed to load article telemetry payload.';
			}
		} catch (err) {
			console.error('Fetch article preview failed:', err);
			errorMessage = 'Database node link offline.';
		} finally {
			isLoading = false;
		}
	}

	onMount(() => {
		fetchArticleDetails();
	});

	async function handlePublish() {
		const token = localStorage.getItem('admin_token');
		if (!token) return;

		try {
			const res = await fetch(`http://localhost:8080/api/admin/news/${data.id}/publish`, {
				method: 'POST',
				headers: {
					Authorization: `Bearer ${token}`
				}
			});

			if (res.ok) {
				goto('/admin/drafts');
			} else {
				alert('Failed to publish article.');
			}
		} catch (err) {
			console.error('Publish error:', err);
			alert('Network issue committing publish operation.');
		}
	}

	async function handleReject() {
		const token = localStorage.getItem('admin_token');
		if (!token) return;

		try {
			const res = await fetch(`http://localhost:8080/api/admin/news/${data.id}/reject`, {
				method: 'POST',
				headers: {
					Authorization: `Bearer ${token}`
				}
			});

			if (res.ok) {
				goto('/admin/drafts');
			} else {
				alert('Failed to reject article.');
			}
		} catch (err) {
			console.error('Reject error:', err);
			alert('Network issue committing reject operation.');
		}
	}

	function formatDate(dateStr: string) {
		return new Date(dateStr).toLocaleString('en-US', {
			year: 'numeric',
			month: 'long',
			day: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		});
	}

	function getReadingTime(text: string) {
		const words = text.split(/\s+/).length;
		const minutes = Math.ceil(words / 220);
		return `${minutes} min read`;
	}

	function goBack() {
		// Try to go back in browser history, fallback to drafts list
		if (window.history.length > 1) {
			window.history.back();
		} else {
			goto('/admin/drafts');
		}
	}
</script>

<svelte:head>
	<title>Article Preview // System Admin</title>
</svelte:head>

<section
	class="mx-auto flex w-full max-w-4xl flex-grow flex-col justify-start px-4 py-12 sm:px-6 md:py-16 lg:px-8"
>
	<!-- Back link -->
	<div class="mb-8">
		<button
			onclick={goBack}
			class="group inline-flex cursor-pointer items-center space-x-2 font-mono text-xs text-slate-400 transition-colors hover:text-[#22D3EE]"
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
			<span>RETURN TO PREVIOUS LEVEL</span>
		</button>
	</div>

	<!-- Loading -->
	{#if isLoading}
		<div class="flex flex-grow items-center justify-center py-20">
			<div class="space-y-4 text-center font-mono text-xs">
				<div
					class="mx-auto h-6 w-6 animate-spin rounded-full border-2 border-slate-800 border-t-[#22D3EE]"
				></div>
				<div class="animate-pulse tracking-widest text-slate-500 uppercase">
					Syncing Article Buffer...
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
	{:else if !article}
		<div
			class="my-12 rounded-xl border border-dashed border-[rgba(255,255,255,0.08)] bg-[#0F172A]/10 px-4 py-20 text-center"
		>
			<p class="mb-1 font-mono text-xs tracking-wider text-slate-500 uppercase">RECORD NOT FOUND</p>
		</div>
	{:else}
		<!-- Preview Controls Bar -->
		<div
			class="mb-8 flex flex-col items-center justify-between gap-4 rounded-xl border border-[rgba(255,255,255,0.08)] bg-[#0F172A]/40 p-4 sm:flex-row"
		>
			<div class="font-mono text-xs text-slate-400">
				STATUS: <span
					class="font-bold uppercase
					{article.status === 'published' ? 'text-cyan-400' : ''}
					{article.status === 'draft' ? 'text-amber-500' : ''}
					{article.status === 'rejected' ? 'text-[#E11D48]' : ''}
				">{article.status}</span
				>
				<span class="mx-2">|</span>
				ID: <span>{article.id}</span>
			</div>

			<div class="flex w-full gap-3 sm:w-auto">
				{#if article.status !== 'published'}
					<button
						onclick={handlePublish}
						class="flex-1 cursor-pointer rounded-xl border border-cyan-500 bg-cyan-950/20 px-4 py-2 font-mono text-xs font-semibold text-[#22D3EE] transition-all hover:bg-cyan-500 hover:text-[#0A0E17] sm:flex-none"
					>
						PUBLISH_ARTICLE()
					</button>
				{/if}
				{#if article.status !== 'rejected'}
					<button
						onclick={handleReject}
						class="flex-1 cursor-pointer rounded-xl border border-amber-500/50 bg-amber-950/10 px-4 py-2 font-mono text-xs font-semibold text-amber-500 transition-all hover:bg-amber-500 hover:text-[#0A0E17] sm:flex-none"
					>
						REJECT_ARTICLE()
					</button>
				{/if}
			</div>
		</div>

		<!-- Article Preview View -->
		<div class="rounded-2xl border border-[rgba(255,255,255,0.08)] bg-[#070A10]/40 p-6 md:p-8">
			<!-- Header -->
			<div class="mb-6 flex flex-wrap items-center gap-3 font-mono text-xs text-slate-500">
				<span
					class="rounded border border-[#22D3EE]/20 bg-[#22D3EE]/5 px-2 py-0.5 font-bold text-[#22D3EE] uppercase"
				>
					{article.category}
				</span>
				<span>•</span>
				<span>{formatDate(article.created_at)}</span>
				<span>•</span>
				<span>{getReadingTime(article.content)}</span>
			</div>

			<h1
				class="mb-4 font-serif text-2xl leading-tight font-medium text-white sm:text-3xl md:text-4xl"
			>
				{article.title}
			</h1>

			<p
				class="mb-8 border-b border-[rgba(255,255,255,0.08)] pb-8 font-sans text-sm leading-relaxed font-light text-slate-300 md:text-base"
			>
				{article.summary}
			</p>

			<!-- Image -->
			<div
				class="relative mb-8 aspect-[16/9] w-full overflow-hidden rounded-xl border border-[rgba(255,255,255,0.08)] bg-[#0F172A]"
			>
				<Image
					src={article.image_url}
					content={article.content}
					alt={article.title}
					class="h-full w-full object-cover"
				/>
			</div>

			<!-- Content -->
			<div
				class="article-content max-w-none font-sans text-sm leading-relaxed font-light text-slate-300 md:text-base"
			>
				{@html article.content}
			</div>
		</div>
	{/if}
</section>

<style>
	/* Nest editorial styling */
	:global(.article-content p) {
		margin-bottom: 1.5rem;
		line-height: 1.8;
		font-weight: 300;
		color: #cbd5e1;
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
</style>
