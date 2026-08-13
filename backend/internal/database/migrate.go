package database

import (
	"database/sql"
	"fmt"
)

// schema defines every table used by Neuralwire.
const schema = `
CREATE TABLE IF NOT EXISTS news (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    title        TEXT    NOT NULL,
    slug         TEXT    NOT NULL UNIQUE,
    url          TEXT    NOT NULL,
    source       TEXT    NOT NULL DEFAULT '',
    category     TEXT    NOT NULL DEFAULT 'ai',
    summary      TEXT    NOT NULL DEFAULT '',
    content      TEXT    NOT NULL DEFAULT '',
    image_url    TEXT    NOT NULL DEFAULT '',
    status       TEXT    NOT NULL DEFAULT 'draft'
                 CHECK (status IN ('draft','published','rejected')),
    published_at DATETIME,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_news_status_published_at
    ON news (status, published_at DESC);

CREATE INDEX IF NOT EXISTS idx_news_category_status
    ON news (category, status);

-- Public list with category filter sorts by published_at; include it in the
-- composite so SQLite can satisfy the ORDER BY from the index.
CREATE INDEX IF NOT EXISTS idx_news_category_status_published
    ON news (category, status, published_at DESC);

-- Admin list orders by created_at; a dedicated index avoids a full scan and
-- temp sort even when filtering by status.
CREATE INDEX IF NOT EXISTS idx_news_created_at
    ON news (created_at DESC);

-- Admin list with status filter orders by created_at DESC; this composite
-- lets SQLite satisfy both the filter and the sort from one index.
CREATE INDEX IF NOT EXISTS idx_news_status_created_at
    ON news (status, created_at DESC);

-- ExistsByURL runs once per feed item during every fetch cycle; without an
-- index this is a full table scan on each call.
CREATE INDEX IF NOT EXISTS idx_news_url
    ON news (url);

CREATE TABLE IF NOT EXISTS rss_sources (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT    NOT NULL,
    url             TEXT    NOT NULL UNIQUE,
    category        TEXT    NOT NULL DEFAULT 'ai',
    enabled         INTEGER NOT NULL DEFAULT 1,
    last_fetched_at DATETIME,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS categories (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL UNIQUE,
    slug       TEXT    NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS app_settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL DEFAULT '',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS article_views (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    news_id    INTEGER NOT NULL,
    viewer_key TEXT    NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_article_views_news
    ON article_views (news_id, created_at);

-- RecordView dedup looks up (news_id, viewer_key, created_at > cutoff);
-- the viewer_key column makes the look-up selective.
CREATE INDEX IF NOT EXISTS idx_article_views_news_viewer
    ON article_views (news_id, viewer_key, created_at);
`

// scoringColumns are added to the news table when it already exists from an
// older schema. Each ALTER is a no-op when the column is already present.
var scoringColumns = []struct{ name, decl string }{
	{"value_score", "INTEGER NOT NULL DEFAULT 0"},
	{"value_breakdown", "TEXT NOT NULL DEFAULT ''"},
	{"value_confidence", "REAL NOT NULL DEFAULT 0"},
	{"value_recommendation", "TEXT NOT NULL DEFAULT ''"},
	{"value_reason", "TEXT NOT NULL DEFAULT ''"},
	{"value_label", "TEXT NOT NULL DEFAULT ''"},
	{"value_method", "TEXT NOT NULL DEFAULT ''"},
}

// Migrate applies the schema and additive migrations. It is idempotent.
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(schema); err != nil {
		return err
	}

	existing := map[string]bool{}
	rows, err := db.Query(`PRAGMA table_info(news)`)
	if err != nil {
		return fmt.Errorf("pragma table_info: %w", err)
	}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			rows.Close()
			return fmt.Errorf("pragma scan: %w", err)
		}
		existing[name] = true
	}
	rows.Close()

	for _, col := range scoringColumns {
		if existing[col.name] {
			continue
		}
		if _, err := db.Exec(fmt.Sprintf(`ALTER TABLE news ADD COLUMN %s %s`, col.name, col.decl)); err != nil {
			return fmt.Errorf("add column %s: %w", col.name, err)
		}
	}
	return nil
}
