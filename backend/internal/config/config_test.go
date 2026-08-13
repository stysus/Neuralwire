package config

import (
	"testing"
	"time"
)

// clearEnv removes every variable Load reads so the test sees pure defaults.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"PORT", "DB_PATH", "USER_AGENT", "CORS_ALLOW_ORIGIN", "AI_SUMMARY_API_KEY",
		"AI_SUMMARY_MODEL", "AI_SUMMARY_BASE_URL", "AI_SUMMARY_PROVIDER",
		"AI_IMAGE_GENERATION_ENABLED", "APP_ENV", "TRUST_PROXY",
		"ADMIN_USERNAME", "ADMIN_PASSWORD",
		"ADMIN_TOKEN_SECRET", "SCRAPE_MAX_PER_SOURCE", "SCRAPE_MAX_INSERT_PER_SOURCE",
		"SCRAPE_TIMEOUT_SECONDS",
		"SCRAPE_MIN_CONTENT_CHARS", "SCRAPE_DELAY_MIN_SECONDS", "SCRAPE_DELAY_MAX_SECONDS",
		"VIEW_RATE_LIMIT", "TRENDING_CACHE_TTL_SECONDS", "LOGIN_RATE_LIMIT", "GLOBAL_RATE_LIMIT",
	} {
		t.Setenv(key, "")
	}
}

func TestLoadDefaults(t *testing.T) {
	clearEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want 8080", cfg.Port)
	}
	if cfg.UserAgent == "" {
		t.Error("UserAgent empty, want default NeuralwireBot dev UA")
	}
	if cfg.ScrapeMaxPerSource != 5 {
		t.Errorf("ScrapeMaxPerSource = %d, want 5", cfg.ScrapeMaxPerSource)
	}
	if cfg.ScrapeMaxInsertPerSource != 5 {
		t.Errorf("ScrapeMaxInsertPerSource = %d, want 5", cfg.ScrapeMaxInsertPerSource)
	}
	if cfg.ScrapeTimeout != 15*time.Second {
		t.Errorf("ScrapeTimeout = %v, want 15s", cfg.ScrapeTimeout)
	}
	if cfg.ScrapeMinContentChars != 500 {
		t.Errorf("ScrapeMinContentChars = %d, want 500", cfg.ScrapeMinContentChars)
	}
	if cfg.ScrapeDelayMin != 1*time.Second {
		t.Errorf("ScrapeDelayMin = %v, want 1s", cfg.ScrapeDelayMin)
	}
	if cfg.ScrapeDelayMax != 2*time.Second {
		t.Errorf("ScrapeDelayMax = %v, want 2s", cfg.ScrapeDelayMax)
	}
	if cfg.AdminUsername != "admin" || cfg.AdminPassword != "admin123" {
		t.Errorf("admin defaults = %q/%q, want admin/admin123", cfg.AdminUsername, cfg.AdminPassword)
	}
}

