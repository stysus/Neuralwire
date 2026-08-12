package repository

import (
	"database/sql"
	"fmt"
	"time"

	"neuralwire/backend/internal/models"
)

// RSSSourceRepository persists configured RSS feeds.
type RSSSourceRepository struct {
	db *sql.DB
}

// NewRSSSourceRepository creates an RSSSourceRepository.
func NewRSSSourceRepository(db *sql.DB) *RSSSourceRepository {
	return &RSSSourceRepository{db: db}
}

// ListEnabled returns all feeds that should be polled.
func (r *RSSSourceRepository) ListEnabled() ([]models.RSSSource, error) {
	rows, err := r.db.Query(`
		SELECT id, name, url, category, enabled, last_fetched_at, created_at
		FROM rss_sources
		WHERE enabled = 1
		ORDER BY name ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list rss sources: %w", err)
	}
	defer rows.Close()

	sources := []models.RSSSource{}
	for rows.Next() {
		var s models.RSSSource
		var enabled int
		var lastFetched sql.NullString
		var createdAt string
		if err := rows.Scan(
			&s.ID, &s.Name, &s.URL, &s.Category, &enabled, &lastFetched, &createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan rss source: %w", err)
		}
		s.Enabled = enabled == 1
		if lastFetched.Valid {
			t, err := parseSQLiteTime(lastFetched.String)
			if err != nil {
				return nil, fmt.Errorf("parse last_fetched_at: %w", err)
			}
			s.LastFetchedAt = &t
		}
		t, err := parseSQLiteTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse rss source created_at: %w", err)
		}
		s.CreatedAt = t
		sources = append(sources, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rss sources: %w", err)
	}
	return sources, nil
}

// UpdateLastFetched records the time a source was last polled.
func (r *RSSSourceRepository) UpdateLastFetched(id int64, t time.Time) error {
	if _, err := r.db.Exec(
		`UPDATE rss_sources SET last_fetched_at = ? WHERE id = ?`,
		t.UTC().Format(time.RFC3339), id,
	); err != nil {
		return fmt.Errorf("update last_fetched: %w", err)
	}
	return nil
}
