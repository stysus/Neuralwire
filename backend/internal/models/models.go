// Package models defines the core domain types for Neuralwire.
package models

import "time"

// NewsStatus represents the lifecycle of a news article.
type NewsStatus string

const (
	StatusDraft     NewsStatus = "draft"
	StatusPublished NewsStatus = "published"
	StatusRejected  NewsStatus = "rejected"
)

// Valid reports whether s is a known news status.
func (s NewsStatus) Valid() bool {
	switch s {
	case StatusDraft, StatusPublished, StatusRejected:
		return true
	default:
		return false
	}
}

// News is a single news article stored in the SQLite database.
type News struct {
	ID          int64      `json:"id"`
	Title       string     `json:"title"`
	Slug        string     `json:"slug"`
	URL         string     `json:"url"`
	Source      string     `json:"source"`
	Category    string     `json:"category"`
	Summary     string     `json:"summary"`
	Content     string     `json:"content"`
	ImageURL    string     `json:"image_url"`
	Status      NewsStatus `json:"status"`
	PublishedAt *time.Time `json:"published_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Category is a browseable news category.
type Category struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
}

// RSSSource is a configured RSS/Atom feed to poll.
type RSSSource struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	URL           string     `json:"url"`
	Category      string     `json:"category"`
	Enabled       bool       `json:"enabled"`
	LastFetchedAt *time.Time `json:"last_fetched_at"`
	CreatedAt     time.Time  `json:"created_at"`
}
