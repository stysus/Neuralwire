import { getNewsBySlug, getNews } from '$lib/api';
import type { News } from '$lib/mockData';
import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, params }) => {
	const { slug } = params;
	const article = await getNewsBySlug(slug, fetch);

	if (!article) {
		error(404, {
			message: `Article '${slug}' not found`
		});
	}

	let related: News[] = [];
	try {
		const res = await fetch(`http://localhost:8080/api/news/${article.id}/related?limit=12`, {
			signal: AbortSignal.timeout(2000)
		});
		if (res.ok) {
			const json = await res.json();
			related = json && json.data ? json.data : [];
		}
	} catch (e) {
		console.warn('Failed to fetch TF-IDF related articles, falling back to category fallback', e);
	}

	// Fallback to category articles if API failed or returned empty
	if (related.length === 0) {
		const categoryArticles = await getNews(fetch, article.category);
		related = categoryArticles.filter((item) => item.id !== article.id).slice(0, 12);
	}

	return {
		article,
		related
	};
};
