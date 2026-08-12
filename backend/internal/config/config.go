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
	// AISummaryProvider is a named preset that sets BaseURL/Model defaults.
	AISummaryProvider string
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

	baseURL, model := summaryDefaults(
		getenv("AI_SUMMARY_PROVIDER", ""),
		getenv("AI_SUMMARY_BASE_URL", ""),
		getenv("AI_SUMMARY_MODEL", ""),
	)

	return Config{
		Port:              getenv("PORT", "8080"),
		DatabasePath:      getenv("DB_PATH", "data/neuralwire.db"),
		CORSAllowOrigins:  getenvList("CORS_ALLOW_ORIGIN", []string{"http://localhost:5173", "http://127.0.0.1:5173"}),
		AISummaryAPIKey:   os.Getenv("AI_SUMMARY_API_KEY"),
		AISummaryModel:    model,
		AISummaryBaseURL:  baseURL,
		AISummaryProvider: getenv("AI_SUMMARY_PROVIDER", ""),
		CronSchedule:      getenv("CRON_SCHEDULE", "0 */6 * * *"),
		FetchOnStartup:    fetchOnStartup,
	}, nil
}

// summaryDefaults resolves the AI summary endpoint and model. An explicit
// AI_SUMMARY_BASE_URL/AI_SUMMARY_MODEL always win; otherwise a named provider
// preset supplies sensible defaults, falling back to OpenAI.
func summaryDefaults(provider, baseURL, model string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai":
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		if model == "" {
			model = "gpt-4o-mini"
		}
	case "gemini":
		if baseURL == "" {
			baseURL = "https://generativelanguage.googleapis.com/v1beta/openai"
		}
		if model == "" {
			model = "gemini-1.5-flash"
		}
	case "openrouter":
		if baseURL == "" {
			baseURL = "https://openrouter.ai/api/v1"
		}
		if model == "" {
			model = "openai/gpt-4o-mini"
		}
	case "groq":
		if baseURL == "" {
			baseURL = "https://api.groq.com/openai/v1"
		}
		if model == "" {
			model = "llama-3.3-70b-versatile"
		}
	case "ollama":
		if baseURL == "" {
			baseURL = "http://localhost:11434/v1"
		}
		if model == "" {
			model = "llama3.2"
		}
	default:
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		if model == "" {
			model = "gpt-4o-mini"
		}
	}
	return baseURL, model
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
