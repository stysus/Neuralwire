import { getNews } from '$lib/api';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, url }) => {
	const query = url.searchParams.get('q') || '';
	const news = await getNews(fetch, undefined, query);
	return {
		news,
		query
	};
};
