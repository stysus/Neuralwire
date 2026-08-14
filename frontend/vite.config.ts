import tailwindcss from '@tailwindcss/vite';
import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit({
			compilerOptions: {
				// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
				runes: ({ filename }) =>
					filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			},

			// adapter-static builds the app into static files served by the backend
			// in production. `fallback: "index.html"` keeps SPA routing working.
			adapter: adapter({
				pages: 'build',
				assets: 'build',
				fallback: 'index.html'
			})
		})
	]
});
