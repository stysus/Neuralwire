import { getSiteUrl } from '$lib/siteUrl';

// Prerendered at build time so the backend can serve it as a static file in
// production (the SPA fallback would otherwise return index.html for it).
export const prerender = true;

export async function GET(): Promise<Response> {
	const body = `# allow crawling everywhere except the admin panel\nUser-agent: *\nDisallow: /admin\n\nSitemap: ${getSiteUrl()}/sitemap.xml\n`;
	return new Response(body, {
		headers: {
			'content-type': 'text/plain; charset=utf-8',
			'cache-control': 'public, max-age=3600'
		}
	});
}
