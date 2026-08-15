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

	// ValueScore fields are computed by the backend scoring pipeline
	// (AI + heuristic weighted). They are advisory: admins stay the final
	// decision makers and scoring never auto-publishes.
	ValueScore          int     `json:"value_score"`
	ValueBreakdown      string  `json:"value_breakdown"`
	ValueConfidence     float64 `json:"value_confidence"`
	ValueRecommendation string  `json:"value_recommendation"`
	ValueReason         string  `json:"value_reason"`
	ValueLabel          string  `json:"value_label"`
	ValueMethod         string  `json:"value_method"`
	// ViewCount is the number of recorded reads/views. It is populated only
	// for trending endpoints (computed from article_views), not stored on
	// the news row itself.
	ViewCount int `json:"view_count"`
}

// AutoPublishConfig controls the scheduled auto-fetch and auto-publish
// pipeline. It is stored in app_settings as JSON and editable from the admin
// panel (STY-57).
type AutoPublishConfig struct {
	// Enabled turns the scheduler on/off. When false no fetch or publish
	// runs automatically.
	Enabled bool `json:"enabled"`
	// AutoPostEnabled additionally publishes qualifying drafts (score +
	// category filter). When false, the scheduler only fetches (creates
	// drafts) without publishing anything.
	AutoPostEnabled bool `json:"auto_post_enabled"`
	// IntervalMinutes is how often the scheduler runs. 0 falls back to 360
	// (6 hours).
	IntervalMinutes int `json:"interval_minutes"`
	// Categories is the whitelist of category names to auto-post. Empty
	// means all categories are eligible.
	Categories []string `json:"categories"`
	// MinScoreLabel is the minimum value label ("high", "medium", "low") an
	// article must have to be auto-published. Empty means no score filter.
	// Kept for backward compatibility; prefer MinScoreLabels (STY-60).
	MinScoreLabel string `json:"min_score_label"`
	// MinScoreLabels is a whitelist of value labels allowed for auto-publish
	// (e.g. ["medium","high"]). Empty means any label passes. When set it
	// takes precedence over MinScoreLabel (STY-60).
	MinScoreLabels []string `json:"min_score_labels,omitempty"`
	// MaxPostsPerCycle caps how many drafts are auto-published in one cycle.
	// 0 means unlimited.
	MaxPostsPerCycle int `json:"max_posts_per_cycle,omitempty"`
	// PostIntervalMinutes is how often the auto-post step runs, independent
	// from the fetch interval. 0 means posts happen on the same schedule as
	// fetch (STY-60).
	PostIntervalMinutes int `json:"post_interval_minutes,omitempty"`
}

// DefaultAutoPublishConfig returns the out-of-the-box scheduler settings:
// disabled by default, 6-hour interval, no filters.
func DefaultAutoPublishConfig() AutoPublishConfig {
	return AutoPublishConfig{
		Enabled:             false,
		AutoPostEnabled:     false,
		IntervalMinutes:     360,
		Categories:          []string{},
		MinScoreLabel:       "",
		MinScoreLabels:      []string{},
		MaxPostsPerCycle:    0,
		PostIntervalMinutes: 0,
	}
}

// ScoreThresholds are the configurable bounds for the HIGH/MEDIUM/LOW
// advisory labels. Stored in app_settings so admins can tune them without a
// redeploy.
type ScoreThresholds struct {
	LowMax    int `json:"low_max"`    // scores below this are LOW
	MediumMin int `json:"medium_min"` // MEDIUM range start
	MediumMax int `json:"medium_max"` // MEDIUM range end
	HighMin   int `json:"high_min"`   // scores at/above this are HIGH
}

// Label maps a score to HIGH / MEDIUM / LOW using the configured thresholds.
func (t ScoreThresholds) Label(score int) string {
	if score >= t.HighMin {
		return "HIGH"
	}
	if score >= t.MediumMin && score <= t.MediumMax {
		return "MEDIUM"
	}
	return "LOW"
}

// DefaultScoreThresholds returns the out-of-the-box bounds.
func DefaultScoreThresholds() ScoreThresholds {
	return ScoreThresholds{LowMax: 59, MediumMin: 60, MediumMax: 79, HighMin: 80}
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
