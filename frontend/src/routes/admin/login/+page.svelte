<script lang="ts">
	import { goto } from '$app/navigation';
	import { BASE_URL } from '$lib/api';

	let username = $state('');
	let password = $state('');
	let isLoading = $state(false);
	let errorMessage = $state('');

	async function handleLogin(e: Event) {
		e.preventDefault();
		isLoading = true;
		errorMessage = '';

		try {
			const res = await fetch(`${BASE_URL}/admin/login`, {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json'
				},
				body: JSON.stringify({
					username: username.trim(),
					password: password
				})
			});

			const data = await res.json();

			if (res.ok && data.token) {
				localStorage.setItem('admin_token', data.token);
				goto('/admin');
			} else {
				errorMessage = data.error || 'Authentication handshake rejected.';
			}
		} catch (err) {
			console.error('Login connection error:', err);
			errorMessage = 'Authentication node offline or unreachable.';
		} finally {
			isLoading = false;
		}
	}
</script>

<svelte:head>
	<title>Auth Handshake | Neuralwire</title>
</svelte:head>

<div class="flex flex-grow items-center justify-center px-4 py-20">
	<!-- Background glow -->
	<div
		class="pulse-glow-bg pointer-events-none absolute h-[300px] w-[300px] rounded-full bg-[#22D3EE]/5 blur-[100px]"
	></div>

	<div
		class="glow-hover relative w-full max-w-md overflow-hidden rounded-2xl border border-[rgba(255,255,255,0.08)] bg-[#0F172A]/30 p-6 backdrop-blur-sm sm:p-8"
	>
		<!-- Scan line indicator -->
		<div
			class="absolute top-0 left-0 h-[1px] w-full bg-gradient-to-r from-transparent via-[#22D3EE]/60 to-transparent"
		></div>

		<!-- Branding & Title -->
		<div class="mb-8 text-center">
			<div class="mb-3 inline-flex items-center space-x-2">
				<span
					class="rounded border border-[#22D3EE]/30 bg-[#22D3EE]/10 px-1 py-0.5 font-mono text-[9px] font-bold tracking-wider text-[#22D3EE] uppercase"
					>sysop</span
				>
				<span class="font-sans text-sm font-bold tracking-tight text-white uppercase">
					Neural<span class="text-[#22D3EE]">wire</span>
				</span>
			</div>
			<h2 class="font-serif text-lg font-normal tracking-tight text-white uppercase">
				SECURITY HANDSHAKE
			</h2>
			<p class="mt-1 font-mono text-[10px] tracking-wider text-slate-500 uppercase">
				Awaiting Credentials Verification...
			</p>
		</div>

		<!-- Error Feedback -->
		{#if errorMessage}
			<div
				class="mb-6 animate-pulse rounded-lg border border-[#E11D48]/30 bg-[#E11D48]/5 p-3 text-center font-mono text-[11px] text-[#E11D48]"
			>
				Error: {errorMessage}
			</div>
		{/if}

		<!-- Login Form -->
		<form onsubmit={handleLogin} class="space-y-5 font-mono text-xs">
			<div>
				<label for="username" class="mb-1.5 block tracking-widest text-slate-400 uppercase"
					>Username</label
				>
				<input
					type="text"
					id="username"
					placeholder="Enter username..."
					bind:value={username}
					required
					disabled={isLoading}
					class="w-full rounded-xl border border-[rgba(255,255,255,0.08)] bg-[#070A10] px-3.5 py-2.5 text-slate-100 placeholder-slate-600 transition-all focus:border-[#22D3EE]/50 focus:outline-none"
				/>
			</div>

			<div>
				<label for="password" class="mb-1.5 block tracking-widest text-slate-400 uppercase"
					>Password</label
				>
				<input
					type="password"
					id="password"
					placeholder="••••••••"
					bind:value={password}
					required
					disabled={isLoading}
					class="w-full rounded-xl border border-[rgba(255,255,255,0.08)] bg-[#070A10] px-3.5 py-2.5 text-slate-100 placeholder-slate-600 transition-all focus:border-[#22D3EE]/50 focus:outline-none"
				/>
			</div>

			<div class="pt-2">
				<button
					type="submit"
					disabled={isLoading}
					class="accent-glow-glow flex w-full cursor-pointer items-center justify-center space-x-2 rounded-xl bg-[#22D3EE] py-3 font-bold tracking-widest text-[#0A0E17] uppercase transition-all duration-300 hover:bg-[#22D3EE]/95 disabled:bg-slate-800 disabled:text-slate-500"
				>
					{#if isLoading}
						<div
							class="h-4 w-4 animate-spin rounded-full border-2 border-[#0A0E17] border-t-transparent"
						></div>
						<span>Connecting...</span>
					{:else}
						<span>Login</span>
					{/if}
				</button>
			</div>
		</form>
	</div>
</div>
