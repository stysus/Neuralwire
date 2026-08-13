import { getSiteUrl } from '$lib/siteUrl';

export const prerender = false;

export async function GET(): Promise<Response> {
	const body = `# allow crawling everywhere except the admin panel\nUser-agent: *\nDisallow: /admin\n\nSitemap: ${getSiteUrl()}/sitemap.xml\n`;
	return new Response(body, {
		headers: {
			'content-type': 'text/plain; charset=utf-8',
			'cache-control': 'public, max-age=3600'
		}
	});
}
