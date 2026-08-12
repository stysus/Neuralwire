package database

import "database/sql"

// defaultCategories is seeded on first boot.
var defaultCategories = []struct {
	name string
	slug string
}{
	{name: "AI", slug: "ai"},
	{name: "Machine Learning", slug: "machine-learning"},
	{name: "Research", slug: "research"},
	{name: "Tools", slug: "tools"},
	{name: "Industry", slug: "industry"},
}

// defaultRSSSources is seeded on first boot. The fetcher polls every
// enabled source and inserts articles as drafts.
var defaultRSSSources = []struct {
	name     string
	url      string
	category string
}{
	{name: "OpenAI Blog", url: "https://openai.com/blog/rss.xml", category: "ai"},
	{name: "Google AI Blog", url: "https://blog.google/technology/ai/rss/", category: "ai"},
	{name: "Hugging Face Blog", url: "https://huggingface.co/blog/feed.xml", category: "tools"},
	{name: "arXiv cs.AI", url: "https://rss.arxiv.org/rss/cs.AI", category: "research"},
	{name: "Reddit r/artificial", url: "https://www.reddit.com/r/artificial/.rss", category: "ai"},
}

// Seed inserts default categories and RSS sources when their tables are
// empty. It is idempotent and safe to run on every boot.
func Seed(db *sql.DB) error {
	for _, c := range defaultCategories {
		if _, err := db.Exec(
			`INSERT OR IGNORE INTO categories (name, slug) VALUES (?, ?)`,
			c.name, c.slug,
		); err != nil {
			return err
		}
	}

	for _, s := range defaultRSSSources {
		if _, err := db.Exec(
			`INSERT OR IGNORE INTO rss_sources (name, url, category) VALUES (?, ?, ?)`,
			s.name, s.url, s.category,
		); err != nil {
			return err
		}
	}
	return nil
}
