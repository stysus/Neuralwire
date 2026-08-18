// Package config loads server configuration from environment variables.
package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime configuration for the backend.
type Config struct {
	// AppEnv is the runtime environment: "development" (default) or
	// "production". Production refuses to boot with default credentials or a
	// development-only token secret.
	AppEnv string
	// TrustProxy enables trusting X-Forwarded-For for client IP resolution.
	// Only enable when the server is behind a trusted reverse proxy
	// (Nginx/Caddy/cloud LB); otherwise clients can spoof the header and
	// bypass per-IP rate limits.
	TrustProxy bool
	// Port is the HTTP listen port (default "8080").
	Port string
	// DatabasePath is the SQLite database file path.
	DatabasePath string
	// UserAgent is sent with every outbound RSS/scrape request. Defaults to
	// a browser-like NeuralwireBot dev UA; set a real bot UA with your domain
	// before going public.
	UserAgent string
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
	// AIImageGenerationEnabled enables AI cover-image generation (DALL-E style)
	// when true, disables it when false, and auto-detects from the provider /
	// base URL when unset. Providers that cannot generate images (DeepSeek,
	// Ollama, Gemini, Groq, OpenRouter) skip generation and use stock fallback.
	AIImageGenerationEnabled *bool
	// AdminUsername is the login for POST /api/admin/login.
	AdminUsername string
	// AdminPassword is the login password. Change it outside development.
	AdminPassword string
	// AdminTokenSecret signs admin bearer tokens. Change it outside dev.
	AdminTokenSecret string
	// ScrapeMaxPerSource caps how many newest articles per source get
	// full-content scraping in one fetch cycle.
	ScrapeMaxPerSource int
	// ScrapeMaxInsertPerSource caps how many new articles per source are
	// stored as drafts in one fetch cycle (scraped or fallback alike).
	ScrapeMaxInsertPerSource int
	// ScrapeTimeout bounds each full-content scrape attempt.
	ScrapeTimeout time.Duration
	// ScrapeMinContentChars is the minimum content length for a fetched
	// article to be kept as a draft; shorter content is skipped.
	ScrapeMinContentChars int
	// ScrapeDelayMin is the lower bound of the politeness delay applied
	// before every external request during a fetch cycle.
	ScrapeDelayMin time.Duration
	// ScrapeDelayMax is the upper bound of the politeness delay applied
	// before every external request during a fetch cycle.
	ScrapeDelayMax time.Duration
	// ViewRateLimit is the max view-count requests per IP per minute
	// (default 30). <=0 disables the per-IP view limiter.
	ViewRateLimit int
	// TrendingCacheTTLSeconds caches trending results for this many seconds
	// (default 300 = 5m). <=0 disables trending caching.
	TrendingCacheTTLSeconds int
	// LoginRateLimit is the max login attempts per IP per minute
	// (default 5). <=0 disables login rate limiting.
	LoginRateLimit int
	// GlobalRateLimit is the max requests per IP per minute applied to every
	// request (default 120). <=0 disables global rate limiting.
	GlobalRateLimit int
	// HTTPCompressionEnabled enables gzip/brotli response compression
	// (default true). Set to false to disable compression entirely.
	HTTPCompressionEnabled bool
	// LogLevel is the minimum log level emitted: "debug", "info", "warn" or
	// "error" (default "info").
	LogLevel string
	// LogFormat is the structured log output format: "text" (default) or
	// "json".
	LogFormat string
	// StaticDir is the path to the built frontend (adapter-static output).
	// When non-empty and the directory exists, the server serves these static
	// files so the frontend and API run from one process in production.
	StaticDir string
	// UploadDir is the directory where admin-uploaded images are stored. It
	// must be writable by the server process; in production it should point
	// at a persistent volume. Files are served under /uploads/.
	UploadDir string
	// BackupDir is where database backups are stored. Empty disables backups.
	BackupDir string
	// BackupRetention keeps the newest N backups and prunes older ones.
	BackupRetention int
	// BackupIntervalHours is how often the automatic backup runs (0 disables).
	BackupIntervalHours int
}

