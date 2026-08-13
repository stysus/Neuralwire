import { env } from '$env/dynamic/public';

/**
 * Absolute site URL (no trailing slash). Falls back to the local dev origin
 * when PUBLIC_SITE_URL is not configured.
 */
export function getSiteUrl(): string {
	const raw = env.PUBLIC_SITE_URL;
	if (raw && raw.trim().length > 0) {
		return raw.trim().replace(/\/+$/, '');
	}
	return 'http://localhost:5173';
}

/**
 * Resolve a possibly-relative URL (e.g. `/images/cover.jpg`) to an absolute
 * URL. Returns the favicon as the fallback image when none is provided.
 */
export function absoluteUrl(url?: string | null): string {
	const site = getSiteUrl();
	if (!url || url.trim().length === 0) {
		return `${site}/favicon.svg`;
	}
	if (/^https?:\/\//i.test(url)) {
		return url;
	}
	return `${site}${url.startsWith('/') ? url : `/${url}`}`;
}
