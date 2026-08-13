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
	// AI category
	{name: "OpenAI Blog", url: "https://openai.com/blog/rss.xml", category: "ai"},
	{name: "Google AI Blog", url: "https://blog.google/technology/ai/rss/", category: "ai"},
	{name: "Anthropic Blog", url: "https://www.anthropic.com/feed", category: "ai"},
	{name: "Meta AI Blog", url: "https://ai.meta.com/blog/rss/", category: "ai"},
	{name: "DeepMind Blog", url: "https://deepmind.google/blog/rss.xml", category: "ai"},
	// Tools category
	{name: "Hugging Face Blog", url: "https://huggingface.co/blog/feed.xml", category: "tools"},
	{name: "AWS Machine Learning", url: "https://aws.amazon.com/blogs/machine-learning/feed/", category: "tools"},
	{name: "GitHub Blog", url: "https://github.blog/feed/", category: "tools"},
	// Research category
	{name: "MIT AI News", url: "https://news.mit.edu/topic/mitartificial-intelligence2-rss.xml", category: "research"},
	{name: "arXiv AI", url: "https://rss.arxiv.org/rss/cs.AI", category: "research"},
	// Industry category
	{name: "TechCrunch AI", url: "https://techcrunch.com/category/artificial-intelligence/feed/", category: "industry"},
	{name: "VentureBeat AI", url: "https://venturebeat.com/category/ai/feed/", category: "industry"},
	{name: "The Verge AI", url: "https://www.theverge.com/rss/ai-artificial-intelligence/index.xml", category: "industry"},
	// Machine Learning category
	{name: "Machine Learning Mastery", url: "https://machinelearningmastery.com/feed/", category: "machine-learning"},
}

// defaultSettings seeds the admin-configurable scoring thresholds on first
// boot. Defaults: LOW <60, MEDIUM 60-79, HIGH >=80.
var defaultSettings = map[string]string{
	"score_low_max":    "59",
	"score_medium_min": "60",
	"score_medium_max": "79",
	"score_high_min":   "80",
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

	for k, v := range defaultSettings {
		if _, err := db.Exec(
			`INSERT OR IGNORE INTO app_settings (key, value) VALUES (?, ?)`,
			k, v,
		); err != nil {
			return err
		}
	}
	return nil
}