// Load builds a Config from the environment, applying defaults. It first
// loads a .env file from the current working directory (without overriding
// variables already present in the process environment), then reads the
// environment.
func Load() (Config, error) {
	loadDotEnv(".env")

	baseURL, model := summaryDefaults(
		getenv("AI_SUMMARY_PROVIDER", ""),
		getenv("AI_SUMMARY_BASE_URL", ""),
		getenv("AI_SUMMARY_MODEL", ""),
	)

	scrapeMax, err := getenvInt("SCRAPE_MAX_PER_SOURCE", 5)
	if err != nil {
		return Config{}, err
	}
	scrapeMaxInsert, err := getenvInt("SCRAPE_MAX_INSERT_PER_SOURCE", 5)
	if err != nil {
		return Config{}, err
	}
	scrapeTimeoutSec, err := getenvInt("SCRAPE_TIMEOUT_SECONDS", 15)
	if err != nil {
		return Config{}, err
	}
	scrapeMinContent, err := getenvInt("SCRAPE_MIN_CONTENT_CHARS", 500)
	if err != nil {
		return Config{}, err
	}
	scrapeDelayMinSec, err := getenvInt("SCRAPE_DELAY_MIN_SECONDS", 1)
	if err != nil {
		return Config{}, err
	}
	scrapeDelayMaxSec, err := getenvInt("SCRAPE_DELAY_MAX_SECONDS", 2)
	if err != nil {
		return Config{}, err
	}
	viewRateLimit, err := getenvInt("VIEW_RATE_LIMIT", 30)
	if err != nil {
		return Config{}, err
	}
	trendingCacheTTL, err := getenvInt("TRENDING_CACHE_TTL_SECONDS", 300)
	if err != nil {
		return Config{}, err
	}
	loginRateLimit, err := getenvInt("LOGIN_RATE_LIMIT", 5)
	if err != nil {
		return Config{}, err
	}
	globalRateLimit, err := getenvInt("GLOBAL_RATE_LIMIT", 120)
	if err != nil {
		return Config{}, err
	}

	imgGenEnabled := parseOptionalBool(os.Getenv("AI_IMAGE_GENERATION_ENABLED"))
	trustProxy := parseOptionalBool(os.Getenv("TRUST_PROXY"))
	compressionEnabled := parseOptionalBool(os.Getenv("HTTP_COMPRESSION_ENABLED"))

	return Config{
		AppEnv:                   strings.ToLower(strings.TrimSpace(getenv("APP_ENV", "development"))),
		TrustProxy:               trustProxy != nil && *trustProxy,
		Port:                     getenv("PORT", "8080"),
		DatabasePath:             getenv("DB_PATH", "data/neuralwire.db"),
		UserAgent:                getenv("USER_AGENT", "Mozilla/5.0 (compatible; NeuralwireBot/1.0-dev; +https://neuralwire.example)"),
		CORSAllowOrigins:         getenvList("CORS_ALLOW_ORIGIN", []string{"http://localhost:5173", "http://127.0.0.1:5173"}),
		AISummaryAPIKey:          os.Getenv("AI_SUMMARY_API_KEY"),
		AISummaryModel:           model,
		AISummaryBaseURL:         baseURL,
		AISummaryProvider:        getenv("AI_SUMMARY_PROVIDER", ""),
		AIImageGenerationEnabled: imgGenEnabled,
		AdminUsername:            getenv("ADMIN_USERNAME", "admin"),
		AdminPassword:            getenv("ADMIN_PASSWORD", "admin123"),
		AdminTokenSecret:         getenv("ADMIN_TOKEN_SECRET", "neuralwire-dev-secret-7f3c9a1e4b8d2f6a"),
		ScrapeMaxPerSource:       scrapeMax,
		ScrapeMaxInsertPerSource: scrapeMaxInsert,
		ScrapeTimeout:            time.Duration(scrapeTimeoutSec) * time.Second,
		ScrapeMinContentChars:    scrapeMinContent,
		ScrapeDelayMin:           time.Duration(scrapeDelayMinSec) * time.Second,
		ScrapeDelayMax:           time.Duration(scrapeDelayMaxSec) * time.Second,
		ViewRateLimit:            viewRateLimit,
		TrendingCacheTTLSeconds:  trendingCacheTTL,
		LoginRateLimit:           loginRateLimit,
		GlobalRateLimit:          globalRateLimit,
		HTTPCompressionEnabled:   compressionEnabled == nil || *compressionEnabled,
		LogLevel:                 strings.ToLower(strings.TrimSpace(getenv("LOG_LEVEL", "info"))),
		LogFormat:                strings.ToLower(strings.TrimSpace(getenv("LOG_FORMAT", "text"))),
		StaticDir:                strings.TrimSpace(getenv("STATIC_DIR", "../frontend/build")),
		UploadDir:                strings.TrimSpace(getenv("UPLOAD_DIR", "./data/uploads")),
		BackupDir:                strings.TrimSpace(getenv("BACKUP_DIR", "./data/backups")),
		BackupRetention:          getenvIntOr("BACKUP_RETENTION", 7),
		BackupIntervalHours:      getenvIntOr("BACKUP_INTERVAL_HOURS", 24),
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

// getenvInt parses an integer environment variable, returning the fallback
// when the variable is unset or empty.
func getenvInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return 0, err
	}
	return n, nil
}

