import { env } from '$env/dynamic/public';
import { mockCategories, type Category, type News } from './mockData';

export const BASE_URL = (env.PUBLIC_API_URL?.trim() || '/api').replace(/\/+$/, '');

// ---------------------------------------------------------------------------
// Client-side API response cache (STY-96)
// ---------------------------------------------------------------------------
// Only successful responses (res.ok) are cached. Failed/fallback results are
// never stored, so the next call retries the network.

const API_CACHE_TTL = 60_000; // 60 s — tunable

interface CacheEntry {
	data: unknown;
	expiresAt: number;
}

const apiCache = new Map<string, CacheEntry>();

/** Fetch JSON, returning cached data when still valid. */
async function fetchJsonCached<T>(
	url: string,
	f: typeof fetch,
	signal?: AbortSignal
): Promise<T | null> {
	const now = Date.now();
	const cached = apiCache.get(url);
	if (cached && now < cached.expiresAt) return cached.data as T;

	try {
		const res = await f(url, { signal });
		if (res.ok) {
			const data = await res.json();
			apiCache.set(url, { data, expiresAt: now + API_CACHE_TTL });
			return data;
		}
	} catch {
		// network / timeout — fall through to return null
	}

	return null;
}

/**
 * Clear the API cache. If `pattern` is provided, only keys that include the
 * substring are removed; otherwise the entire cache is flushed.
 */
export function clearCache(pattern?: string) {
	if (!pattern) {
		apiCache.clear();
		return;
	}
	for (const key of [...apiCache.keys()]) {
		if (key.includes(pattern)) apiCache.delete(key);
	}
}

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
	const data = await fetchJsonCached<{ data: Category[] }>(
		`${BASE_URL}/categories`,
		f,
		AbortSignal.timeout(2000)
	);
	if (data?.data && data.data.length > 0) return data.data;
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

	// Build query string. Search uses the backend ?q= endpoint, optionally combined
	// with a category filter; otherwise fetch a large page to populate the feeds.
	const isSearch = searchQuery && searchQuery.trim().length > 0;
	let url = isSearch
		? `${BASE_URL}/news?q=${encodeURIComponent(searchQuery)}&page_size=20`
		: `${BASE_URL}/news?page_size=100`;
	if (categorySlug) {
		url += `&category=${encodeURIComponent(categorySlug)}`;
	}

	const result = await fetchJsonCached<{ data: News[] }>(url, f, AbortSignal.timeout(2000));
	const articles = result?.data ?? [];

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
