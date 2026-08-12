package repository

import (
	"database/sql"
	"fmt"
	"time"

	"neuralwire/backend/internal/models"
	"neuralwire/backend/internal/slug"
)

// CategoryRepository persists news categories.
type CategoryRepository struct {
	db *sql.DB
}

// NewCategoryRepository creates a CategoryRepository.
func NewCategoryRepository(db *sql.DB) *CategoryRepository {
	return &CategoryRepository{db: db}
}

// List returns all categories ordered by name.
func (r *CategoryRepository) List() ([]models.Category, error) {
	rows, err := r.db.Query(
		`SELECT id, name, slug, created_at FROM categories ORDER BY name ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()

	categories := []models.Category{}
	for rows.Next() {
		var c models.Category
		var createdAt string
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &createdAt); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		t, err := parseSQLiteTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse category created_at: %w", err)
		}
		c.CreatedAt = t
		categories = append(categories, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate categories: %w", err)
	}
	return categories, nil
}

// EnsureCreated inserts a category when it does not exist yet and returns
// its slug. This keeps fetched articles referentially consistent.
func (r *CategoryRepository) EnsureCreated(name string) (string, error) {
	slug := slug.FromName(name)
	now := time.Now().UTC().Format(time.RFC3339)

	if _, err := r.db.Exec(
		`INSERT OR IGNORE INTO categories (name, slug, created_at) VALUES (?, ?, ?)`,
		name, slug, now,
	); err != nil {
		return "", fmt.Errorf("ensure category: %w", err)
	}
	return slug, nil
}