func TestLoadRespectsExplicitEnv(t *testing.T) {
	clearEnv(t)
	t.Setenv("SCRAPE_MIN_CONTENT_CHARS", "1200")
	t.Setenv("SCRAPE_TIMEOUT_SECONDS", "7")
	t.Setenv("SCRAPE_MAX_PER_SOURCE", "3")
	t.Setenv("SCRAPE_MAX_INSERT_PER_SOURCE", "7")
	t.Setenv("SCRAPE_DELAY_MIN_SECONDS", "2")
	t.Setenv("SCRAPE_DELAY_MAX_SECONDS", "4")
	t.Setenv("USER_AGENT", "NeuralwireBot/1.0 (+https://neuralwire.com)")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.UserAgent != "NeuralwireBot/1.0 (+https://neuralwire.com)" {
		t.Errorf("UserAgent = %q, want custom UA", cfg.UserAgent)
	}
	if cfg.ScrapeMinContentChars != 1200 {
		t.Errorf("ScrapeMinContentChars = %d, want 1200", cfg.ScrapeMinContentChars)
	}
	if cfg.ScrapeTimeout != 7*time.Second {
		t.Errorf("ScrapeTimeout = %v, want 7s", cfg.ScrapeTimeout)
	}
	if cfg.ScrapeMaxPerSource != 3 {
		t.Errorf("ScrapeMaxPerSource = %d, want 3", cfg.ScrapeMaxPerSource)
	}
	if cfg.ScrapeMaxInsertPerSource != 7 {
		t.Errorf("ScrapeMaxInsertPerSource = %d, want 7", cfg.ScrapeMaxInsertPerSource)
	}
	if cfg.ScrapeDelayMin != 2*time.Second {
		t.Errorf("ScrapeDelayMin = %v, want 2s", cfg.ScrapeDelayMin)
	}
	if cfg.ScrapeDelayMax != 4*time.Second {
		t.Errorf("ScrapeDelayMax = %v, want 4s", cfg.ScrapeDelayMax)
	}
	if cfg.ViewRateLimit != 30 {
		t.Errorf("ViewRateLimit = %d, want default 30", cfg.ViewRateLimit)
	}
	if cfg.TrendingCacheTTLSeconds != 300 {
		t.Errorf("TrendingCacheTTLSeconds = %d, want default 300", cfg.TrendingCacheTTLSeconds)
	}
	if cfg.LoginRateLimit != 5 {
		t.Errorf("LoginRateLimit = %d, want default 5", cfg.LoginRateLimit)
	}
	if cfg.GlobalRateLimit != 120 {
		t.Errorf("GlobalRateLimit = %d, want default 120", cfg.GlobalRateLimit)
	}
}

func TestLoadRejectsBadEnv(t *testing.T) {
	clearEnv(t)
	t.Setenv("SCRAPE_MIN_CONTENT_CHARS", "abc")
	if _, err := Load(); err == nil {
		t.Error("Load with invalid SCRAPE_MIN_CONTENT_CHARS succeeded, want error")
	}
}

func TestSupportsImageGeneration(t *testing.T) {
	cases := []struct {
		provider string
		baseURL  string
		want     bool
	}{
		{"openai", "https://api.openai.com/v1", true},
		{"", "https://api.openai.com/v1", true},
		{"openai", "", true},
		{"deepseek", "https://api.deepseek.com/v1", false},
		{"", "https://api.deepseek.com/v1", false},
		{"gemini", "https://generativelanguage.googleapis.com/v1beta/openai", false},
		{"groq", "https://api.groq.com/openai/v1", false},
		{"openrouter", "https://openrouter.ai/api/v1", false},
		{"ollama", "http://localhost:11434/v1", false},
		{"", "http://127.0.0.1:11434/v1", false},
	}
	for _, c := range cases {
		if got := SupportsImageGeneration(c.provider, c.baseURL); got != c.want {
			t.Errorf("SupportsImageGeneration(%q, %q) = %v, want %v", c.provider, c.baseURL, got, c.want)
		}
	}
}

func TestLoadImageGenEnabledFlag(t *testing.T) {
	clearEnv(t)

	t.Setenv("AI_IMAGE_GENERATION_ENABLED", "false")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AIImageGenerationEnabled == nil || *cfg.AIImageGenerationEnabled != false {
		t.Errorf("AIImageGenerationEnabled = %v, want false pointer", cfg.AIImageGenerationEnabled)
	}

	clearEnv(t)
	t.Setenv("AI_IMAGE_GENERATION_ENABLED", "true")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AIImageGenerationEnabled == nil || *cfg.AIImageGenerationEnabled != true {
		t.Errorf("AIImageGenerationEnabled = %v, want true pointer", cfg.AIImageGenerationEnabled)
	}
}

func TestLoadAppEnv(t *testing.T) {
	clearEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AppEnv != "development" {
		t.Errorf("AppEnv = %q, want default development", cfg.AppEnv)
	}
	if cfg.TrustProxy {
		t.Error("TrustProxy = true, want default false")
	}

	clearEnv(t)
	t.Setenv("APP_ENV", "Production")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AppEnv != "production" {
		t.Errorf("AppEnv = %q, want normalized production", cfg.AppEnv)
	}

	clearEnv(t)
	t.Setenv("TRUST_PROXY", "true")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.TrustProxy {
		t.Error("TrustProxy = false, want true when TRUST_PROXY=true")
	}
}
