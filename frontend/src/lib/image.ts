/**
 * Extracts the first absolute <img> src URL from an HTML content string.
 * Supports http:// and https:// URLs.
 */
export function extractFirstImage(html: string): string | null {
	if (!html) return null;
	// Matches <img ... src="http(s)://..." ...>
	const regex = /<img\s+[^>]*src=["'](https?:\/\/[^"']+)["']/i;
	const match = html.match(regex);
	return match ? match[1] : null;
}
