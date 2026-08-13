package repository

import (
	"database/sql"
	"fmt"
	"strconv"

	"neuralwire/backend/internal/models"
)

// SettingsRepository reads and writes admin-configurable app settings.
type SettingsRepository struct {
	db *sql.DB
}

// NewSettingsRepository creates a SettingsRepository.
func NewSettingsRepository(db *sql.DB) *SettingsRepository {
	return &SettingsRepository{db: db}
}

// GetScoreThresholds loads the HIGH/MEDIUM/LOW bounds from app_settings,
// falling back to defaults for any missing or invalid key.
func (r *SettingsRepository) GetScoreThresholds() models.ScoreThresholds {
	def := models.DefaultScoreThresholds()
	vals := map[string]int{
		"score_low_max":    def.LowMax,
		"score_medium_min": def.MediumMin,
		"score_medium_max": def.MediumMax,
		"score_high_min":   def.HighMin,
	}

	rows, err := r.db.Query(`SELECT key, value FROM app_settings WHERE key IN ('score_low_max','score_medium_min','score_medium_max','score_high_min')`)
	if err != nil {
		return def
	}
	defer rows.Close()

	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			continue
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			continue
		}
		vals[k] = n
	}

	return models.ScoreThresholds{
		LowMax:    vals["score_low_max"],
		MediumMin: vals["score_medium_min"],
		MediumMax: vals["score_medium_max"],
		HighMin:   vals["score_high_min"],
	}
}

// SetScoreThresholds persists the threshold bounds. A missing key falls back
// to the default value.
func (r *SettingsRepository) SetScoreThresholds(t models.ScoreThresholds) error {
	def := models.DefaultScoreThresholds()
	vals := map[string]int{
		"score_low_max":    t.LowMax,
		"score_medium_min": t.MediumMin,
		"score_medium_max": t.MediumMax,
		"score_high_min":   t.HighMin,
	}
	// Sanity: clamp to 0-100.
	for k, v := range vals {
		if v < 0 || v > 100 {
			vals[k] = 50
		}
	}
	// Fallback missing zero values to defaults.
	for k, v := range vals {
		if v == 0 {
			switch k {
			case "score_low_max":
				vals[k] = def.LowMax
			case "score_medium_min":
				vals[k] = def.MediumMin
			case "score_medium_max":
				vals[k] = def.MediumMax
			case "score_high_min":
				vals[k] = def.HighMin
			}
		}
	}

	for k, v := range vals {
		if _, err := r.db.Exec(`
			INSERT INTO app_settings (key, value, updated_at)
			VALUES (?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`,
			k, strconv.Itoa(v),
		); err != nil {
			return fmt.Errorf("upsert setting %s: %w", k, err)
		}
	}
	return nil
}
