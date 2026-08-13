<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import type { PageData } from './$types';
	import Image from '$lib/Image.svelte';

	let { data }: { data: PageData } = $props();

	let article = $state<any>(null);
	let isLoading = $state(true);
	let errorMessage = $state('');

	// Edit states
	let isEditing = $state(false);
	let editTitle = $state('');
	let editCategory = $state('');
	let editSummary = $state('');
	let editContent = $state('');
	let editImageURL = $state('');
	let isSaving = $state(false);

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
				// Pre-populate edit states
				editTitle = article.title || '';
				editCategory = article.category || '';
				editSummary = article.summary || '';
				editContent = article.content || '';
				editImageURL = article.image_url || '';
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

	function addCopyButtons() {
		// Wait a tick for Svelte DOM to render
		setTimeout(() => {
			const container = document.querySelector('.article-content');
			if (!container) return;

			const preBlocks = container.querySelectorAll('pre');
			preBlocks.forEach((pre) => {
				// Avoid duplicate copy buttons
				if (pre.querySelector('.copy-btn')) return;

				pre.style.position = 'relative';

				const btn = document.createElement('button');
				btn.className =
					'copy-btn absolute top-2 right-2 px-2.5 py-1 font-mono text-[10px] bg-[#0A0E17] border border-[rgba(255,255,255,0.15)] text-slate-400 rounded hover:text-cyan-400 hover:border-cyan-400 transition-all cursor-pointer font-bold select-none';
				btn.innerText = 'COPY';
				btn.onclick = async (e) => {
					e.stopPropagation();
					const codeText = pre.querySelector('code')?.innerText || pre.innerText;
					try {
						let cleanText = codeText;
						if (cleanText.endsWith('COPY')) {
							cleanText = cleanText.slice(0, -4);
						} else if (cleanText.endsWith('COPIED!')) {
							cleanText = cleanText.slice(0, -7);
						}
						await navigator.clipboard.writeText(cleanText.trim());
						btn.innerText = 'COPIED!';
						btn.style.borderColor = '#22D3EE';
						btn.style.color = '#22D3EE';
						setTimeout(() => {
							btn.innerText = 'COPY';
							btn.style.borderColor = 'rgba(255,255,255,0.15)';
							btn.style.color = '#94a3b8';
						}, 2000);
					} catch (err) {
						console.error('Clipboard copy failed:', err);
						btn.innerText = 'ERROR';
					}
				};

				pre.appendChild(btn);
			});
		}, 100);
	}

	// Svelte 5 effect to attach copy buttons to pre blocks
	$effect(() => {
		if (article && !isEditing) {
			addCopyButtons();
		}
	});

	// WYSIWYG text format executor
	function formatText(command: string, value: string = '') {
		document.execCommand(command, false, value);
	}

	async function handleSave() {
		const token = localStorage.getItem('admin_token');
		if (!token) return;

		isSaving = true;

		try {
			const res = await fetch(`http://localhost:8080/api/admin/news/${data.id}`, {
				method: 'PUT',
				headers: {
					Authorization: `Bearer ${token}`,
					'Content-Type': 'application/json'
				},
				body: JSON.stringify({
					title: editTitle.trim(),
					category: editCategory.trim(),
					summary: editSummary.trim(),
					content: editContent, // Already formatted HTML from contenteditable
					image_url: editImageURL.trim()
				})
			});

			if (res.ok) {
				isEditing = false;
				await fetchArticleDetails();
			} else {
				const data = await res.json();
				alert(`Failed to save article changes: ${data.error || 'Unknown error'}`);
			}
		} catch (err) {
			console.error('Save article changes failed:', err);
			alert('Network issue committing article update.');
		} finally {
			isSaving = false;
		}
	}

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
		if (!text) return '0 min read';
		const words = text.replace(/<[^>]*>/g, ' ').split(/\s+/).length;
		const minutes = Math.ceil(words / 220);
		return `${minutes} min read`;
	}

	function goBack() {
		if (isEditing) {
			isEditing = false;
		} else if (window.history.length > 1) {
			window.history.back();
		} else {
			goto('/admin/drafts');
		}
	}
</script>

