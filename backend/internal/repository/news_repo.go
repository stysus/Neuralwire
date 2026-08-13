// Package repository contains SQL-backed data access for the domain types.
package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"neuralwire/backend/internal/models"
	"neuralwire/backend/internal/slug"
)

// NewsRepository persists news articles.
type NewsRepository struct {
	db *sql.DB
}

// NewNewsRepository creates a NewsRepository.
func NewNewsRepository(db *sql.DB) *NewsRepository {
	return &NewsRepository{db: db}
}

const newsColumns = `id, title, slug, url, source, category, summary,
	content, image_url, status, published_at, created_at,
	value_score, value_breakdown, value_confidence, value_recommendation,
	value_reason, value_label, value_method`

// Create inserts a draft article and returns its ID. The slug is derived
// from the title and made unique in the database.
func (r *NewsRepository) Create(n models.News) (int64, error) {
	uniqueSlug, err := slug.Unique(slug.FromTitle(n.Title), r.slugExists)
	if err != nil {
		return 0, fmt.Errorf("unique slug: %w", err)
	}

	res, err := r.db.Exec(`
		INSERT INTO news (title, slug, url, source, category, summary, content, image_url, status,
			value_score, value_breakdown, value_confidence, value_recommendation,
			value_reason, value_label, value_method)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.Title, uniqueSlug, n.URL, n.Source, n.Category,
		n.Summary, n.Content, n.ImageURL, string(models.StatusDraft),
		n.ValueScore, n.ValueBreakdown, n.ValueConfidence, n.ValueRecommendation,
		n.ValueReason, n.ValueLabel, n.ValueMethod,
	)
	if err != nil {
		return 0, fmt.Errorf("insert news: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	return id, nil
}

// GetByID returns an article by primary key.
func (r *NewsRepository) GetByID(id int64) (*models.News, error) {
	row := r.db.QueryRow(`SELECT `+newsColumns+` FROM news WHERE id = ?`, id)
	return scanNews(row)
}

// GetBySlug returns an article by slug.
func (r *NewsRepository) GetBySlug(slug string) (*models.News, error) {
	row := r.db.QueryRow(`SELECT `+newsColumns+` FROM news WHERE slug = ?`, slug)
	return scanNews(row)
}

// ExistsByURL reports whether an article with the given URL already exists.
func (r *NewsRepository) ExistsByURL(url string) (bool, error) {
	var exists int
	err := r.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM news WHERE url = ?)`, url).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("exists by url: %w", err)
	}
	return exists == 1, nil
}

func (r *NewsRepository) slugExists(s string) (bool, error) {
	var exists int
	err := r.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM news WHERE slug = ?)`, s).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("exists by slug: %w", err)
	}
	return exists == 1, nil
}

// ListPublished returns a page of published articles ordered by publish
// date descending. An empty category filters across all categories.
func (r *NewsRepository) ListPublished(category string, page, pageSize int) ([]models.News, int, error) {
	where := "WHERE status = ?"
	args := []any{string(models.StatusPublished)}
	if category != "" {
		where += " AND category = ?"
		args = append(args, category)
	}

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM news `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count published: %w", err)
	}

	query := `SELECT ` + newsColumns + ` FROM news ` + where +
		` ORDER BY published_at DESC, id DESC LIMIT ? OFFSET ?`
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list published: %w", err)
	}
	defer rows.Close()

	news := make([]models.News, 0, pageSize)
	for rows.Next() {
		n, err := scanNews(rows)
		if err != nil {
			return nil, 0, err
		}
		news = append(news, *n)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate news: %w", err)
	}
	return news, total, nil
}

