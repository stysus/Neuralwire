package config

import (
	"testing"
	"time"
)

// clearEnv removes every variable Load reads so the test sees pure defaults.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"PORT", "DB_PATH", "CORS_ALLOW_ORIGIN", "AI_SUMMARY_API_KEY",
		"AI_SUMMARY_MODEL", "AI_SUMMARY_BASE_URL", "AI_SUMMARY_PROVIDER",
		"ADMIN_USERNAME", "ADMIN_PASSWORD",
		"ADMIN_TOKEN_SECRET", "SCRAPE_MAX_PER_SOURCE", "SCRAPE_TIMEOUT_SECONDS",
		"SCRAPE_MIN_CONTENT_CHARS",
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
	if cfg.ScrapeMaxPerSource != 20 {
		t.Errorf("ScrapeMaxPerSource = %d, want 20", cfg.ScrapeMaxPerSource)
	}
	if cfg.ScrapeTimeout != 15*time.Second {
		t.Errorf("ScrapeTimeout = %v, want 15s", cfg.ScrapeTimeout)
	}
	if cfg.ScrapeMinContentChars != 500 {
		t.Errorf("ScrapeMinContentChars = %d, want 500", cfg.ScrapeMinContentChars)
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

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
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
}

func TestLoadRejectsBadEnv(t *testing.T) {
	clearEnv(t)
	t.Setenv("SCRAPE_MIN_CONTENT_CHARS", "abc")
	if _, err := Load(); err == nil {
		t.Error("Load with invalid SCRAPE_MIN_CONTENT_CHARS succeeded, want error")
	}
}
