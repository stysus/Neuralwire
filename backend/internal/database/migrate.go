package database

import "database/sql"

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
`

// Migrate applies the schema. It is idempotent.
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(schema); err != nil {
		return err
	}
	return nil
}
