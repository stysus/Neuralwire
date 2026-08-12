import { getCategories } from '$lib/api';
import type { LayoutLoad } from './$types';

export const load: LayoutLoad = async ({ fetch }) => {
	const categories = await getCategories(fetch);
	return {
		categories
	};
};