// ListAdmin returns a page of articles across all statuses ordered by
// creation date descending. An empty filter includes everything.
func (r *NewsRepository) ListAdmin(status, category, valueLabel string, page, pageSize int) ([]models.News, int, error) {
	whereClauses := []string{}
	args := []any{}
	if status != "" {
		whereClauses = append(whereClauses, "status = ?")
		args = append(args, status)
	}
	if category != "" {
		whereClauses = append(whereClauses, "category = ?")
		args = append(args, category)
	}
	if valueLabel != "" {
		whereClauses = append(whereClauses, "value_label = ?")
		args = append(args, valueLabel)
	}

	where := ""
	if len(whereClauses) > 0 {
		where = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM news `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count admin news: %w", err)
	}

	query := `SELECT ` + newsColumns + ` FROM news ` + where +
		` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list admin news: %w", err)
	}
	defer rows.Close()

	news := make([]models.News, 0, pageSize)
	for rows.Next() {
		n, err := scanNews(rows)
		if err != nil {
			return nil, 0, err
		}
		news = append(news, *n)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate admin news: %w", err)
	}
	return news, total, nil
}

// SetStatus transitions an article to the given status. When publishing,
// published_at is set to now unless it is already set.
func (r *NewsRepository) SetStatus(id int64, status models.NewsStatus) error {
	var err error
	switch status {
	case models.StatusPublished:
		_, err = r.db.Exec(`
			UPDATE news
			SET status = ?, published_at = COALESCE(published_at, ?)
			WHERE id = ?`,
			string(status), time.Now().UTC().Format(time.RFC3339), id,
		)
	case models.StatusRejected:
		_, err = r.db.Exec(`UPDATE news SET status = ? WHERE id = ?`, string(status), id)
	default:
		return fmt.Errorf("unsupported status transition: %s", status)
	}
	if err != nil {
		return fmt.Errorf("set status: %w", err)
	}
	return nil
}

// Update updates an article's fields.
func (r *NewsRepository) Update(id int64, n models.News) error {
	existing, err := r.GetByID(id)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("news not found")
	}

	uniqueSlug := existing.Slug
	if existing.Title != n.Title {
		uniqueSlug, err = slug.Unique(slug.FromTitle(n.Title), r.slugExists)
		if err != nil {
			return fmt.Errorf("unique slug: %w", err)
		}
	}

	_, err = r.db.Exec(`
		UPDATE news
		SET title = ?, slug = ?, category = ?, summary = ?, content = ?, image_url = ?,
			value_score = ?, value_breakdown = ?, value_confidence = ?,
			value_recommendation = ?, value_reason = ?, value_label = ?, value_method = ?
		WHERE id = ?`,
		n.Title, uniqueSlug, n.Category, n.Summary, n.Content, n.ImageURL,
		n.ValueScore, n.ValueBreakdown, n.ValueConfidence, n.ValueRecommendation,
		n.ValueReason, n.ValueLabel, n.ValueMethod,
		id,
	)
	if err != nil {
		return fmt.Errorf("update news: %w", err)
	}
	return nil
}

// Delete removes an article.
func (r *NewsRepository) Delete(id int64) error {
	if _, err := r.db.Exec(`DELETE FROM news WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete news: %w", err)
	}
	return nil
}

// DeleteByStatus removes all articles matching a specific status (e.g. draft, published).
func (r *NewsRepository) DeleteByStatus(status string) error {
	if _, err := r.db.Exec(`DELETE FROM news WHERE status = ?`, status); err != nil {
		return fmt.Errorf("delete news by status: %w", err)
	}
	return nil
}

// scanner is satisfied by *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanNews(s scanner) (*models.News, error) {
	var n models.News
	var status string
	var publishedAt sql.NullString
	var createdAt string

	if err := s.Scan(
		&n.ID, &n.Title, &n.Slug, &n.URL, &n.Source, &n.Category,
		&n.Summary, &n.Content, &n.ImageURL, &status, &publishedAt, &createdAt,
		&n.ValueScore, &n.ValueBreakdown, &n.ValueConfidence, &n.ValueRecommendation,
		&n.ValueReason, &n.ValueLabel, &n.ValueMethod,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan news: %w", err)
	}

	n.Status = models.NewsStatus(status)
	if publishedAt.Valid {
		t, err := parseSQLiteTime(publishedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse published_at: %w", err)
		}
		n.PublishedAt = &t
	}
	t, err := parseSQLiteTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	n.CreatedAt = t
	return &n, nil
}

// parseSQLiteTime accepts both RFC3339 (written by the app) and the
// "YYYY-MM-DD HH:MM:SS" format produced by SQLite's CURRENT_TIMESTAMP.
func parseSQLiteTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02 15:04:05", s)
}
