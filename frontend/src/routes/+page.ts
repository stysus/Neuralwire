import { getNews } from '$lib/api';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
	const news = await getNews(fetch);
	return {
		news
	};
};
