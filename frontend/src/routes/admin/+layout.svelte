<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	let { children } = $props();

	let isAuthenticated = $state(false);
	let isCheckingAuth = $state(true);

	// Check if page is login page
	const isLoginPage = $derived($page.url.pathname === '/admin/login');

	function checkAuth() {
		const token = localStorage.getItem('admin_token');
		if (!token) {
			isAuthenticated = false;
			if (!isLoginPage) {
				goto('/admin/login');
			}
		} else {
			isAuthenticated = true;
			if (isLoginPage) {
				goto('/admin');
			}
		}
		isCheckingAuth = false;
	}

	onMount(() => {
		checkAuth();
	});

	// Reactively check auth state on path change
	$effect(() => {
		// Depend on pathname changes
		const path = $page.url.pathname;
		if (!isCheckingAuth) {
			const token = localStorage.getItem('admin_token');
			if (!token && path !== '/admin/login') {
				goto('/admin/login');
			} else if (token && path === '/admin/login') {
				goto('/admin');
			}
		}
	});

	function handleLogout() {
		localStorage.removeItem('admin_token');
		isAuthenticated = false;
		goto('/admin/login');
	}
</script>

<svelte:head>
	<title>System Admin | Neuralwire</title>
</svelte:head>

<div class="bg-grid-pattern relative flex min-h-screen flex-col bg-[#0A0E17] text-slate-200">
	<!-- Header / Admin Nav -->
	{#if isAuthenticated && !isLoginPage}
		<header
			class="sticky top-0 z-40 border-b border-[rgba(255,255,255,0.08)] bg-[#070A10]/95 backdrop-blur-md"
		>
			<div class="mx-auto flex h-14 max-w-7xl items-center justify-between px-4 sm:px-6 lg:px-8">
				<!-- Brand -->
				<div class="flex items-center space-x-3">
					<a href="/admin" class="flex items-center space-x-2">
						<span
							class="rounded border border-[#22D3EE]/30 bg-[#22D3EE]/5 px-1 py-0.5 font-mono text-[10px] font-bold tracking-wider text-[#22D3EE] uppercase"
							>sysop</span
						>
						<span class="font-sans text-sm font-bold tracking-tight uppercase">
							Neural<span class="text-[#22D3EE]">wire</span>
						</span>
					</a>
					<span class="font-mono text-slate-600">/</span>
					<span class="font-mono text-xs tracking-wider text-slate-400">ADMIN_PANEL_v2.0</span>
				</div>

				<!-- Nav Links -->
				<nav class="hidden items-center space-x-1 font-mono text-xs md:flex">
					<a
						href="/admin"
						class="rounded px-3 py-1.5 transition-colors {$page.url.pathname === '/admin'
							? 'bg-[#22D3EE]/5 text-[#22D3EE]'
							: 'text-slate-400 hover:text-white'}"
					>
						[DASHBOARD]
					</a>
					<a
						href="/admin/drafts"
						class="rounded px-3 py-1.5 transition-colors {$page.url.pathname === '/admin/drafts'
							? 'bg-[#22D3EE]/5 text-[#22D3EE]'
							: 'text-slate-400 hover:text-white'}"
					>
						[DRAFTS]
					</a>
					<a
						href="/admin/published"
						class="rounded px-3 py-1.5 transition-colors {$page.url.pathname === '/admin/published'
							? 'bg-[#22D3EE]/5 text-[#22D3EE]'
							: 'text-slate-400 hover:text-white'}"
					>
						[PUBLISHED]
					</a>
					<a
						href="/admin/rejected"
						class="rounded px-3 py-1.5 transition-colors {$page.url.pathname === '/admin/rejected'
							? 'bg-[#22D3EE]/5 text-[#22D3EE]'
							: 'text-slate-400 hover:text-white'}"
					>
						[REJECTED]
					</a>
				</nav>

				<!-- Exit Session -->
				<div class="flex items-center space-x-4">
					<a
						href="/"
						class="hidden font-mono text-[10px] text-slate-500 hover:text-slate-300 sm:inline-block"
					>
						PUBLIC_SITE →
					</a>
					<button
						onclick={handleLogout}
						class="cursor-pointer rounded border border-[#E11D48]/30 bg-[#E11D48]/5 px-2.5 py-1 font-mono text-xs text-[#E11D48] transition-all hover:border-[#E11D48] hover:bg-[#E11D48]/10 hover:text-white"
					>
						[EXIT_SYS]
					</button>
				</div>
			</div>

			<!-- Mobile Header Nav Row -->
			<div
				class="flex items-center justify-around border-t border-[rgba(255,255,255,0.05)] bg-[#070A10]/50 py-2 font-mono text-[10px] md:hidden"
			>
				<a
					href="/admin"
					class={$page.url.pathname === '/admin' ? 'text-[#22D3EE]' : 'text-slate-400'}>DASH</a
				>
				<a
					href="/admin/drafts"
					class={$page.url.pathname === '/admin/drafts' ? 'text-[#22D3EE]' : 'text-slate-400'}
					>DRAFTS</a
				>
				<a
					href="/admin/published"
					class={$page.url.pathname === '/admin/published' ? 'text-[#22D3EE]' : 'text-slate-400'}
					>PUB</a
				>
				<a
					href="/admin/rejected"
					class={$page.url.pathname === '/admin/rejected' ? 'text-[#22D3EE]' : 'text-slate-400'}
					>REJ</a
				>
			</div>
		</header>
	{/if}

	<!-- Main Admin Portal Workspace -->
	<main class="relative z-10 flex w-full flex-grow flex-col">
		{#if isCheckingAuth}
			<div class="flex flex-grow items-center justify-center py-24">
				<div class="space-y-4 text-center font-mono text-xs">
					<div
						class="mx-auto h-8 w-8 animate-spin rounded-full border-2 border-slate-700 border-t-[#22D3EE]"
					></div>
					<div class="animate-pulse tracking-widest text-slate-500 uppercase">
						Initializing Auth Handshake...
					</div>
				</div>
			</div>
		{:else if isAuthenticated || isLoginPage}
			{@render children()}
		{/if}
	</main>
</div>
