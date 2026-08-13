export interface News {
	id: number;
	title: string;
	slug: string;
	url: string;
	source: string;
	category: string; // matches Category slug
	summary: string;
	content: string;
	image_url: string;
	view_count: number;
	status: 'draft' | 'published' | 'rejected';
	published_at: string;
	created_at: string;
	// Advisory news-value scoring (AI + heuristic weighted). Scoring never
	// auto-publishes; admins remain the final decision makers.
	value_score?: number;
	value_breakdown?: string;
	value_confidence?: number;
	value_recommendation?: string;
	value_reason?: string;
	value_label?: 'HIGH' | 'MEDIUM' | 'LOW' | '';
	value_method?: string;
}

export interface ScoreThresholds {
	low_max: number;
	medium_min: number;
	medium_max: number;
	high_min: number;
}

export interface Category {
	id: number;
	name: string;
	slug: string;
	created_at: string;
}

export const mockCategories: Category[] = [
	{ id: 1, name: 'Neural Networks', slug: 'neural-nets', created_at: '2026-01-01T00:00:00Z' },
	{ id: 2, name: 'Quantum Computing', slug: 'quantum', created_at: '2026-01-01T00:00:00Z' },
	{ id: 3, name: 'AI Ethics & Law', slug: 'ethics', created_at: '2026-01-01T00:00:00Z' },
	{ id: 4, name: 'Cybernetics', slug: 'cybernetics', created_at: '2026-01-01T00:00:00Z' },
	{ id: 5, name: 'Future Society', slug: 'future-society', created_at: '2026-01-01T00:00:00Z' }
];
