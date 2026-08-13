// Package repository contains SQL-backed data access for the domain types.
package repository

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"neuralwire/backend/internal/models"
	"neuralwire/backend/internal/scoring"
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
// date descending. An empty category filters across all categories. A
// non-empty query filters by keyword match on title or summary; multi-word
// queries are split into tokens and matched with AND semantics (each token
// must appear in the title or summary), so "gemma model" finds articles
// containing both words anywhere.
func (r *NewsRepository) ListPublished(category, query string, page, pageSize int) ([]models.News, int, error) {
	where := "WHERE status = ?"
	args := []any{string(models.StatusPublished)}
	if category != "" {
		where += " AND category = ?"
		args = append(args, category)
	}
	if query != "" {
		tokens := strings.Fields(strings.ToLower(query))
		var conds []string
		for _, tok := range tokens {
			like := "%" + tok + "%"
			conds = append(conds, `(LOWER(title) LIKE ? OR LOWER(summary) LIKE ?)`)
			args = append(args, like, like)
		}
		if len(conds) > 0 {
			where += " AND (" + strings.Join(conds, " AND ") + ")"
		}
	}

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM news `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count published: %w", err)
	}

	querySQL := `SELECT ` + newsColumns + ` FROM news ` + where +
		` ORDER BY published_at DESC, id DESC LIMIT ? OFFSET ?`
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := r.db.Query(querySQL, args...)
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

// viewCooldown bounds how often the same viewer can count as a new read for
// the same article, so refreshes or accidental re-opens don't inflate views.
const viewCooldown = 6 * time.Hour

// RecordView counts a read of an article. A viewer_key (client-generated id)
// combined with the cooldown window deduplicates views per visitor. It is a
// no-op when the article does not exist.
func (r *NewsRepository) RecordView(newsID int64, viewerKey string) error {
	var exists int
	if err := r.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM news WHERE id = ?)`, newsID).Scan(&exists); err != nil {
		return fmt.Errorf("exists news for view: %w", err)
	}
	if exists == 0 {
		return nil
	}

	if viewerKey != "" {
		// Only count once per viewer per cooldown window. Both sides are
		// normalized to UTC RFC3339 so SQLite's string comparison is correct.
		var recent int
		err := r.db.QueryRow(`
			SELECT EXISTS(
				SELECT 1 FROM article_views
				WHERE news_id = ? AND viewer_key = ? AND created_at > ?
			)`,
			newsID, viewerKey, time.Now().UTC().Add(-viewCooldown).Format(time.RFC3339),
		).Scan(&recent)
		if err != nil {
			return fmt.Errorf("check recent view: %w", err)
		}
		if recent == 1 {
			return nil
		}
	}

	if _, err := r.db.Exec(
		`INSERT INTO article_views (news_id, viewer_key, created_at) VALUES (?, ?, ?)`,
		newsID, viewerKey, time.Now().UTC().Format(time.RFC3339),
	); err != nil {
		return fmt.Errorf("insert article view: %w", err)
	}
	return nil
}

// TrendingWindow selects the time range for trending rankings.
type TrendingWindow string

const (
	TrendingDay  TrendingWindow = "day"
	TrendingWeek TrendingWindow = "week"
	TrendingAll  TrendingWindow = "all"
)