<svelte:head>
	<title
		>{isEditing ? 'Editing: ' + editTitle : article?.title || 'Article Preview'} | System Admin</title
	>
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
			<span>{isEditing ? 'CANCEL_EDIT()' : 'RETURN_TO_PREVIOUS_LEVEL()'}</span>
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
		<!-- Controls Bar -->
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
				{#if isEditing}
					<button
						onclick={handleSave}
						disabled={isSaving}
						class="flex-grow cursor-pointer rounded-xl border border-[#22D3EE] bg-cyan-950/20 px-6 py-2.5 font-mono text-xs font-semibold text-[#22D3EE] transition-all hover:bg-cyan-500 hover:text-[#0A0E17] disabled:opacity-50 sm:flex-none"
					>
						{isSaving ? 'SAVING_CHANGES...' : 'SAVE_CHANGES()'}
					</button>
				{:else}
					<button
						onclick={() => (isEditing = true)}
						class="flex-grow cursor-pointer rounded-xl border border-slate-500/50 bg-[#0F172A] px-4 py-2 font-mono text-xs font-semibold text-slate-300 transition-all hover:border-[#22D3EE] hover:text-[#22D3EE] sm:flex-none"
					>
						{article.status === 'draft' ? 'EDIT_DRAFT()' : 'EDIT_ARTICLE()'}
					</button>
					{#if article.status !== 'published'}
						<button
							onclick={handlePublish}
							class="flex-grow cursor-pointer rounded-xl border border-cyan-500 bg-cyan-950/20 px-4 py-2 font-mono text-xs font-semibold text-[#22D3EE] transition-all hover:bg-cyan-500 hover:text-[#0A0E17] sm:flex-none"
						>
							PUBLISH_ARTICLE()
						</button>
					{/if}
					{#if article.status !== 'rejected'}
						<button
							onclick={handleReject}
							class="flex-grow cursor-pointer rounded-xl border border-amber-500/50 bg-amber-950/10 px-4 py-2 font-mono text-xs font-semibold text-amber-500 transition-all hover:bg-amber-500 hover:text-[#0A0E17] sm:flex-none"
						>
							REJECT_ARTICLE()
						</button>
					{/if}
				{/if}
			</div>
		</div>

		<!-- Rich Text Editor formatting toolbar -->
		{#if isEditing}
			<div
				class="sticky top-4 z-10 mb-6 flex flex-wrap items-center gap-1 rounded-xl border border-[rgba(255,255,255,0.08)] bg-[#0b0f17]/95 p-2 shadow-2xl backdrop-blur"
			>
				<span class="px-2 font-mono text-[10px] font-bold text-[#22D3EE] uppercase select-none"
					>WYSIWYG Toolbar:</span
				>
				<button
					onclick={() => formatText('bold')}
					class="cursor-pointer rounded px-3 py-1 font-sans text-xs font-bold text-slate-300 transition-colors hover:bg-slate-800 hover:text-white"
					title="Bold">B</button
				>
				<button
					onclick={() => formatText('italic')}
					class="cursor-pointer rounded px-3 py-1 font-sans text-xs text-slate-300 italic transition-colors hover:bg-slate-800 hover:text-white"
					title="Italic">I</button
				>
				<button
					onclick={() => formatText('underline')}
					class="cursor-pointer rounded px-3 py-1 font-sans text-xs text-slate-300 underline transition-colors hover:bg-slate-800 hover:text-white"
					title="Underline">U</button
				>
				<span class="text-slate-700 select-none">|</span>
				<button
					onclick={() => formatText('formatBlock', 'H2')}
					class="cursor-pointer rounded px-3 py-1 font-sans text-xs font-semibold text-slate-300 transition-colors hover:bg-slate-800 hover:text-white"
					title="Heading 2">H2</button
				>
				<button
					onclick={() => formatText('formatBlock', 'H3')}
					class="cursor-pointer rounded px-3 py-1 font-sans text-xs font-semibold text-slate-300 transition-colors hover:bg-slate-800 hover:text-white"
					title="Heading 3">H3</button
				>
				<button
					onclick={() => formatText('formatBlock', 'p')}
					class="cursor-pointer rounded px-3 py-1 font-sans text-xs text-slate-300 transition-colors hover:bg-slate-800 hover:text-white"
					title="Paragraph">P</button
				>
				<span class="text-slate-700 select-none">|</span>
				<button
					onclick={() => formatText('insertUnorderedList')}
					class="cursor-pointer rounded px-3 py-1 font-sans text-xs text-slate-300 transition-colors hover:bg-slate-800 hover:text-white"
					title="Bullet List">&bull; List</button
				>
				<button
					onclick={() => formatText('formatBlock', 'blockquote')}
					class="cursor-pointer rounded px-3 py-1 font-sans text-xs text-slate-300 italic transition-colors hover:bg-slate-800 hover:text-white"
					title="Blockquote">&ldquo; Quote</button
				>
				<button
					onclick={() => formatText('formatBlock', 'pre')}
					class="cursor-pointer rounded px-3 py-1 font-mono text-xs text-slate-300 transition-colors hover:bg-slate-800 hover:text-white"
					title="Code Block">``` Code</button
				>
			</div>
		{/if}

		<!-- WYSIWYG In-Place Article View -->
		<div class="rounded-2xl border border-[rgba(255,255,255,0.08)] bg-[#070A10]/40 p-6 md:p-8">
			<!-- Header -->
			<div class="mb-6 flex flex-wrap items-center gap-3 font-mono text-xs text-slate-500">
				{#if isEditing}
					<span
						contenteditable="true"
						bind:textContent={editCategory}
						class="editable-field rounded border border-[#22D3EE]/20 bg-[#22D3EE]/5 px-2 py-0.5 font-bold text-[#22D3EE] uppercase outline-none"
					></span>
				{:else}
					<span
						class="rounded border border-[#22D3EE]/20 bg-[#22D3EE]/5 px-2 py-0.5 font-bold text-[#22D3EE] uppercase"
					>
						{article.category}
					</span>
				{/if}
				<span>•</span>
				<span>{formatDate(article.created_at)}</span>
				<span>•</span>
				<span>{getReadingTime(editContent)}</span>
			</div>

			<!-- Title -->
			{#if isEditing}
				<h1
					contenteditable="true"
					bind:textContent={editTitle}
					class="editable-field mb-4 font-serif text-2xl leading-tight font-medium text-white outline-none sm:text-3xl md:text-4xl"
				></h1>
			{:else}
				<h1
					class="mb-4 font-serif text-2xl leading-tight font-medium text-white sm:text-3xl md:text-4xl"
				>
					{article.title}
				</h1>
			{/if}

			<!-- Summary -->
			{#if isEditing}
				<p
					contenteditable="true"
					bind:textContent={editSummary}
					class="editable-field mb-8 border-b border-[rgba(255,255,255,0.08)] pb-8 font-sans text-sm leading-relaxed font-light text-slate-300 outline-none md:text-base"
				></p>
			{:else}
				<p
					class="mb-8 border-b border-[rgba(255,255,255,0.08)] pb-8 font-sans text-sm leading-relaxed font-light text-slate-300 md:text-base"
				>
					{article.summary}
				</p>
			{/if}

			<!-- Image URL editor in-place -->
			{#if isEditing}
				<div class="mb-6 rounded-xl border border-[rgba(255,255,255,0.08)] bg-[#0F172A]/40 p-4">
					<label class="mb-2 block font-mono text-[10px] text-slate-400">COVER IMAGE URL</label>
					<input
						type="text"
						bind:value={editImageURL}
						class="mb-3 w-full rounded-lg border border-[rgba(255,255,255,0.08)] bg-[#0F172A]/60 px-3 py-2 font-sans text-xs text-white transition-colors focus:border-[#22D3EE] focus:outline-none"
						placeholder="Paste cover image link..."
					/>
					<div class="relative aspect-[16/9] w-full overflow-hidden rounded-lg bg-[#0F172A]">
						<img
							src={editImageURL}
							alt="Cover preview"
							class="h-full w-full object-cover"
							onerror={(e) => {
								(e.target as HTMLImageElement).style.opacity = '0';
							}}
						/>
					</div>
				</div>
			{:else}
				<!-- Image Display -->
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
			{/if}

			<!-- Content (Visual editor) -->
			{#if isEditing}
				<div
					contenteditable="true"
					bind:innerHTML={editContent}
					class="article-content editable-field min-h-[300px] max-w-none font-sans text-sm leading-relaxed font-light text-slate-300 outline-none md:text-base"
				></div>
			{:else}
				<div
					class="article-content max-w-none font-sans text-sm leading-relaxed font-light text-slate-300 md:text-base"
				>
					{#if article.content}
						{@html article.content}
					{:else}
						<div
							class="rounded-lg border border-[#22D3EE]/20 bg-[#22D3EE]/5 p-4 font-mono text-xs text-[#22D3EE]"
						>
							Curator Mode: The original full text is not stored. The AI digest below is the
							published brief.
						</div>
						<p class="mt-4 text-slate-200">{article.summary || 'No summary available.'}</p>
					{/if}
				</div>
			{/if}
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
		border-top-left-radius: 0;
		border-bottom-left-radius: 0;
	}

	:global(.article-content pre) {
		background-color: #0b0f17;
		border: 1px solid rgba(255, 255, 255, 0.08);
		border-radius: 0.75rem;
		padding: 1.25rem;
		margin: 1.5rem 0;
		overflow-x: auto;
		position: relative;
	}

	:global(.article-content code) {
		font-family: 'Fira Code', Consolas, Monaco, 'Andale Mono', 'Ubuntu Mono', monospace;
		font-size: 0.875rem;
		color: #e2e8f0;
	}

	:global(.article-content p code, .article-content li code) {
		background-color: rgba(255, 255, 255, 0.06);
		padding: 0.2rem 0.4rem;
		border-radius: 0.25rem;
		font-size: 0.875rem;
		border: 1px solid rgba(255, 255, 255, 0.04);
	}

	/* Copy button CSS with hover states */
	:global(.copy-btn) {
		opacity: 0;
	}

	:global(pre:hover .copy-btn) {
		opacity: 1;
	}

	/* WYSIWYG Editable Visual Indicators */
	.editable-field {
		border: 1px dashed transparent;
		transition:
			border-color 0.2s ease,
			background-color 0.2s ease;
		padding: 0.25rem;
		border-radius: 0.5rem;
	}

	.editable-field:hover {
		border-color: rgba(34, 211, 238, 0.25);
		background-color: rgba(255, 255, 255, 0.01);
	}

	.editable-field:focus {
		border-color: #22d3ee;
		background-color: rgba(34, 211, 238, 0.03);
		box-shadow: 0 0 10px rgba(34, 211, 238, 0.1);
	}
</style>
