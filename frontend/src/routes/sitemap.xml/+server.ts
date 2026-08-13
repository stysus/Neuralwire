import { getSiteUrl } from '$lib/siteUrl';
import { BASE_URL } from '$lib/api';
import type { News } from '$lib/mockData';

interface SitemapEntry {
	loc: string;
	lastmod?: string;
	changefreq: string;
	priority: string;
}

function escapeXml(value: string): string {
	return value
		.replace(/&/g, '&amp;')
		.replace(/</g, '&lt;')
		.replace(/>/g, '&gt;')
		.replace(/"/g, '&quot;')
		.replace(/'/g, '&apos;');
}

function toDateIso(value?: string | null): string | undefined {
	if (!value) return undefined;
	const iso = new Date(value).toISOString();
	return iso ? iso.slice(0, 10) : undefined;
}

function render(urls: SitemapEntry[]): string {
	const body = urls
		.map((entry) => {
			const lastmod = entry.lastmod ? `<lastmod>${escapeXml(entry.lastmod)}</lastmod>` : '';
			return `\t<url>\n\t\t<loc>${escapeXml(entry.loc)}</loc>${lastmod}\n\t\t<changefreq>${entry.changefreq}</changefreq>\n\t\t<priority>${entry.priority}</priority>\n\t</url>`;
		})
		.join('\n');

	return `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${body}\n</urlset>\n`;
}

export const prerender = false;

export async function GET(): Promise<Response> {
	const site = getSiteUrl();
	const today = new Date().toISOString().slice(0, 10);
	const urls: SitemapEntry[] = [
		{ loc: `${site}/`, lastmod: today, changefreq: 'daily', priority: '1.0' },
		{ loc: `${site}/about`, changefreq: 'monthly', priority: '0.5' },
		{ loc: `${site}/search`, changefreq: 'weekly', priority: '0.4' }
	];

	// Categories
	try {
		const res = await fetch(`${BASE_URL}/categories`, { signal: AbortSignal.timeout(5000) });
		if (res.ok) {
			const data = await res.json();
			const categories = data && data.data ? data.data : [];
			for (const category of categories) {
				urls.push({
					loc: `${site}/category/${category.slug}`,
					changefreq: 'daily',
					priority: '0.6'
				});
			}
		}
	} catch {
		// Categories unreachable: still emit the static pages.
	}

	// Published articles
	try {
		const res = await fetch(`${BASE_URL}/news?page_size=100`, {
			signal: AbortSignal.timeout(5000)
		});
		if (res.ok) {
			const data = await res.json();
			const news: News[] = data && data.data ? data.data : [];
			for (const article of news.filter((item) => item.status === 'published')) {
				urls.push({
					loc: `${site}/${article.slug}`,
					lastmod: toDateIso(article.published_at || article.created_at),
					changefreq: 'weekly',
					priority: '0.8'
				});
			}
		}
	} catch {
		// Articles unreachable: still emit categories and static pages.
	}

	return new Response(render(urls), {
		headers: {
			'content-type': 'application/xml; charset=utf-8',
			'cache-control': 'public, max-age=3600'
		}
	});
}