// getenvIntOr parses an integer environment variable, silently falling back
// to the default when unset or invalid (used for optional settings).
func getenvIntOr(key string, fallback int) int {
	n, err := getenvInt(key, fallback)
	if err != nil {
		return fallback
	}
	return n
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

// parseOptionalBool parses a boolean env value ("1","true","yes","on" => true,
// "0","false","no","off" => false). Empty or invalid returns nil (auto-detect).
func parseOptionalBool(v string) *bool {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return nil
	}
	switch v {
	case "1", "true", "yes", "on":
		b := true
		return &b
	case "0", "false", "no", "off":
		b := false
		return &b
	default:
		return nil
	}
}

// SupportsImageGeneration reports whether the configured AI provider/endpoint
// can generate images (DALL-E style /images/generations). Known text-only
// providers (DeepSeek, Ollama, Gemini, Groq, OpenRouter) and localhost return
// false so image generation is skipped instead of spamming 404s.
func SupportsImageGeneration(provider, baseURL string) bool {
	provider = strings.ToLower(strings.TrimSpace(provider))
	baseURL = strings.ToLower(strings.TrimSpace(baseURL))

	if strings.Contains(baseURL, "localhost") || strings.Contains(baseURL, "127.0.0.1") {
		return false
	}
	// Text-only providers that expose chat completions but no images endpoint.
	for _, textOnly := range []string{
		"deepseek", "ollama", "gemini", "groq", "openrouter",
	} {
		if strings.Contains(provider, textOnly) || strings.Contains(baseURL, textOnly) {
			return false
		}
	}
	// OpenAI (or any OpenAI-compatible host advertising images) is assumed to
	// support generation unless explicitly disabled via the env flag.
	return provider == "" || strings.Contains(provider, "openai") || strings.Contains(baseURL, "openai")
}

// loadDotEnv reads a simple KEY=VALUE dot-env file and exports the variables
// that are not already set in the process environment. Lines starting with
// '#' are comments; inline '#' after the value is ignored. Values may be
// wrapped in single or double quotes. Missing or unreadable files are
// silently ignored so the app still runs without a .env.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if i := strings.Index(value, " #"); i >= 0 {
			value = strings.TrimSpace(value[:i])
		}
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		if key == "" || os.Getenv(key) != "" {
			continue
		}
		os.Setenv(key, value)
	}
}

// ReadDotEnvDirect reads a key-value file directly from disk and returns the mapped keys.
func ReadDotEnvDirect(path string) map[string]string {
	m := make(map[string]string)
	f, err := os.Open(path)
	if err != nil {
		return m
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if i := strings.Index(value, " #"); i >= 0 {
			value = strings.TrimSpace(value[:i])
		}
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		if key != "" {
			m[key] = value
		}
	}
	return m
}

// LoadAIConfig reads the latest AI configuration directly from the .env file.
func LoadAIConfig() (apiKey, baseURL, model string) {
	env := ReadDotEnvDirect(".env")

	// Get values from .env directly, fallback to os.Getenv
	apiKey = env["AI_SUMMARY_API_KEY"]
	if apiKey == "" {
		apiKey = os.Getenv("AI_SUMMARY_API_KEY")
	}

	provider := env["AI_SUMMARY_PROVIDER"]
	if provider == "" {
		provider = os.Getenv("AI_SUMMARY_PROVIDER")
	}

	rawBaseURL := env["AI_SUMMARY_BASE_URL"]
	if rawBaseURL == "" {
		rawBaseURL = os.Getenv("AI_SUMMARY_BASE_URL")
	}

	rawModel := env["AI_SUMMARY_MODEL"]
	if rawModel == "" {
		rawModel = os.Getenv("AI_SUMMARY_MODEL")
	}

	baseURL, model = summaryDefaults(provider, rawBaseURL, rawModel)

	// Clean up baseURL to prevent misconfiguration if user appends endpoint paths
	baseURL = strings.TrimSuffix(baseURL, "/chat/completions")
	baseURL = strings.TrimSuffix(baseURL, "/chat/completions/")
	baseURL = strings.TrimSuffix(baseURL, "/images/generations")
	baseURL = strings.TrimSuffix(baseURL, "/images/generations/")

	return apiKey, baseURL, model
}
