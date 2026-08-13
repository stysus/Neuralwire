import { mockCategories, type Category, type News } from './mockData';

export const BASE_URL = 'http://localhost:8080/api';

/**
 * Gets SvelteKit-compatible fetch or global fetch.
 */
function getFetch(customFetch?: typeof fetch): typeof fetch {
	return customFetch || fetch;
}

/**
 * Fetch all categories.
 */
export async function getCategories(customFetch?: typeof fetch): Promise<Category[]> {
	const f = getFetch(customFetch);
	try {
		const res = await f(`${BASE_URL}/categories`, { signal: AbortSignal.timeout(2000) });
		if (res.ok) {
			const data = await res.json();
			return data && data.data && data.data.length > 0 ? data.data : mockCategories;
		}
	} catch (e) {
		console.warn('Backend categories API unreachable, using mock categories.', e);
	}
	return mockCategories;
}

/**
 * Fetch all news, optional filtering by category or query.
 */
export async function getNews(
	customFetch?: typeof fetch,
	categorySlug?: string,
	searchQuery?: string
): Promise<News[]> {
	const f = getFetch(customFetch);
	let articles: News[] = [];

	// Build query string. Search uses the backend ?q= endpoint, optionally combined
	// with a category filter; otherwise fetch a large page to populate the feeds.
	const isSearch = searchQuery && searchQuery.trim().length > 0;
	let url = isSearch
		? `${BASE_URL}/news?q=${encodeURIComponent(searchQuery)}&page_size=20`
		: `${BASE_URL}/news?page_size=100`;
	if (categorySlug) {
		url += `&category=${encodeURIComponent(categorySlug)}`;
	}

	try {
		const res = await f(url, { signal: AbortSignal.timeout(2000) });
		if (res.ok) {
			const result = await res.json();
			articles = result && result.data ? result.data : [];
		}
	} catch (e) {
		console.warn('Backend news API unreachable.', e);
	}

	// Filter by published status (backend search already filters by category/query).
	return articles
		.filter((item) => item.status === 'published')
		.sort((a, b) => {
			const dateA = new Date(a.published_at || a.created_at).getTime();
			const dateB = new Date(b.published_at || b.created_at).getTime();
			return dateB - dateA;
		});
}

/**
 * Fetch a single news article by slug.
 */
export async function getNewsBySlug(
	slug: string,
	customFetch?: typeof fetch
): Promise<News | null> {
	const f = getFetch(customFetch);

	// First, fetch the list of news to find the ID corresponding to this slug.
	const articles = await getNews(customFetch);
	const matched = articles.find((item) => item.slug === slug);

	if (!matched) {
		return null;
	}

	// Now try to fetch the detailed article by ID from the backend API.
	try {
		const res = await f(`${BASE_URL}/news/${matched.id}`, { signal: AbortSignal.timeout(2000) });
		if (res.ok) {
			const detailed = await res.json();
			if (detailed) {
				return detailed;
			}
		}
	} catch (e) {
		console.warn(`Backend news detail API for ID ${matched.id} unreachable, using mock detail.`, e);
	}

	return matched;
}

/**
 * Helper to slugify a string.
 */
export function slugify(str: string): string {
	return str
		.toLowerCase()
		.trim()
		.replace(/[^\w\s-]/g, '')
		.replace(/[\s_-]+/g, '-')
		.replace(/^-+|-+$/g, '');
}
