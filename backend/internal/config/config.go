// Package config loads server configuration from environment variables.
package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds all runtime configuration for the backend.
type Config struct {
	// Port is the HTTP listen port (default "8080").
	Port string
	// DatabasePath is the SQLite database file path.
	DatabasePath string
	// CORSAllowOrigins is the list of allowed frontend origins.
	CORSAllowOrigins []string
	// AISummaryAPIKey enables AI summaries when non-empty.
	AISummaryAPIKey string
	// AISummaryModel is the OpenAI-compatible model to use.
	AISummaryModel string
	// AISummaryBaseURL is the OpenAI-compatible API base URL.
	AISummaryBaseURL string
	// CronSchedule is the cron expression for the RSS fetcher.
	CronSchedule string
	// FetchOnStartup triggers one fetch run when the server boots.
	FetchOnStartup bool
}

// Load builds a Config from the environment, applying defaults.
func Load() (Config, error) {
	fetchOnStartup, err := getenvBool("FETCH_ON_STARTUP", true)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Port:             getenv("PORT", "8080"),
		DatabasePath:     getenv("DB_PATH", "data/neuralwire.db"),
		CORSAllowOrigins: getenvList("CORS_ALLOW_ORIGIN", []string{"http://localhost:5173", "http://127.0.0.1:5173"}),
		AISummaryAPIKey:  os.Getenv("AI_SUMMARY_API_KEY"),
		AISummaryModel:   getenv("AI_SUMMARY_MODEL", "gpt-4o-mini"),
		AISummaryBaseURL: getenv("AI_SUMMARY_BASE_URL", "https://api.openai.com/v1"),
		CronSchedule:     getenv("CRON_SCHEDULE", "0 */6 * * *"),
		FetchOnStartup:   fetchOnStartup,
	}, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvBool(key string, fallback bool) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, err
	}
	return b, nil
}

// getenvList parses a comma-separated environment variable into a slice,
// trimming whitespace and dropping empty entries. It returns the fallback
// list when the variable is unset or empty.
func getenvList(key string, fallback []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	var out []string
	for _, part := range strings.Split(v, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}
