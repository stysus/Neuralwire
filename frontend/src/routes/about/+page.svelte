<script lang="ts">
	import { getSiteUrl } from '$lib/siteUrl';
</script>

<svelte:head>
	<title>About | Neuralwire AI News & Editorial</title>
	<meta
		name="description"
		content="Neuralwire is a premier news syndicate and editorial chronicle investigating the emergence of supercomputing, quantum intelligence, and cybernetics."
	/>
	<meta name="robots" content="index, follow" />
	<link rel="canonical" href="{getSiteUrl()}/about" />
	<meta property="og:title" content="About | Neuralwire AI News & Editorial" />
	<meta
		property="og:description"
		content="Neuralwire is a premier news syndicate and editorial chronicle investigating the emergence of supercomputing, quantum intelligence, and cybernetics."
	/>
	<meta property="og:type" content="website" />
	<meta property="og:url" content="{getSiteUrl()}/about" />
	<meta property="og:image" content="{getSiteUrl()}/favicon.svg" />
	<meta name="twitter:card" content="summary_large_image" />
	<meta name="twitter:title" content="About | Neuralwire AI News & Editorial" />
	<meta
		name="twitter:description"
		content="Neuralwire is a premier news syndicate and editorial chronicle investigating the emergence of supercomputing, quantum intelligence, and cybernetics."
	/>
	<meta name="twitter:image" content="{getSiteUrl()}/favicon.svg" />
</svelte:head>

<section class="mx-auto max-w-4xl px-4 py-16 sm:px-6 md:py-24 lg:px-8">
	<!-- Heading Header -->
	<div class="mb-12 border-b border-[rgba(255,255,255,0.08)] pb-8">
		<span class="tag-mono mb-2 block text-xs font-bold tracking-widest text-[#22D3EE]"
			>MISSION & MANIFESTO</span
		>
		<h1 class="mb-4 font-serif text-4xl font-medium text-white md:text-5xl">
			BRIDGING CODE AND HUMANITY
		</h1>
		<p class="max-w-2xl font-sans text-sm leading-relaxed font-light text-slate-400 md:text-base">
			Neuralwire is an independent digital publication dedicated to chronicling the progress of
			machine intelligence. We report on the milestones of neural networks, the development of
			quantum systems, and the profound societal and ethical shifts occurring at the intersection of
			code and organic life.
		</p>
	</div>

	<!-- Core Values Grid -->
	<div class="mb-16 grid grid-cols-1 gap-8 md:grid-cols-2">
		<div class="glow-hover rounded-xl border border-[rgba(255,255,255,0.08)] bg-[#0F172A]/20 p-6">
			<span class="mb-2 block font-mono text-[10px] tracking-wider text-[#22D3EE] uppercase"
				>EDITORIAL RIGOR</span
			>
			<h2 class="mb-3 font-serif text-lg font-normal text-white">Objective Journalism</h2>
			<p class="font-sans text-xs leading-relaxed font-light text-slate-400">
				We investigate scientific publications and engineering breakthroughs with rigor. We cut
				through the commercial hype of modern marketing and write clear, objective, and deeply
				researched reports on actual mathematical advancements.
			</p>
		</div>

		<div class="glow-hover rounded-xl border border-[rgba(255,255,255,0.08)] bg-[#0F172A]/20 p-6">
			<span class="mb-2 block font-mono text-[10px] tracking-wider text-[#22D3EE] uppercase"
				>CYBERNETIC OUTLOOK</span
			>
			<h2 class="mb-3 font-serif text-lg font-normal text-white">Futurist Perspective</h2>
			<p class="font-sans text-xs leading-relaxed font-light text-slate-400">
				We believe the convergence of human neural structures and silicon processing is the defining
				transition of the 21st century. Our editorial focuses on where programming interfaces
				directly with human perception.
			</p>
		</div>

		<div class="glow-hover rounded-xl border border-[rgba(255,255,255,0.08)] bg-[#0F172A]/20 p-6">
			<span class="mb-2 block font-mono text-[10px] tracking-wider text-[#22D3EE] uppercase"
				>COMPUTE & PHYSICS</span
			>
			<h2 class="mb-3 font-serif text-lg font-normal text-white">Physical Foundations</h2>
			<p class="font-sans text-xs leading-relaxed font-light text-slate-400">
				From the thermodynamics of deep compute nodes to topological qubits and quantum coherence,
				we cover the underlying physical structures that enable tomorrow's computing frontiers.
			</p>
		</div>

		<div class="glow-hover rounded-xl border border-[rgba(255,255,255,0.08)] bg-[#0F172A]/20 p-6">
			<span class="mb-2 block font-mono text-[10px] tracking-wider text-[#22D3EE] uppercase"
				>ETHICAL VIGILANCE</span
			>
			<h2 class="mb-3 font-serif text-lg font-normal text-white">Human Protections</h2>
			<p class="font-sans text-xs leading-relaxed font-light text-slate-400">
				As digital persona emulation, generative systems, and algorithmic decisions replace organic
				structures, we hold synthetic systems accountable, ensuring human rights and privacy are
				preserved.
			</p>
		</div>
	</div>
</section>