// ListTrending returns the most-viewed published articles in the given time
// window, ordered by view count descending then newest first. Limit is
// clamped to 1-50.
func (r *NewsRepository) ListTrending(window TrendingWindow, limit int) ([]models.News, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 50 {
		limit = 50
	}

	// published_at is in UTC RFC3339; article_views.created_at is the same.
	where := ""
	args := []any{}
	switch window {
	case TrendingDay:
		where = `AND av.created_at > ?`
		args = append(args, time.Now().UTC().Add(-24*time.Hour).Format(time.RFC3339))
	case TrendingWeek:
		where = `AND av.created_at > ?`
		args = append(args, time.Now().UTC().Add(-7*24*time.Hour).Format(time.RFC3339))
	}

	query := `
		SELECT n.id, n.title, n.slug, n.url, n.source, n.category, n.summary,
			n.content, n.image_url, n.status, n.published_at, n.created_at,
			n.value_score, n.value_breakdown, n.value_confidence, n.value_recommendation,
			n.value_reason, n.value_label, n.value_method,
			COUNT(av.id) AS view_count
		FROM news n
		JOIN article_views av ON av.news_id = n.id
		WHERE n.status = ? ` + where + `
		GROUP BY n.id
		ORDER BY view_count DESC, n.published_at DESC, n.id DESC
		LIMIT ?`
	args = append([]any{string(models.StatusPublished)}, args...)
	args = append(args, limit)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list trending: %w", err)
	}
	defer rows.Close()

	var news []models.News
	for rows.Next() {
		var n models.News
		var status string
		var publishedAt sql.NullString
		var createdAt string
		if err := rows.Scan(
			&n.ID, &n.Title, &n.Slug, &n.URL, &n.Source, &n.Category,
			&n.Summary, &n.Content, &n.ImageURL, &status, &publishedAt, &createdAt,
			&n.ValueScore, &n.ValueBreakdown, &n.ValueConfidence, &n.ValueRecommendation,
			&n.ValueReason, &n.ValueLabel, &n.ValueMethod,
			&n.ViewCount,
		); err != nil {
			return nil, fmt.Errorf("scan trending: %w", err)
		}
		n.Status = models.NewsStatus(status)
		if publishedAt.Valid {
			if t, err := parseSQLiteTime(publishedAt.String); err == nil {
				n.PublishedAt = &t
			}
		}
		if t, err := parseSQLiteTime(createdAt); err == nil {
			n.CreatedAt = t
		}
		news = append(news, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trending: %w", err)
	}
	return news, nil
}

// ListRelated returns the published articles most similar to the given one,
// ranked by a TF-IDF weighted relevance score (category + source + keyword
// overlap where rare keywords weigh more). The current article is excluded.
// Limit is clamped to 1-50.
func (r *NewsRepository) ListRelated(currentID int64, limit int) ([]models.News, error) {
	if limit <= 0 {
		limit = 12
	}
	if limit > 50 {
		limit = 50
	}

	current, err := r.GetByID(currentID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, nil
	}

	rows, err := r.db.Query(`
		SELECT `+newsColumns+`
		FROM news
		WHERE status = ? AND id != ?
		ORDER BY published_at DESC, id DESC`, string(models.StatusPublished), currentID)
	if err != nil {
		return nil, fmt.Errorf("list related candidates: %w", err)
	}
	defer rows.Close()

	var candidates []models.News
	for rows.Next() {
		n, err := scanNews(rows)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, *n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate related candidates: %w", err)
	}

	// TF-IDF weighting: compute document frequency of each keyword across
	// the whole candidate corpus so rare/meaningful terms dominate.
	docFreq := make(map[string]int)
	for _, c := range candidates {
		kw := scoring.RelatedKeywords(c.Title + " " + c.Summary)
		for k := range kw {
			docFreq[k]++
		}
	}
	total := len(candidates)
	weight := func(kw string) float64 {
		df := docFreq[kw]
		if df <= 0 {
			return 0
		}
		return 1 + float64(total)/float64(df) // common words approach 1, rare words grow
	}

	currentText := current.Title + " " + current.Summary
	curKW := scoring.RelatedKeywords(currentText)

	type scored struct {
		news  models.News
		score float64
	}
	var ranked []scored
	for _, c := range candidates {
		s := 0.0
		if strings.EqualFold(strings.TrimSpace(current.Category), strings.TrimSpace(c.Category)) {
			s += 40
		}
		if strings.EqualFold(strings.TrimSpace(current.Source), strings.TrimSpace(c.Source)) {
			s += 10
		}
		candKW := scoring.RelatedKeywords(c.Title + " " + c.Summary)
		for k := range curKW {
			if _, ok := candKW[k]; ok {
				s += 2 * weight(k)
			}
		}
		if s > 0 {
			ranked = append(ranked, scored{news: c, score: s})
		}
	}

	// Stable sort: score desc, then newest first.
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].news.ID > ranked[j].news.ID
	})

	out := make([]models.News, 0, limit)
	for i := 0; i < len(ranked) && i < limit; i++ {
		out = append(out, ranked[i].news)
	}
	return out, nil
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
