import { getNewsBySlug, getNews } from '$lib/api';
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

	// Fetch articles in the same category to show as related recommendations
	const categoryArticles = await getNews(fetch, article.category);
	const related = categoryArticles.filter((item) => item.id !== article.id).slice(0, 3);

	return {
		article,
		related
	};
};
